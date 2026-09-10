package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bookdb/bookdb/internal/catalog"
)

// TestContract_ResponseBodiesValid verifies that response bodies for all S1
// operations contain the required fields defined in the OpenAPI spec.
func TestContract_ResponseBodiesValid(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "contract-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	key := issued.Secret

	// GET /works/{id} — must have work_id, canonical_title, normalized_title.
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("getWork: expected 200, got %d", rec.Code)
	}
	var work map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &work); err != nil {
		t.Fatalf("getWork unmarshal: %v", err)
	}
	for _, field := range []string{"work_id", "canonical_title", "normalized_title", "created_at", "updated_at"} {
		if _, ok := work[field]; !ok {
			t.Errorf("getWork: missing required field %q", field)
		}
	}

	// GET /works/{id}/editions — must have editions array.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune+"/editions", key)
	if rec.Code != http.StatusOK {
		t.Fatalf("listEditions: expected 200, got %d", rec.Code)
	}
	var editionsBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &editionsBody); err != nil {
		t.Fatalf("listEditions unmarshal: %v", err)
	}
	if _, ok := editionsBody["editions"]; !ok {
		t.Error("listEditions: missing required field 'editions'")
	}

	// GET /expressions/{id} — must have expression_id, work_id, language_code.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/expressions/"+catalog.ExprIDDuneEN, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("getExpression: expected 200, got %d", rec.Code)
	}
	var expr map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &expr); err != nil {
		t.Fatalf("getExpression unmarshal: %v", err)
	}
	for _, field := range []string{"expression_id", "work_id", "language_code", "expression_title"} {
		if _, ok := expr[field]; !ok {
			t.Errorf("getExpression: missing required field %q", field)
		}
	}

	// GET /editions/{id} — must have edition_id, expression_id, edition_title.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/editions/"+catalog.EdIDDunePrint, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("getEdition: expected 200, got %d", rec.Code)
	}
	var edition map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &edition); err != nil {
		t.Fatalf("getEdition unmarshal: %v", err)
	}
	for _, field := range []string{"edition_id", "expression_id", "edition_title"} {
		if _, ok := edition[field]; !ok {
			t.Errorf("getEdition: missing required field %q", field)
		}
	}

	// GET /people/{id} — must have person_id, display_name.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/people/"+catalog.PersonIDFrank, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("getPerson: expected 200, got %d", rec.Code)
	}
	var person map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &person); err != nil {
		t.Fatalf("getPerson unmarshal: %v", err)
	}
	for _, field := range []string{"person_id", "display_name"} {
		if _, ok := person[field]; !ok {
			t.Errorf("getPerson: missing required field %q", field)
		}
	}

	// GET /people/{id}/works — must have works array.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/people/"+catalog.PersonIDFrank+"/works", key)
	if rec.Code != http.StatusOK {
		t.Fatalf("listWorks: expected 200, got %d", rec.Code)
	}
	var worksBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &worksBody); err != nil {
		t.Fatalf("listWorks unmarshal: %v", err)
	}
	if _, ok := worksBody["works"]; !ok {
		t.Error("listWorks: missing required field 'works'")
	}

	// GET /organizations/{id} — must have organization_id, display_name.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/organizations/"+catalog.OrgIDAce, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("getOrg: expected 200, got %d", rec.Code)
	}
	var org map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &org); err != nil {
		t.Fatalf("getOrg unmarshal: %v", err)
	}
	for _, field := range []string{"organization_id", "display_name"} {
		if _, ok := org[field]; !ok {
			t.Errorf("getOrg: missing required field %q", field)
		}
	}

	// GET /resolve — must have status.
	rec = doRequest(t, srv, http.MethodGet, "/api/v1/resolve?namespace=isbn13&value=9780441013597", key)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve: expected 200, got %d", rec.Code)
	}
	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("resolve unmarshal: %v", err)
	}
	if _, ok := res["status"]; !ok {
		t.Error("resolve: missing required field 'status'")
	}
}

// TestContract_ErrorBodiesValid verifies that error responses contain the
// required fields (type, title, status, detail).
func TestContract_ErrorBodiesValid(t *testing.T) {
	srv, _ := newTestServer(t)

	// 401 — missing key.
	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "missing_api_key")

	// 404 — unknown work.
	srv2, store2 := newTestServer(t)
	issued, _ := store2.Create(context.Background(), "k", []string{ScopeRead}, nil, nil)
	rec = doRequest(t, srv2, http.MethodGet, "/api/v1/works/00000000-0000-4000-8000-000000000000", issued.Secret)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "not_found")

	// 403 — insufficient scope.
	srv3, store3 := newTestServer(t)
	issued3, _ := store3.Create(context.Background(), "k", []string{"export:create"}, nil, nil)
	rec = doRequest(t, srv3, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune, issued3.Secret)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	assertErrorBody(t, rec, "insufficient_scope")
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, expectedType string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body unmarshal: %v", err)
	}
	for _, field := range []string{"type", "title", "status", "detail"} {
		if _, ok := body[field]; !ok {
			t.Errorf("error body: missing required field %q", field)
		}
	}
	if body["type"] != expectedType {
		t.Errorf("error body: expected type %q, got %v", expectedType, body["type"])
	}
}

// TestContract_KeyNeverInResponse verifies that the API key secret never
// appears in any response body or header.
func TestContract_KeyNeverInResponse(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "secret-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	secret := issued.Secret

	paths := []string{
		"/api/v1/works/" + catalog.WorkIDDune,
		"/api/v1/works/" + catalog.WorkIDDune + "/editions",
		"/api/v1/expressions/" + catalog.ExprIDDuneEN,
		"/api/v1/editions/" + catalog.EdIDDunePrint,
		"/api/v1/people/" + catalog.PersonIDFrank,
		"/api/v1/people/" + catalog.PersonIDFrank + "/works",
		"/api/v1/organizations/" + catalog.OrgIDAce,
		"/api/v1/resolve?namespace=isbn13&value=9780441013597",
	}
	for _, path := range paths {
		rec := doRequest(t, srv, http.MethodGet, path, secret)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, secret) {
			t.Errorf("%s: response body contains the API key secret", path)
		}
		for header, vals := range rec.Header() {
			for _, val := range vals {
				if strings.Contains(val, secret) {
					t.Errorf("%s: response header %s contains the API key secret", path, header)
				}
			}
		}
	}
}
