package config

import (
	"strings"
	"testing"
	"time"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"BOOKDB_ENV", "BOOKDB_LOG_LEVEL", "BOOKDB_LOG_FORMAT", "BOOKDB_API_ADDR",
		"BOOKDB_DATABASE_URL", "BOOKDB_DATABASE_URL_DIRECT",
		"BOOKDB_NATS_URL", "BOOKDB_NATS_JETSTREAM_ENABLED",
		"BOOKDB_VALKEY_URL", "BOOKDB_OPENSEARCH_URL", "BOOKDB_S3_ENDPOINT",
		"BOOKDB_S3_SECRET_KEY_REF", "BOOKDB_AUTH_LOCAL_ENABLED",
		"BOOKDB_OIDC_ISSUER", "BOOKDB_OIDC_CLIENT_ID", "BOOKDB_SOURCE_REGISTRY_PATH",
		"BOOKDB_OTEL_ENABLED", "BOOKDB_METRICS_ADDR",
	} {
		t.Setenv(k, "")
	}
}

func TestDefaults_HasRequiredFields(t *testing.T) {
	c := Defaults()
	if c.Database.DSN == "" {
		t.Fatal("default database DSN must be non-empty")
	}
	if c.NATS.URL == "" {
		t.Fatal("default NATS URL must be non-empty")
	}
	if c.Log.Level == "" || c.Log.Format == "" {
		t.Fatal("default log level/format must be non-empty")
	}
	if c.API.ReadTimeout != 15*time.Second {
		t.Fatalf("unexpected default read timeout %s", c.API.ReadTimeout)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("BOOKDB_ENV", "production")
	t.Setenv("BOOKDB_DATABASE_URL", "postgres://u:p@db:5432/bookdb?sslmode=require")
	t.Setenv("BOOKDB_LOG_LEVEL", "debug")
	t.Setenv("BOOKDB_NATS_JETSTREAM_ENABLED", "false")
	t.Setenv("BOOKDB_DB_MAX_OPEN_CONNS", "123")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != "production" {
		t.Fatalf("env not applied: %q", cfg.Env)
	}
	if cfg.Database.DSN != "postgres://u:p@db:5432/bookdb?sslmode=require" {
		t.Fatalf("database DSN not applied: %q", cfg.Database.DSN)
	}
	if cfg.Log.Level != "debug" {
		t.Fatalf("log level not applied: %q", cfg.Log.Level)
	}
	if cfg.NATS.JetStream {
		t.Fatal("NATS JetStream should be false after env override")
	}
	if cfg.Database.MaxOpenConns != 123 {
		t.Fatalf("max open conns not applied: %d", cfg.Database.MaxOpenConns)
	}
}

func TestValidate_MissingDatabaseFails(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Database.DSN = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty database DSN")
	}
}

func TestValidate_BadNATSURLFails(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.NATS.URL = "http://notnats:4222"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for bad NATS URL")
	}
}

func TestValidate_OIDCRequiresIssuer(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.ClientID = "client"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for OIDC without issuer")
	}
}

func TestSanitized_RedactsDSN(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Database.DSN = "postgres://bookdb:supersecretpass@127.0.0.1:5432/bookdb?sslmode=disable"
	s := cfg.Sanitized()
	dsn, _ := s["database"].(map[string]any)["dsn"].(string)
	if dsn == "" {
		t.Fatal("sanitized database DSN missing")
	}
	if strings.Contains(dsn, "supersecretpass") {
		t.Fatalf("sanitized DSN leaks password: %q", dsn)
	}
}

func TestSanitized_RedactsServiceURLs(t *testing.T) {
	clearEnv(t)
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Database.DSN = "postgres://bookdb:supersecretpass@127.0.0.1:5432/bookdb?sslmode=disable"
	cfg.Database.DSNDirect = "postgres://bookdb:directsecret@127.0.0.1:5432/bookdb?sslmode=disable"
	cfg.NATS.URL = "nats://bookdb:natssecret@127.0.0.1:4222"
	cfg.Valkey.URL = "redis://bookdb:valkeysecret@127.0.0.1:6379/0"
	cfg.OpenSearch.URL = "http://bookdb:searchsecret@127.0.0.1:9200"
	cfg.S3.Endpoint = "http://bookdb:s3secret@127.0.0.1:8333"

	s := cfg.Sanitized()
	checks := map[string]string{
		"database.dsn":        s["database"].(map[string]any)["dsn"].(string),
		"database.dsn_direct": s["database"].(map[string]any)["dsn_direct"].(string),
		"nats.url":            s["nats"].(map[string]any)["url"].(string),
		"valkey.url":          s["valkey"].(map[string]any)["url"].(string),
		"opensearch.url":      s["opensearch"].(map[string]any)["url"].(string),
		"s3.endpoint":         s["s3"].(map[string]any)["endpoint"].(string),
	}
	for name, got := range checks {
		if strings.Contains(got, "supersecretpass") || strings.Contains(got, "directsecret") || strings.Contains(got, "natssecret") || strings.Contains(got, "valkeysecret") || strings.Contains(got, "searchsecret") || strings.Contains(got, "s3secret") {
			t.Fatalf("%s leaks credentials: %q", name, got)
		}
	}
}

func TestRedactDSN_Various(t *testing.T) {
	cases := map[string]struct {
		dsn            string
		expectRedacted bool
	}{
		"with credentials": {dsn: "postgres://u:p@h:5432/db", expectRedacted: true},
		"simple nats":      {dsn: "nats://127.0.0.1:4222", expectRedacted: false},
		"host port":        {dsn: "redis://valkey:6379/0", expectRedacted: false},
		"postgres host":    {dsn: "postgres://h5432/db", expectRedacted: false},
	}
	for name, tc := range cases {
		got := redactDSN(tc.dsn)
		if tc.expectRedacted && strings.Contains(got, "u:p") {
			t.Fatalf("%s: redactDSN leaked credentials for %q: %q", name, tc.dsn, got)
		}
		if !tc.expectRedacted && got != tc.dsn {
			t.Fatalf("%s: redactDSN unexpectedly changed %q to %q", name, tc.dsn, got)
		}
	}
}
