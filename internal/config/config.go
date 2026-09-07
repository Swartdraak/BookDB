// Package config implements BookDB's typed configuration architecture.
//
// Precedence (highest wins), matching docs/11_TECHNICAL_DOCUMENTATION.md:
//
//	defaults < YAML config file < environment variables < runtime/secret refs
//
// The configuration is intentionally structured into per-subsystem sections
// (Postgres, NATS, Valkey, OpenSearch, S3, Auth, Source registry, Observability)
// so each role can load only what it needs. Secrets are referenced, not inlined,
// via `*_REF`-style patterns for production; development uses plain values from
// `.env.example`.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/auth"
)

// Config is the root BookDB configuration.
type Config struct {
	Env           string       `mapstructure:"env" yaml:"env"`
	Log           Log          `mapstructure:"log" yaml:"log"`
	API           API          `mapstructure:"api" yaml:"api"`
	Database      Database     `mapstructure:"database" yaml:"database"`
	NATS          NATS         `mapstructure:"nats" yaml:"nats"`
	Valkey        Valkey       `mapstructure:"valkey" yaml:"valkey"`
	OpenSearch    OpenSearch   `mapstructure:"opensearch" yaml:"opensearch"`
	S3            S3           `mapstructure:"s3" yaml:"s3"`
	Auth          Auth         `mapstructure:"auth" yaml:"auth"`
	Sources       Sources      `mapstructure:"sources" yaml:"sources"`
	Observability Obs          `mapstructure:"observability" yaml:"observability"`
	FeatureFlags  FeatureFlags `mapstructure:"feature_flags" yaml:"feature_flags"`

	// yamlProvided is true when a YAML config file was found at Load time.
	// It is not serialized and is used only by diagnostics/tests.
	yamlProvided bool `mapstructure:"-" yaml:"-"`
}

// YAMLProvided reports whether a YAML config file was found at Load time.
func (c *Config) YAMLProvided() bool { return c.yamlProvided }

type Log struct {
	Level  string `mapstructure:"level" yaml:"level"`
	Format string `mapstructure:"format" yaml:"format"`
}

type API struct {
	Addr               string        `mapstructure:"addr" yaml:"addr"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	ShutdownTimeout    time.Duration `mapstructure:"shutdown_timeout" yaml:"shutdown_timeout"`
	DevCORSPortOrigins []string      `mapstructure:"dev_cors_origins" yaml:"dev_cors_origins"`
}

type Database struct {
	DSN                string        `mapstructure:"dsn" yaml:"dsn"`
	DSNDirect          string        `mapstructure:"dsn_direct" yaml:"dsn_direct"`
	MaxOpenConns       int           `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns       int           `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime    time.Duration `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	HealthCheckTimeout time.Duration `mapstructure:"health_check_timeout" yaml:"health_check_timeout"`
}

type NATS struct {
	URL              string        `mapstructure:"url" yaml:"url"`
	JetStream        bool          `mapstructure:"jetstream" yaml:"jetstream"`
	Stream           string        `mapstructure:"stream" yaml:"stream"`
	BootstrapSubject string        `mapstructure:"bootstrap_subject" yaml:"bootstrap_subject"`
	PublishTimeout   time.Duration `mapstructure:"publish_timeout" yaml:"publish_timeout"`
}

type Valkey struct {
	URL       string `mapstructure:"url" yaml:"url"`
	PoolSize  int    `mapstructure:"pool_size" yaml:"pool_size"`
	KeyPrefix string `mapstructure:"key_prefix" yaml:"key_prefix"`
}

type OpenSearch struct {
	URL                string        `mapstructure:"url" yaml:"url"`
	Timeout            time.Duration `mapstructure:"timeout" yaml:"timeout"`
	IndexAliasWorks    string        `mapstructure:"index_alias_works" yaml:"index_alias_works"`
	IndexAliasEditions string        `mapstructure:"index_alias_editions" yaml:"index_alias_editions"`
	IndexAliasPeople   string        `mapstructure:"index_alias_people" yaml:"index_alias_people"`
	IndexAliasSeries   string        `mapstructure:"index_alias_series" yaml:"index_alias_series"`
}

type S3 struct {
	Endpoint       string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKey      string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey      string `mapstructure:"secret_key" yaml:"secret_key"`
	SecretKeyRef   string `mapstructure:"secret_key_ref" yaml:"secret_key_ref"`
	Bucket         string `mapstructure:"bucket" yaml:"bucket"`
	Region         string `mapstructure:"region" yaml:"region"`
	ForcePathStyle bool   `mapstructure:"force_path_style" yaml:"force_path_style"`
	DisableSSL     bool   `mapstructure:"disable_ssl" yaml:"disable_ssl"`
}

type Auth struct {
	LocalEnabled bool `mapstructure:"local_enabled" yaml:"local_enabled"`
	OIDC         OIDC `mapstructure:"oidc" yaml:"oidc"`
}

type OIDC struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled"`
	Issuer          string `mapstructure:"issuer" yaml:"issuer"`
	ClientID        string `mapstructure:"client_id" yaml:"client_id"`
	ClientSecret    string `mapstructure:"client_secret" yaml:"client_secret"`
	ClientSecretRef string `mapstructure:"client_secret_ref" yaml:"client_secret_ref"`
	Audience        string `mapstructure:"audience" yaml:"audience"`
	Discovery       bool   `mapstructure:"discovery" yaml:"discovery"`
}

type Sources struct {
	RegistryPath string `mapstructure:"registry_path" yaml:"registry_path"`
}

type Obs struct {
	Enabled          bool   `mapstructure:"enabled" yaml:"enabled"`
	OTLPExporterURL  string `mapstructure:"otlp_exporter_url" yaml:"otlp_exporter_url"`
	ServiceName      string `mapstructure:"service_name" yaml:"service_name"`
	TracesSampler    string `mapstructure:"traces_sampler" yaml:"traces_sampler"`
	MetricsEnabled   bool   `mapstructure:"metrics_enabled" yaml:"metrics_enabled"`
	MetricsNamespace string `mapstructure:"metrics_namespace" yaml:"metrics_namespace"`
	MetricsAddr      string `mapstructure:"metrics_addr" yaml:"metrics_addr"`
}

type FeatureFlags struct {
	OutboxEnabled      bool `mapstructure:"outbox_enabled" yaml:"outbox_enabled"`
	SearchIndexEnabled bool `mapstructure:"search_index_enabled" yaml:"search_index_enabled"`
	AssetPipeline      bool `mapstructure:"asset_pipeline" yaml:"asset_pipeline"`
}

// Defaults returns a Config populated with safe development defaults.
func Defaults() Config {
	return Config{
		Env: "development",
		Log: Log{
			Level:  "info",
			Format: "json",
		},
		API: API{
			Addr:            ":8080",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Database: Database{
			DSN:                "postgres://bookdb:bookdb@127.0.0.1:5432/bookdb?sslmode=disable",
			DSNDirect:          "postgres://bookdb:bookdb@127.0.0.1:5432/bookdb?sslmode=disable",
			MaxOpenConns:       50,
			MaxIdleConns:       10,
			ConnMaxLifetime:    30 * time.Minute,
			HealthCheckTimeout: 5 * time.Second,
		},
		NATS: NATS{
			URL:              "nats://127.0.0.1:4222",
			JetStream:        true,
			Stream:           "BOOKDB",
			BootstrapSubject: "bookdb.bootstrap.v1",
			PublishTimeout:   5 * time.Second,
		},
		Valkey: Valkey{
			URL:       "redis://127.0.0.1:6379/0",
			PoolSize:  20,
			KeyPrefix: "bookdb:",
		},
		OpenSearch: OpenSearch{
			URL:                "http://127.0.0.1:9200",
			Timeout:            10 * time.Second,
			IndexAliasWorks:    "bookdb_works",
			IndexAliasEditions: "bookdb_editions",
			IndexAliasPeople:   "bookdb_people",
			IndexAliasSeries:   "bookdb_series",
		},
		S3: S3{
			Endpoint:       "http://127.0.0.1:8333",
			Bucket:         "bookdb-assets",
			Region:         "us-east-1",
			ForcePathStyle: true,
			DisableSSL:     true,
		},
		Auth: Auth{
			LocalEnabled: true,
			OIDC:         OIDC{Enabled: false},
		},
		Sources: Sources{
			RegistryPath: "config/source-registry.yaml",
		},
		Observability: Obs{
			Enabled:          true,
			ServiceName:      "bookdb",
			TracesSampler:    "parentbased_always_on",
			MetricsEnabled:   true,
			MetricsNamespace: "bookdb",
			MetricsAddr:      ":9090",
		},
		FeatureFlags: FeatureFlags{
			OutboxEnabled:      true,
			SearchIndexEnabled: true,
			AssetPipeline:      true,
		},
	}
}

// Load reads configuration from (in order of precedence, highest last):
//  1. defaults
//  2. an optional YAML file at `path` (skipped if empty or not found)
//  3. environment variables matching the conventions in the docs
//
// It returns the resulting Config and any error encountered.
func Load(path string) (*Config, error) {
	cfg := Defaults()

	// 2. YAML file (optional) — recorded for diagnostics. The environment
	// (step 3) takes precedence over YAML. For M0 we do not parse the full
	// YAML graph here; full typed YAML support is an M1 item. We simply note
	// when the file is present so `bookdb doctor` can report it.
	if path != "" {
		abs := path
		if !filepath.IsAbs(abs) {
			if wd, err := os.Getwd(); err == nil {
				abs = filepath.Join(wd, path)
			}
		}
		if _, err := os.Stat(abs); err == nil {
			cfg.yamlProvided = true
		}
	}

	// 3. Environment variables override everything else.
	cfg.applyEnv()

	return &cfg, nil
}

// applyEnv overlays environment variables onto cfg.
func (cfg *Config) applyEnv() {
	cfg.Env = envString("BOOKDB_ENV", cfg.Env)
	cfg.Log.Level = envString("BOOKDB_LOG_LEVEL", cfg.Log.Level)
	cfg.Log.Format = envString("BOOKDB_LOG_FORMAT", cfg.Log.Format)
	cfg.API.Addr = envString("BOOKDB_API_ADDR", cfg.API.Addr)
	if v := envDuration("BOOKDB_API_READ_TIMEOUT"); v > 0 {
		cfg.API.ReadTimeout = v
	}
	if v := envDuration("BOOKDB_API_WRITE_TIMEOUT"); v > 0 {
		cfg.API.WriteTimeout = v
	}
	if v := envDuration("BOOKDB_API_SHUTDOWN_TIMEOUT"); v > 0 {
		cfg.API.ShutdownTimeout = v
	}

	cfg.Database.DSN = envString("BOOKDB_DATABASE_URL", cfg.Database.DSN)
	cfg.Database.DSNDirect = envString("BOOKDB_DATABASE_URL_DIRECT", cfg.Database.DSNDirect)
	cfg.Database.MaxOpenConns = envInt("BOOKDB_DB_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns)
	cfg.Database.MaxIdleConns = envInt("BOOKDB_DB_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns)
	if v := envDuration("BOOKDB_DB_CONN_MAX_LIFETIME"); v > 0 {
		cfg.Database.ConnMaxLifetime = v
	}

	cfg.NATS.URL = envString("BOOKDB_NATS_URL", cfg.NATS.URL)
	cfg.NATS.JetStream = envBool("BOOKDB_NATS_JETSTREAM_ENABLED", cfg.NATS.JetStream)
	cfg.NATS.Stream = envString("BOOKDB_NATS_STREAM_BOOKDB", cfg.NATS.Stream)
	cfg.NATS.BootstrapSubject = envString("BOOKDB_NATS_BOOTSTRAP_SUBJECT", cfg.NATS.BootstrapSubject)

	cfg.Valkey.URL = envString("BOOKDB_VALKEY_URL", cfg.Valkey.URL)
	cfg.Valkey.KeyPrefix = envString("BOOKDB_VALKEY_KEY_PREFIX", cfg.Valkey.KeyPrefix)

	cfg.OpenSearch.URL = envString("BOOKDB_OPENSEARCH_URL", cfg.OpenSearch.URL)
	cfg.OpenSearch.IndexAliasWorks = envString("BOOKDB_OPENSEARCH_INDEX_ALIAS_WORKS", cfg.OpenSearch.IndexAliasWorks)
	cfg.OpenSearch.IndexAliasEditions = envString("BOOKDB_OPENSEARCH_INDEX_ALIAS_EDITIONS", cfg.OpenSearch.IndexAliasEditions)
	cfg.OpenSearch.IndexAliasPeople = envString("BOOKDB_OPENSEARCH_INDEX_ALIAS_PEOPLE", cfg.OpenSearch.IndexAliasPeople)
	cfg.OpenSearch.IndexAliasSeries = envString("BOOKDB_OPENSEARCH_INDEX_ALIAS_SERIES", cfg.OpenSearch.IndexAliasSeries)

	cfg.S3.Endpoint = envString("BOOKDB_S3_ENDPOINT", cfg.S3.Endpoint)
	cfg.S3.AccessKey = envString("BOOKDB_S3_ACCESS_KEY", cfg.S3.AccessKey)
	cfg.S3.SecretKey = envString("BOOKDB_S3_SECRET_KEY", cfg.S3.SecretKey)
	// Secret references take precedence over inline values in production.
	if ref := os.Getenv("BOOKDB_S3_SECRET_KEY_REF"); ref != "" {
		cfg.S3.SecretKeyRef = ref
	}
	cfg.S3.Bucket = envString("BOOKDB_S3_BUCKET", cfg.S3.Bucket)
	cfg.S3.Region = envString("BOOKDB_S3_REGION", cfg.S3.Region)
	cfg.S3.ForcePathStyle = envBool("BOOKDB_S3_FORCE_PATH_STYLE", cfg.S3.ForcePathStyle)

	cfg.Auth.LocalEnabled = envBool("BOOKDB_AUTH_LOCAL_ENABLED", cfg.Auth.LocalEnabled)
	if os.Getenv("BOOKDB_OIDC_ISSUER") != "" {
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.Issuer = os.Getenv("BOOKDB_OIDC_ISSUER")
	}
	cfg.Auth.OIDC.ClientID = envString("BOOKDB_OIDC_CLIENT_ID", cfg.Auth.OIDC.ClientID)
	cfg.Auth.OIDC.ClientSecret = envString("BOOKDB_OIDC_CLIENT_SECRET", cfg.Auth.OIDC.ClientSecret)
	if ref := os.Getenv("BOOKDB_OIDC_CLIENT_SECRET_REF"); ref != "" {
		cfg.Auth.OIDC.ClientSecretRef = ref
	}
	cfg.Auth.OIDC.Audience = envString("BOOKDB_OIDC_AUDIENCE", cfg.Auth.OIDC.Audience)
	cfg.Auth.OIDC.Discovery = envBool("BOOKDB_OIDC_DISCOVERY", cfg.Auth.OIDC.Discovery)

	cfg.Sources.RegistryPath = envString("BOOKDB_SOURCE_REGISTRY_PATH", cfg.Sources.RegistryPath)

	cfg.Observability.Enabled = envBool("BOOKDB_OTEL_ENABLED", cfg.Observability.Enabled)
	cfg.Observability.OTLPExporterURL = envString("BOOKDB_OTEL_EXPORTER_OTLP_ENDPOINT", cfg.Observability.OTLPExporterURL)
	cfg.Observability.ServiceName = envString("BOOKDB_OTEL_SERVICE_NAME", cfg.Observability.ServiceName)
	cfg.Observability.TracesSampler = envString("BOOKDB_OTEL_TRACES_SAMPLER", cfg.Observability.TracesSampler)
	cfg.Observability.MetricsEnabled = envBool("BOOKDB_METRICS_ENABLED", cfg.Observability.MetricsEnabled)
	cfg.Observability.MetricsAddr = envString("BOOKDB_METRICS_ADDR", cfg.Observability.MetricsAddr)

	cfg.FeatureFlags.OutboxEnabled = envBool("BOOKDB_FEATURE_OUTBOX_ENABLED", cfg.FeatureFlags.OutboxEnabled)
	cfg.FeatureFlags.SearchIndexEnabled = envBool("BOOKDB_FEATURE_SEARCH_INDEX_ENABLED", cfg.FeatureFlags.SearchIndexEnabled)
	cfg.FeatureFlags.AssetPipeline = envBool("BOOKDB_FEATURE_ASSET_PIPELINE_ENABLED", cfg.FeatureFlags.AssetPipeline)
}

// Validate performs cheap, cross-field validation. It does not contact any
// external service (that is the job of `bookdb doctor` and the health checks).
func (c *Config) Validate() error {
	if c.Database.DSN == "" {
		return fmt.Errorf("config: database DSN is required (BOOKDB_DATABASE_URL)")
	}
	if c.NATS.URL == "" {
		return fmt.Errorf("config: NATS URL is required (BOOKDB_NATS_URL)")
	}
	if !strings.HasPrefix(c.NATS.URL, "nats://") && !strings.HasPrefix(c.NATS.URL, "nats+tls://") {
		return fmt.Errorf("config: NATS URL %q must start with nats://", c.NATS.URL)
	}

	if err := auth.ValidateSettings(auth.Settings{
		LocalEnabled: c.Auth.LocalEnabled,
		OIDC: auth.OIDCSettings{
			Enabled:         c.Auth.OIDC.Enabled,
			Issuer:          c.Auth.OIDC.Issuer,
			ClientID:        c.Auth.OIDC.ClientID,
			ClientSecret:    c.Auth.OIDC.ClientSecret,
			ClientSecretRef: c.Auth.OIDC.ClientSecretRef,
			Audience:        c.Auth.OIDC.Audience,
			Discovery:       c.Auth.OIDC.Discovery,
		},
	}); err != nil {
		return err
	}
	return nil
}

// Sanitized returns a config with secret-bearing fields redacted, for safe
// logging and health/diagnostic output.
func (c *Config) Sanitized() map[string]any {
	return map[string]any{
		"env":        c.Env,
		"log":        map[string]string{"level": c.Log.Level, "format": c.Log.Format},
		"api":        map[string]any{"addr": c.API.Addr, "read_timeout": c.API.ReadTimeout.String(), "write_timeout": c.API.WriteTimeout.String()},
		"database":   map[string]any{"dsn": redactDSN(c.Database.DSN), "dsn_direct": redactDSN(c.Database.DSNDirect), "max_open_conns": c.Database.MaxOpenConns},
		"nats":       map[string]any{"url": redactURL(c.NATS.URL), "jetstream": c.NATS.JetStream, "stream": c.NATS.Stream},
		"valkey":     map[string]any{"url": redactURL(c.Valkey.URL)},
		"opensearch": map[string]any{"url": redactURL(c.OpenSearch.URL), "timeout": c.OpenSearch.Timeout.String()},
		"s3":         map[string]any{"endpoint": redactURL(c.S3.Endpoint), "bucket": c.S3.Bucket, "region": c.S3.Region, "force_path_style": c.S3.ForcePathStyle},
		"auth": map[string]any{
			"local_enabled":  c.Auth.LocalEnabled,
			"oidc_enabled":   c.Auth.OIDC.Enabled,
			"oidc_issuer":    c.Auth.OIDC.Issuer,
			"oidc_client_id": c.Auth.OIDC.ClientID,
		},
		"sources": map[string]any{"registry_path": c.Sources.RegistryPath},
		"observability": map[string]any{
			"enabled":         c.Observability.Enabled,
			"service_name":    c.Observability.ServiceName,
			"metrics_enabled": c.Observability.MetricsEnabled,
			"metrics_addr":    c.Observability.MetricsAddr,
		},
	}
}

// redactDSN strips the userinfo (user:password@) from a DSN/URL for
// log-safe display. It never returns the password.
func redactDSN(dsn string) string {
	return redactURL(dsn)
}

// redactURL strips userinfo from a URL-like value while preserving the rest of
// the address for diagnostics.
func redactURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = url.UserPassword("****", "****")
	return parsed.String()
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	switch toLower3(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

func envInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func envDuration(key string) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		seconds := 0
		for _, r := range v {
			if r < '0' || r > '9' {
				return 0
			}
			seconds = seconds*10 + int(r-'0')
		}
		return time.Duration(seconds) * time.Second
	}
	return d
}

// toLower3 avoids importing strings (used to reduce dependency surface in this
// file; see logging's toLower for the same rationale).
func toLower3(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
