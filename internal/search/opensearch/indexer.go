package opensearch

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Indexer applies outbox events to the OpenSearch projection.
// It is idempotent: reprocessing the same event is safe.
type Indexer struct {
	client *Client
	db     *sql.DB
}

// NewIndexer creates an Indexer that reads from the outbox and writes to OpenSearch.
func NewIndexer(client *Client, db *sql.DB) *Indexer {
	return &Indexer{client: client, db: db}
}

// EnsureIndex creates the index and alias if they don't exist.
func (ix *Indexer) EnsureIndex(ctx context.Context, entity string, version int) error {
	index := IndexName(entity, version)
	alias := AliasName(entity)

	// Check if index exists.
	reqURL := ix.client.endpoint(index)
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, reqURL, nil)
	resp, err := ix.client.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			// Index exists, ensure alias.
			return ix.ensureAlias(ctx, index, alias)
		}
	}

	// Create index with basic mapping.
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"entity_id":   map[string]string{"type": "keyword"},
				"title":       map[string]string{"type": "text"},
				"normalized":  map[string]string{"type": "keyword"},
				"language":    map[string]string{"type": "keyword"},
				"authors":     map[string]string{"type": "text"},
				"source_key":  map[string]string{"type": "keyword"},
				"indexed_at":  map[string]string{"type": "date"},
			},
		},
	}
	body, _ := json.Marshal(mapping)
	reqURL = ix.client.endpoint(index)
	req, _ = http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = ix.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch: create index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch: create index returned %s", resp.Status)
	}

	return ix.ensureAlias(ctx, index, alias)
}

func (ix *Indexer) ensureAlias(ctx context.Context, index, alias string) error {
	// Add alias if not present.
	body, _ := json.Marshal(map[string]any{
		"actions": []map[string]any{
			{"add": map[string]string{"index": index, "alias": alias}},
		},
	})
	reqURL := ix.client.endpoint("_aliases")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ix.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch: add alias: %w", err)
	}
	defer resp.Body.Close()
	// 400 "alias already exists" is acceptable.
	if resp.StatusCode >= 300 && resp.StatusCode != 400 {
		return fmt.Errorf("opensearch: add alias returned %s", resp.Status)
	}
	return nil
}

// IndexDocument indexes a single document into the search projection.
func (ix *Indexer) IndexDocument(ctx context.Context, entity string, docID string, doc map[string]any) error {
	alias := AliasName(entity)
	doc["indexed_at"] = time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(doc)

	reqURL := ix.client.endpoint(alias + "/_doc/" + docID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ix.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("opensearch: index doc: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch: index doc returned %s", resp.Status)
	}
	return nil
}

// Search performs a basic text search against the projection.
func (ix *Indexer) Search(ctx context.Context, entity string, query string, limit int) ([]map[string]any, error) {
	alias := AliasName(entity)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	body, _ := json.Marshal(map[string]any{
		"size": limit,
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  query,
				"fields": []string{"title", "authors", "normalized"},
			},
		},
	})
	reqURL := ix.client.endpoint(alias + "/_search")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ix.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opensearch: search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("opensearch: search returned %s", resp.Status)
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string         `json:"_id"`
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("opensearch: decode search: %w", err)
	}

	var docs []map[string]any
	for _, hit := range result.Hits.Hits {
		doc := hit.Source
		doc["_id"] = hit.ID
		docs = append(docs, doc)
	}
	return docs, nil
}

// ProcessOutbox reads unpublished outbox events and indexes them.
// Returns the number of events processed.
func (ix *Indexer) ProcessOutbox(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	rows, err := ix.db.QueryContext(ctx, `
		SELECT id, aggregate_type, aggregate_id, event_type, payload
		FROM bookdb.outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1`, batchSize)
	if err != nil {
		return 0, fmt.Errorf("opensearch: read outbox: %w", err)
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var (
			id           int64
			aggType      string
			aggID        string
			eventType    string
			payload      []byte
		)
		if err := rows.Scan(&id, &aggType, &aggID, &eventType, &payload); err != nil {
			return processed, fmt.Errorf("opensearch: scan outbox: %w", err)
		}

		// Index the document.
		var doc map[string]any
		if err := json.Unmarshal(payload, &doc); err != nil {
			// Mark as published even on parse error to avoid poison pill.
			ix.markPublished(ctx, id)
			continue
		}
		doc["entity_id"] = aggID
		doc["source_key"] = doc["source_key"]

		if err := ix.IndexDocument(ctx, aggType, aggID, doc); err != nil {
			// Don't fail the batch on a single document error.
			ix.markPublished(ctx, id)
			continue
		}

		ix.markPublished(ctx, id)
		processed++
	}
	return processed, rows.Err()
}

func (ix *Indexer) markPublished(ctx context.Context, id int64) {
	_, _ = ix.db.ExecContext(ctx, `
		UPDATE bookdb.outbox SET published_at = now() WHERE id = $1`, id)
}

