package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bookdb/bookdb/internal/apikey"
	"github.com/bookdb/bookdb/internal/catalog"
	"github.com/bookdb/bookdb/internal/database"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := database.RunUp(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := catalog.LoadFixtures(ctx, db); err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return db
}

func newTestServer(t *testing.T) (*Server, *apikey.Store) {
	t.Helper()
	db := openTestDB(t)
	macKey := []byte("test-mac-key-000000000000000000000000")
	srv := NewServer(db, macKey, nil)
	store := apikey.NewStore(db, macKey)
	return srv, store
}

func doRequest(t *testing.T, srv *Server, method, path, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestGetWork_WithoutKey_Returns401(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetWork_WithValidKey_Returns200(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var work catalog.Work
	if err := json.Unmarshal(rec.Body.Bytes(), &work); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if work.CanonicalTitle != "Dune" {
		t.Fatalf("expected Dune, got %q", work.CanonicalTitle)
	}
}

func TestGetWork_WithInvalidKey_Returns401(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, "not-a-real-key")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetWork_WithRevokedKey_Returns401(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Revoke(context.Background(), issued.KeyID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, issued.Secret)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked key, got %d", rec.Code)
	}
}

func TestGetWork_InsufficientScope_Returns403(t *testing.T) {
	srv, store := newTestServer(t)
	// A key with a different scope than catalog:read.
	issued, err := store.Create(context.Background(), "other-key", []string{"export:create"}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, issued.Secret)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGetWork_UnknownID_Returns404(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/00000000-0000-4000-8000-000000000000", issued.Secret)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestResolve_Ambiguous(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/resolve?namespace=isbn13&value=9780441172719", issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res catalog.Resolution
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Status != "ambiguous" {
		t.Fatalf("expected ambiguous, got %s", res.Status)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(res.Candidates))
	}
}

func TestResolve_Resolves(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/resolve?namespace=isbn13&value=9780441013597", issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res catalog.Resolution
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Status != "resolved" {
		t.Fatalf("expected resolved, got %s", res.Status)
	}
}

func TestResolve_WithoutKey_Returns401(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/resolve?namespace=isbn13&value=9780441013597", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListEditionsForWork(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune+"/editions", issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Editions []catalog.Edition `json:"editions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Editions) != 4 {
		t.Fatalf("expected 4 editions, got %d", len(body.Editions))
	}
}
