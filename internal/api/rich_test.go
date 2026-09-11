package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bookdb/bookdb/internal/catalog"
)

func TestGetWorkRich_WithValidKey_Returns200(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/works/"+catalog.WorkIDDune+"/rich", issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Work   catalog.Work    `json:"work"`
		Assets []catalog.Asset `json:"assets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Work.WorkID != catalog.WorkIDDune {
		t.Fatalf("expected work %s, got %s", catalog.WorkIDDune, body.Work.WorkID)
	}
	for _, asset := range body.Assets {
		if asset.Eligibility != "public" {
			t.Fatalf("expected only public assets, got %q", asset.Eligibility)
		}
	}
}

func TestCompareEditions_WithValidKey_Returns200(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	path := "/api/v1/editions/compare?left=" + catalog.EdIDDunePrint + "&right=" + catalog.EdIDDuneEbook
	rec := doRequest(t, srv, http.MethodGet, path, issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		SameWork    bool     `json:"same_work"`
		Differences []string `json:"differences"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.SameWork {
		t.Fatal("expected same_work=true")
	}
	if len(body.Differences) == 0 {
		t.Fatal("expected at least one difference")
	}
}

func TestQualityCoverage_WithValidKey_Returns200(t *testing.T) {
	srv, store := newTestServer(t)
	issued, err := store.Create(context.Background(), "read-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rec := doRequest(t, srv, http.MethodGet, "/api/v1/quality/coverage", issued.Secret)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body catalog.QualityReport
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.ByFormatLanguage) == 0 {
		t.Fatal("expected by_format_language rows")
	}
}
