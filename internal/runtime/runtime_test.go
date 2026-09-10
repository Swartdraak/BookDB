package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bookdb/bookdb/internal/config"
	"github.com/bookdb/bookdb/internal/health"
)

func TestReadinessUsesRegisteredDependencyChecks(t *testing.T) {
	cfg := config.Defaults()
	cfg.Database.DSN = ""

	rec := httptest.NewRecorder()
	writeDependencyHealth(rec, newDependencyRegistry(&cfg, nil), "dev")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for dependency failure, got %d", rec.Code)
	}

	var report health.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode health report: %v", err)
	}
	if report.Status != string(health.StatusDown) {
		t.Fatalf("expected down status, got %q", report.Status)
	}
	if _, ok := report.Dependencies["runtime"]; ok {
		t.Fatal("did not expect synthetic runtime dependency in readiness report")
	}
	if len(report.Dependencies) < 2 {
		t.Fatalf("expected multiple dependency checks, got %#v", report.Dependencies)
	}
	dep, ok := report.Dependencies["database"].(map[string]any)
	if !ok {
		t.Fatalf("expected database dependency entry, got %#v", report.Dependencies["database"])
	}
	if got, _ := dep["status"].(string); got != string(health.StatusDown) {
		t.Fatalf("expected dependency status down, got %q", got)
	}
	if got, _ := dep["required"].(bool); !got {
		t.Fatal("expected dependency to be marked required")
	}
}

func TestReadinessReturnsOKForDegradedOptionalDependencies(t *testing.T) {
	registry := health.NewRegistry()
	registry.Register("required", health.CheckFunc(func(context.Context) health.Status { return health.StatusOK }), true)
	registry.Register("optional", health.CheckFunc(func(context.Context) health.Status { return health.StatusDegraded }), false)

	rec := httptest.NewRecorder()
	writeHealthReport(rec, registry.Result(context.Background(), "dev"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for degraded optional dependency, got %d", rec.Code)
	}

	var report health.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode health report: %v", err)
	}
	if report.Status != string(health.StatusDegraded) {
		t.Fatalf("expected degraded status, got %q", report.Status)
	}
}

func TestAuthModeEndpointReportsHybridAndSanitizedOIDC(t *testing.T) {
	cfg := config.Defaults()
	cfg.Auth.LocalEnabled = true
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.Issuer = "https://issuer.example"
	cfg.Auth.OIDC.ClientID = "bookdb"
	cfg.Auth.OIDC.ClientSecret = "super-secret"
	cfg.Auth.OIDC.Audience = "bookdb-api"
	cfg.Auth.OIDC.Discovery = true

	server, _, err := newRuntimeServer("api", &cfg, nil)
	if err != nil {
		t.Fatalf("newRuntimeServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/mode", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /auth/mode, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode auth mode payload: %v", err)
	}
	if got, _ := payload["mode"].(string); got != "hybrid" {
		t.Fatalf("expected hybrid mode, got %q", got)
	}
	oidc, ok := payload["oidc"].(map[string]any)
	if !ok {
		t.Fatalf("missing oidc payload: %#v", payload)
	}
	if _, leaked := oidc["client_secret"]; leaked {
		t.Fatalf("oidc payload must not expose client_secret: %#v", oidc)
	}
}

func TestAuthOIDCEndpointReturns404WhenDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Auth.OIDC.Enabled = false

	server, _, err := newRuntimeServer("api", &cfg, nil)
	if err != nil {
		t.Fatalf("newRuntimeServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	rec := httptest.NewRecorder()
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from /auth/oidc when disabled, got %d", rec.Code)
	}
}
