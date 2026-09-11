package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

// SearchServer provides the S2 search API endpoints.
// It reads from the OpenSearch projection (not directly from PostgreSQL).
type SearchServer struct {
	db *sql.DB
	// searchFn is the function that performs the actual search.
	// In production this queries OpenSearch; in tests it can be stubbed.
	searchFn func(entity, query string, limit int) ([]map[string]any, error)
}

// NewSearchServer creates a SearchServer.
func NewSearchServer(db *sql.DB, searchFn func(entity, query string, limit int) ([]map[string]any, error)) *SearchServer {
	return &SearchServer{db: db, searchFn: searchFn}
}

// Handler returns the HTTP handler for the /api/v1/search namespace.
func (s *SearchServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	mux.HandleFunc("GET /api/v1/search/{entity}", s.handleEntitySearch)
	return mux
}

func (s *SearchServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Query parameter 'q' is required.")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	if s.searchFn == nil {
		writeError(w, http.StatusServiceUnavailable, "search_unavailable", "Search backend is not configured.")
		return
	}

	docs, err := s.searchFn("work", query, limit)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "search_error", "Search failed.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":   query,
		"total":   len(docs),
		"results": docs,
	})
}

func (s *SearchServer) handleEntitySearch(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	switch entity {
	case "work", "edition", "person", "organization":
	default:
		writeError(w, http.StatusBadRequest, "invalid_entity", "Entity must be work, edition, person, or organization.")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Query parameter 'q' is required.")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	if s.searchFn == nil {
		writeError(w, http.StatusServiceUnavailable, "search_unavailable", "Search backend is not configured.")
		return
	}

	docs, err := s.searchFn(entity, query, limit)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "search_error", "Search failed.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entity":  entity,
		"query":   query,
		"total":   len(docs),
		"results": docs,
	})
}

// ProvenanceHandler returns the source provenance for a canonical entity.
func ProvenanceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityType := r.PathValue("type")
		entityID := r.PathValue("id")

		var records []map[string]any
		rows, err := db.QueryContext(r.Context(), `
			SELECT source_name, source_key, status, raw_json, created_at
			FROM bookdb.source_records
			WHERE source_key = $1
			ORDER BY created_at DESC
			LIMIT 10`, entityID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Failed to load provenance.")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var (
				sourceName string
				sourceKey  string
				status     string
				rawJSON    []byte
				createdAt  string
			)
			if err := rows.Scan(&sourceName, &sourceKey, &status, &rawJSON, &createdAt); err != nil {
				continue
			}
			var raw map[string]any
			_ = json.Unmarshal(rawJSON, &raw)
			records = append(records, map[string]any{
				"source_name": sourceName,
				"source_key":  sourceKey,
				"status":      status,
				"raw":         raw,
				"created_at":  createdAt,
			})
		}

		if records == nil {
			records = []map[string]any{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"entity_type":    entityType,
			"entity_id":      entityID,
			"source_records": records,
		})
	}
}
