package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSearchHTTPContract exercises the S2 search endpoints (issue #22,
// SearchServer) through the real HTTP handler: query parameter validation,
// the configured-backend success path with the searchFn wiring, the
// searchFn error -> 503 search_error path, and the degraded 503 path where
// the search backend is unavailable (searchFn nil, the shape runtime.go's
// newSearchFn returns when OpenSearch is unconfigured or unreachable).
func TestSearchHTTPContract(t *testing.T) {
	searchErr := errors.New("backend down")
	srv := NewSearchServer(nil, func(entity, query string, limit int) ([]map[string]any, error) {
		if query == "boom" {
			return nil, searchErr
		}
		// _id echoes the entity the backend was asked to search, so the
		// test can assert the entity wiring through the response.
		return []map[string]any{
			{"_id": entity, "entity_id": "w1", "canonical_title": "Dune", "matched_query": query, "matched_limit": limit},
		}, nil
	})
	handler := srv.Handler()

	do := func(t *testing.T, method, target string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}

	// 400 — missing query parameter on both endpoints.
	rec := do(t, http.MethodGet, "/api/v1/search")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("search without q: expected 400, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "invalid_request")

	rec = do(t, http.MethodGet, "/api/v1/search/work")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("entity search without q: expected 400, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "invalid_request")

	// 400 — invalid entity.
	rec = do(t, http.MethodGet, "/api/v1/search/universe?q=Dune")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid entity: expected 400, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "invalid_entity")

	// 200 — generic search delegates to searchFn("work", ...) and wraps the
	// docs in the documented response envelope.
	rec = do(t, http.MethodGet, "/api/v1/search?q=Dune&limit=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Query   string           `json:"query"`
		Total   int              `json:"total"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("search unmarshal: %v", err)
	}
	if body.Query != "Dune" || body.Total != 1 || len(body.Results) != 1 {
		t.Fatalf("unexpected search body: %+v", body)
	}
	// The backend must have been invoked for the work entity with the
	// requested limit (the echo doc carries both).
	// matched_limit is echoed back as a JSON number (float64); compare as a
	// number rather than a raw interface, and verify the work-entity wiring
	// through the echoed _id.
	if lim, _ := body.Results[0]["matched_limit"].(float64); lim != 5 {
		t.Fatalf("searchFn wiring wrong (limit): %v", body.Results[0])
	}
	if body.Results[0]["entity_id"] != "w1" || body.Results[0]["_id"] != "work" {
		t.Fatalf("searchFn wiring wrong (entity/doc): %v", body.Results[0])
	}

	// 200 — entity search delegates with the requested entity.
	rec = do(t, http.MethodGet, "/api/v1/search/person?q=Frank")
	if rec.Code != http.StatusOK {
		t.Fatalf("entity search: expected 200, got %d", rec.Code)
	}
	var entityBody struct {
		Entity  string           `json:"entity"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &entityBody); err != nil {
		t.Fatalf("entity search unmarshal: %v", err)
	}
	if entityBody.Entity != "person" || entityBody.Results[0]["matched_query"] != "Frank" {
		t.Fatalf("entity search wiring wrong: %+v", entityBody)
	}

	// 503 — searchFn error maps to search_error.
	rec = do(t, http.MethodGet, "/api/v1/search?q=boom")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("search backend error: expected 503, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "search_error")
}

// TestSearchHTTPContract_BackendUnavailable covers the degraded path:
// newSearchFn in runtime.go returns nil when OpenSearch is unconfigured or
// unreachable, so the mounted handlers must answer 503 search_unavailable
// instead of panicking or 500ing.
func TestSearchHTTPContract_BackendUnavailable(t *testing.T) {
	srv := NewSearchServer(nil, nil)
	handler := srv.Handler()

	for _, target := range []string{
		"/api/v1/search?q=Dune",
		"/api/v1/search/work?q=Dune",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected 503 when backend unavailable, got %d: %s", target, rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: error body unmarshal: %v", target, err)
		}
		if body["type"] != "search_unavailable" {
			t.Fatalf("%s: expected type search_unavailable, got %v", target, body["type"])
		}
	}
}

// TestProvenanceHandlerContract verifies the S2 provenance endpoint returns
// the source records for a canonical entity and an empty list (not null)
// when none exist.
func TestProvenanceHandlerContract(t *testing.T) {
	// The provenance endpoint is mounted on the runtime mux, not the catalog
	// Server's mux (runtime.go mounts api.ProvenanceHandler(dbPool)
	// separately); test the handler factory directly with the test DB.
	db := openTestDB(t)

	// The provenance handler reads path parameters from the Go 1.22+ mux
	// pattern (GET /api/v1/provenance/{type}/{id}), so register it on a real
	// mux with that pattern rather than calling the handler bare.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/provenance/{type}/{id}", ProvenanceHandler(db))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/provenance/work/11111111-1111-1111-1111-111111111111", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("provenance: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		EntityType    string           `json:"entity_type"`
		EntityID      string           `json:"entity_id"`
		SourceRecords []map[string]any `json:"source_records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("provenance unmarshal: %v", err)
	}
	if body.EntityType != "work" || body.EntityID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("provenance echo wrong: %+v", body)
	}
	if body.SourceRecords == nil || len(body.SourceRecords) != 0 {
		t.Fatalf("provenance: expected empty (non-nil) source_records, got %v", body.SourceRecords)
	}
}
