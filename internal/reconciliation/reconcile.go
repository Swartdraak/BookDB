// Package reconciliation implements S3 cross-source reconciliation:
// field-level selection, duplicate candidate detection, merge/split
// operations, and the durable change feed.
package reconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Reconciler performs cross-source reconciliation operations.
type Reconciler struct {
	db *sql.DB
}

// NewReconciler creates a Reconciler backed by the given database.
func NewReconciler(db *sql.DB) *Reconciler {
	return &Reconciler{db: db}
}

// FieldValue is a single field assertion from a source.
type FieldValue struct {
	SourceName string          `json:"source_name"`
	SourceKey  string          `json:"source_key"`
	Value      json.RawMessage `json:"value"`
}

// SelectField applies field-level selection rules to determine the
// canonical value for a field. Rules:
//  1. If only one source provides the field, use it.
//  2. If multiple sources agree, use the value.
//  3. If sources disagree, prefer the source with higher authority
//     (Wikidata > Open Library for structured data).
//  4. Record provenance for all values.
func (r *Reconciler) SelectField(ctx context.Context, entityType string, entityID uuid.UUID, fieldName string, values []FieldValue) (string, error) {
	if len(values) == 0 {
		return "", nil
	}

	// Record all values as provenance.
	for _, v := range values {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO bookdb.field_provenance (entity_type, entity_id, field_name, source_name, source_key, value, selected)
			VALUES ($1, $2, $3, $4, $5, $6, false)
			ON CONFLICT (entity_type, entity_id, field_name, source_name, source_key)
			DO UPDATE SET value = EXCLUDED.value`,
			entityType, entityID, fieldName, v.SourceName, v.SourceKey, v.Value)
		if err != nil {
			return "", fmt.Errorf("reconciliation: record provenance: %w", err)
		}
	}

	// Determine the selected value.
	selected := selectBestValue(values)

	// Mark the selected value.
	_, err := r.db.ExecContext(ctx, `
		UPDATE bookdb.field_provenance
		SET selected = true
		WHERE entity_type = $1 AND entity_id = $2 AND field_name = $3
		  AND source_name = $4 AND source_key = $5`,
		entityType, entityID, fieldName, selected.SourceName, selected.SourceKey)
	if err != nil {
		return "", fmt.Errorf("reconciliation: mark selected: %w", err)
	}

	return string(selected.Value), nil
}

// selectBestValue applies the selection rules.
func selectBestValue(values []FieldValue) FieldValue {
	if len(values) == 1 {
		return values[0]
	}

	// Check if all values agree.
	first := values[0].Value
	allAgree := true
	for _, v := range values[1:] {
		if !jsonEqual(first, v.Value) {
			allAgree = false
			break
		}
	}
	if allAgree {
		return values[0]
	}

	// Sources disagree: prefer higher authority.
	// Authority order: wikidata > openlibrary > other
	authority := map[string]int{
		"wikidata":     3,
		"openlibrary":  2,
	}
	best := values[0]
	bestAuth := authority[best.SourceName]
	for _, v := range values[1:] {
		a := authority[v.SourceName]
		if a > bestAuth {
			best = v
			bestAuth = a
		}
	}
	return best
}

func jsonEqual(a, b json.RawMessage) bool {
	// Simple comparison: normalize by unmarshaling to interface.
	var ia, ib any
	if err := json.Unmarshal(a, &ia); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &ib); err != nil {
		return string(a) == string(b)
	}
	ja, _ := json.Marshal(ia)
	jb, _ := json.Marshal(ib)
	return string(ja) == string(jb)
}

// MergeResult is the outcome of a merge operation.
type MergeResult struct {
	CanonicalID uuid.UUID `json:"canonical_id"`
	RedirectID  uuid.UUID `json:"redirect_id"`
	Revision    int64     `json:"revision"`
}

// Merge combines two entities of the same type into one.
// The entity with the lower UUID becomes the canonical (deterministic).
// The other gets a redirect. All relationships are preserved.
func (r *Reconciler) Merge(ctx context.Context, entityType string, idA, idB uuid.UUID, reason string) (*MergeResult, error) {
	// Deterministic: lower UUID is canonical.
	canonical, redirect := idA, idB
	if idB.String() < idA.String() {
		canonical, redirect = idB, idA
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: begin merge: %w", err)
	}
	defer tx.Rollback()

	// Create the redirect.
	var redirectID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.identity_redirects (from_type, from_id, to_type, to_id, reason)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (from_type, from_id) DO UPDATE SET to_id = EXCLUDED.to_id, reason = EXCLUDED.reason
		RETURNING redirect_id`,
		entityType, redirect, entityType, canonical, reason).Scan(&redirectID)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: create redirect: %w", err)
	}

	// Increment the canonical revision.
	var newRevision int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.canonical_revisions (entity_type, entity_id, revision)
		VALUES ($1, $2, 1)
		ON CONFLICT (entity_type, entity_id)
		DO UPDATE SET revision = canonical_revisions.revision + 1, updated_at = now()
		RETURNING revision`,
		entityType, canonical).Scan(&newRevision)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: increment revision: %w", err)
	}

	// Record the change.
	payload, _ := json.Marshal(map[string]string{
		"merged_from": redirect.String(),
		"reason":      reason,
	})
	_, err = tx.ExecContext(ctx, `
		INSERT INTO bookdb.change_feed (entity_type, entity_id, change_type, payload, revision)
		VALUES ($1, $2, 'merged', $3, $4)`,
		entityType, canonical, payload, newRevision)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: record change: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reconciliation: commit merge: %w", err)
	}

	return &MergeResult{
		CanonicalID: canonical,
		RedirectID:  redirectID,
		Revision:    newRevision,
	}, nil
}

// SplitResult is the outcome of a split operation.
type SplitResult struct {
	NewID      uuid.UUID `json:"new_id"`
	Revision   int64     `json:"revision"`
}

// Split separates a portion of an entity into a new entity.
// The original entity keeps its ID; a new entity is created.
func (r *Reconciler) Split(ctx context.Context, entityType string, originalID uuid.UUID, expectedRevision int64, reason string) (*SplitResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: begin split: %w", err)
	}
	defer tx.Rollback()

	// Version check: ensure the revision matches.
	var currentRevision int64
	err = tx.QueryRowContext(ctx, `
		SELECT revision FROM bookdb.canonical_revisions
		WHERE entity_type = $1 AND entity_id = $2`,
		entityType, originalID).Scan(&currentRevision)
	if err == sql.ErrNoRows {
		currentRevision = 1
	} else if err != nil {
		return nil, fmt.Errorf("reconciliation: check revision: %w", err)
	}
	if currentRevision != expectedRevision {
		return nil, fmt.Errorf("reconciliation: revision mismatch: expected %d, got %d", expectedRevision, currentRevision)
	}

	// Create the new entity ID.
	newID := uuid.New()

	// Increment the original's revision.
	var newRevision int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.canonical_revisions (entity_type, entity_id, revision)
		VALUES ($1, $2, 1)
		ON CONFLICT (entity_type, entity_id)
		DO UPDATE SET revision = canonical_revisions.revision + 1, updated_at = now()
		RETURNING revision`,
		entityType, originalID).Scan(&newRevision)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: increment revision: %w", err)
	}

	// Record the change.
	payload, _ := json.Marshal(map[string]string{
		"split_to": newID.String(),
		"reason":   reason,
	})
	_, err = tx.ExecContext(ctx, `
		INSERT INTO bookdb.change_feed (entity_type, entity_id, change_type, payload, revision)
		VALUES ($1, $2, 'split', $3, $4)`,
		entityType, originalID, payload, newRevision)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: record change: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reconciliation: commit split: %w", err)
	}

	return &SplitResult{
		NewID:    newID,
		Revision: newRevision,
	}, nil
}

// ResolveRedirect follows a redirect chain to find the canonical entity.
// Returns the original ID if no redirect exists.
func (r *Reconciler) ResolveRedirect(ctx context.Context, entityType string, entityID uuid.UUID) (uuid.UUID, error) {
	// Follow at most 10 redirects to prevent cycles.
	current := entityID
	for i := 0; i < 10; i++ {
		var toID string
		err := r.db.QueryRowContext(ctx, `
			SELECT to_id::text FROM bookdb.identity_redirects
			WHERE from_type = $1 AND from_id = $2`,
			entityType, current).Scan(&toID)
		if err == sql.ErrNoRows {
			return current, nil
		}
		if err != nil {
			return current, fmt.Errorf("reconciliation: resolve redirect: %w", err)
		}
		current, err = uuid.Parse(toID)
		if err != nil {
			return current, fmt.Errorf("reconciliation: parse redirect target: %w", err)
		}
	}
	return current, fmt.Errorf("reconciliation: redirect chain too long (possible cycle)")
}

// Change is a single entry in the change feed.
type Change struct {
	ChangeID    int64           `json:"change_id"`
	EntityType  string          `json:"entity_type"`
	EntityID    uuid.UUID       `json:"entity_id"`
	ChangeType  string          `json:"change_type"`
	Payload     json.RawMessage `json:"payload"`
	Revision    int64           `json:"revision"`
	CommittedAt time.Time       `json:"committed_at"`
}

// ConsumeChanges reads changes from the change feed starting after the
// given cursor (last consumed change_id). Returns the changes and the
// new cursor.
func (r *Reconciler) ConsumeChanges(ctx context.Context, cursor int64, limit int) ([]Change, int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT change_id, entity_type, entity_id, change_type, payload, revision, committed_at
		FROM bookdb.change_feed
		WHERE change_id > $1
		ORDER BY change_id
		LIMIT $2`, cursor, limit)
	if err != nil {
		return nil, cursor, fmt.Errorf("reconciliation: consume changes: %w", err)
	}
	defer rows.Close()

	changes := make([]Change, 0, limit)
	newCursor := cursor
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.ChangeID, &c.EntityType, &c.EntityID, &c.ChangeType, &c.Payload, &c.Revision, &c.CommittedAt); err != nil {
			return nil, cursor, fmt.Errorf("reconciliation: scan change: %w", err)
		}
		changes = append(changes, c)
		newCursor = c.ChangeID
	}
	if err := rows.Err(); err != nil {
		return nil, cursor, fmt.Errorf("reconciliation: change rows: %w", err)
	}

	return changes, newCursor, nil
}

// FindDuplicateCandidates finds potential duplicates based on normalized
// title matching. Returns pairs with a confidence score.
func (r *Reconciler) FindDuplicateCandidates(ctx context.Context, entityType string) ([]DuplicateCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.source_key, b.source_key,
		       similarity(a.payload->>'title', b.payload->>'title') as sim
		FROM bookdb.source_records a
		JOIN bookdb.source_records b
		  ON a.source_name != b.source_name
		  AND a.source_key < b.source_key
		WHERE a.payload->>'title' IS NOT NULL
		  AND b.payload->>'title' IS NOT NULL
		  AND similarity(a.payload->>'title', b.payload->>'title') > 0.8
		ORDER BY sim DESC
		LIMIT 100`,
	)
	if err != nil {
		// pg_trgm extension may not be available; fall back to exact match.
		rows, err = r.db.QueryContext(ctx, `
			SELECT a.source_key, b.source_key, 1.0 as sim
			FROM bookdb.source_records a
			JOIN bookdb.source_records b
			  ON a.source_name != b.source_name
			  AND a.source_key < b.source_key
			WHERE a.payload->>'title' = b.payload->>'title'
			  AND a.payload->>'title' IS NOT NULL
			LIMIT 100`)
		if err != nil {
			return nil, fmt.Errorf("reconciliation: find duplicates: %w", err)
		}
	}
	defer rows.Close()

	var candidates []DuplicateCandidate
	for rows.Next() {
		var keyA, keyB string
		var sim float64
		if err := rows.Scan(&keyA, &keyB, &sim); err != nil {
			return nil, fmt.Errorf("reconciliation: scan candidate: %w", err)
		}
		candidates = append(candidates, DuplicateCandidate{
			KeyA:       keyA,
			KeyB:       keyB,
			Confidence: sim,
		})
	}
	return candidates, rows.Err()
}

// DuplicateCandidate is a potential duplicate pair.
type DuplicateCandidate struct {
	KeyA       string  `json:"key_a"`
	KeyB       string  `json:"key_b"`
	Confidence float64 `json:"confidence"`
}

// RecordWikidataClaim stores a Wikidata claim for an entity.
func (r *Reconciler) RecordWikidataClaim(ctx context.Context, entityType string, entityID uuid.UUID, qid, property string, value json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO bookdb.wikidata_claims (entity_type, entity_id, wikidata_qid, property, value)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (entity_type, entity_id, wikidata_qid, property)
		DO UPDATE SET value = EXCLUDED.value`,
		entityType, entityID, qid, property, value)
	if err != nil {
		return fmt.Errorf("reconciliation: record wikidata claim: %w", err)
	}
	return nil
}

// NormalizeTitle normalizes a title for comparison: lowercase, trim,
// collapse whitespace.
func NormalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	// Collapse multiple spaces.
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}
	return title
}

