package runtime

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/api"
	"github.com/bookdb/bookdb/internal/apikey"
	"github.com/bookdb/bookdb/internal/auth"
	"github.com/bookdb/bookdb/internal/catalog"
	"github.com/bookdb/bookdb/internal/config"
	"github.com/bookdb/bookdb/internal/database"
	"github.com/bookdb/bookdb/internal/health"
	"github.com/bookdb/bookdb/internal/logging"
	"github.com/bookdb/bookdb/internal/ratelimit"
	"github.com/bookdb/bookdb/internal/version"
)

var workerKinds = map[string]struct{}{
	"ingest":    {},
	"normalize": {},
	"identity":  {},
	"reconcile": {},
	"publish":   {},
	"index":     {},
	"assets":    {},
}

// Run dispatches the BookDB CLI and returns an exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	case "version":
		return runVersion(stdout)
	case "doctor":
		return runDoctor(stdout, stderr)
	case "migrate":
		return runMigrate(stderr)
	case "key":
		return runKey(ctx, args[1:], stdout, stderr)
	case "fixtures":
		return runFixtures(args[1:], stdout, stderr)
	case "api":
		return runService(ctx, "api", stdout, stderr)
	case "scheduler":
		return runService(ctx, "scheduler", stdout, stderr)
	case "worker":
		return runWorker(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "bookdb: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: bookdb <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  api")
	fmt.Fprintln(w, "  scheduler")
	fmt.Fprintln(w, "  worker <ingest|normalize|identity|reconcile|publish|index|assets>")
	fmt.Fprintln(w, "  migrate")
	fmt.Fprintln(w, "  doctor")
	fmt.Fprintln(w, "  version")
}

func runVersion(stdout io.Writer) int {
	info := version.Get()
	fmt.Fprintf(stdout, "%s %s (%s, %s, %s)\n", info.Name, info.Version, info.Commit, info.Date, info.Go)
	return 0
}

func runDoctor(stdout, stderr io.Writer) int {
	cfg, logger, err := loadRuntimeConfig()
	if err != nil {
		diagnostics := map[string]any{
			"valid":   false,
			"error":   err.Error(),
			"version": version.Get(),
		}
		_ = writeJSON(stdout, diagnostics)
		return 1
	}
	_ = logger

	validationErr := cfg.Validate()
	diagnostics := map[string]any{
		"valid":          validationErr == nil,
		"config_path":    configPath(),
		"yaml_provided":  cfg.YAMLProvided(),
		"sanitized":      cfg.Sanitized(),
		"version":        version.Get(),
		"validation_err": nil,
	}
	if validationErr != nil {
		diagnostics["validation_err"] = validationErr.Error()
		_ = writeJSON(stdout, diagnostics)
		fmt.Fprintln(stderr, validationErr)
		return 1
	}
	if err := writeJSON(stdout, diagnostics); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runMigrate(stderr io.Writer) int {
	cfg, _, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	// Migrations run against the direct (non-pooled) DSN so they are not
	// subject to transaction-mode pooling. Fall back to the pooled DSN when no
	// direct URL is configured.
	dsn := cfg.Database.DSNDirect
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.Database.DSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Open(ctx, dsn, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb migrate: %v\n", err)
		return 1
	}
	defer db.Close()

	report, err := database.RunUp(ctx, db)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb migrate: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "bookdb migrate: current version %q, %d applied, %d pending\n",
		report.CurrentVersion, len(report.Applied), len(report.Pending))
	return 0
}

func runWorker(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "bookdb worker requires one of: ingest, normalize, identity, reconcile, publish, index, assets")
		return 2
	}
	if _, ok := workerKinds[args[0]]; !ok {
		fmt.Fprintf(stderr, "bookdb worker: unknown kind %q\n", args[0])
		return 2
	}
	return runService(ctx, "worker "+args[0], stdout, stderr)
}

func runService(ctx context.Context, name string, stdout, stderr io.Writer) int {
	cfg, logger, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	server, listenAddr, err := newRuntimeServer(name, cfg, logger)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	logger.Info("bookdb service listening", "command", name, "addr", listenAddr)
	fmt.Fprintf(stderr, "bookdb %s listening on %s\n", name, listenAddr)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("bookdb service failed", "command", name, "error", err)
			fmt.Fprintf(stderr, "bookdb %s: %v\n", name, err)
			return 1
		}
		return 0
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("bookdb shutdown failed", "command", name, "error", err)
			fmt.Fprintf(stderr, "bookdb %s shutdown: %v\n", name, err)
			return 1
		}
		logger.Info("bookdb service stopped", "command", name)
		return 0
	}
}

func loadRuntimeConfig() (*config.Config, *slog.Logger, error) {
	cfg, err := config.Load(configPath())
	if err != nil {
		return nil, nil, err
	}
	logger, err := logging.New(logging.Options{Level: cfg.Log.Level, Format: cfg.Log.Format})
	if err != nil {
		return nil, nil, err
	}
	return cfg, logger, nil
}

func configPath() string {
	if path := os.Getenv("BOOKDB_CONFIG_PATH"); path != "" {
		return path
	}
	return "config/bookdb.yaml"
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

type runtimeServer struct {
	server    *http.Server
	startedAt time.Time
}

func newRuntimeServer(name string, cfg *config.Config, logger *slog.Logger) (*http.Server, string, error) {
	addr := cfg.API.Addr
	if name != "api" {
		addr = cfg.Observability.MetricsAddr
	}
	if strings.TrimSpace(addr) == "" {
		return nil, "", fmt.Errorf("bookdb %s: listen address is empty", name)
	}

	startedAt := time.Now().UTC()
	liveRegistry := health.NewRegistry()
	liveRegistry.Register("runtime", health.CheckFunc(func(context.Context) health.Status {
		return health.StatusOK
	}), true)
	dbPool := openDatabasePool(context.Background(), cfg)
	dependencyRegistry := newDependencyRegistry(cfg, dbPool)

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeHealth(w, liveRegistry, version.Get().Version, health.StatusOK)
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		writeDependencyHealth(w, dependencyRegistry, version.Get().Version)
	})
	mux.HandleFunc("/health/startup", func(w http.ResponseWriter, r *http.Request) {
		writeDependencyHealth(w, dependencyRegistry, version.Get().Version)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		writeMetrics(w, startedAt)
	})
	mux.HandleFunc("/auth/mode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(authModePayload(cfg))
	})
	mux.HandleFunc("/auth/oidc", func(w http.ResponseWriter, r *http.Request) {
		settings := authSettings(cfg)
		if !settings.OIDC.Enabled {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(auth.PublicOIDC(settings))
	})
	// S1 key-protected canonical catalog API. Only mounted for the api role
	// and when a database pool is available.
	if name == "api" && dbPool != nil {
		limiter := newRateLimiter(context.Background(), cfg)
		catalogAPI := api.NewServer(dbPool, apiMacKey(cfg), limiter)
		mux.Handle("/api/v1/", catalogAPI.Handler())
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service":   name,
			"version":   version.Get(),
			"status":    "bookdb backend skeleton",
			"auth_mode": auth.ResolveMode(authSettings(cfg)),
		})
	})

	_ = logger
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server, addr, nil
}

func writeHealth(w http.ResponseWriter, registry *health.Registry, versionString string, status health.Status) {
	report := registry.Result(context.Background(), versionString)
	report.Status = string(status)
	writeHealthReport(w, report)
}

func writeDependencyHealth(w http.ResponseWriter, registry *health.Registry, versionString string) {
	report := registry.Result(context.Background(), versionString)
	writeHealthReport(w, report)
}

func newDependencyRegistry(cfg *config.Config, db *sql.DB) *health.Registry {
	registry := health.NewRegistry()
	registry.Register("database", health.CheckFunc(func(ctx context.Context) health.Status {
		return checkDatabase(ctx, db, cfg.Database.DSN)
	}), true)
	registry.Register("nats", health.CheckFunc(func(ctx context.Context) health.Status {
		return probeEndpoint(ctx, cfg.NATS.URL, "4222")
	}), false)
	registry.Register("valkey", health.CheckFunc(func(ctx context.Context) health.Status {
		return probeEndpoint(ctx, cfg.Valkey.URL, "6379")
	}), false)
	registry.Register("opensearch", health.CheckFunc(func(ctx context.Context) health.Status {
		return probeEndpoint(ctx, cfg.OpenSearch.URL, "9200")
	}), false)
	registry.Register("s3", health.CheckFunc(func(ctx context.Context) health.Status {
		return probeEndpoint(ctx, cfg.S3.Endpoint, "8333")
	}), false)
	return registry
}

// openDatabasePool opens a PostgreSQL pool for readiness checks. It returns
// nil when the DSN is empty or the pool cannot be created; readiness then
// falls back to a TCP probe so a misconfigured environment still reports
// down rather than panicking.
func openDatabasePool(ctx context.Context, cfg *config.Config) *sql.DB {
	dsn := cfg.Database.DSN
	if strings.TrimSpace(dsn) == "" {
		return nil
	}
	db, err := database.Open(ctx, dsn, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		return nil
	}
	return db
}

// checkDatabase reports readiness based on a real query. When a live pool is
// available it runs SELECT 1; otherwise it falls back to a TCP probe so the
// endpoint still distinguishes reachable-but-unqueryable from unreachable.
func checkDatabase(ctx context.Context, db *sql.DB, dsn string) health.Status {
	if db != nil {
		if err := db.PingContext(ctx); err != nil {
			return health.StatusDown
		}
		var one int
		if err := db.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
			return health.StatusDown
		}
		return health.StatusOK
	}
	return probeEndpoint(ctx, dsn, "5432")
}

func probeEndpoint(ctx context.Context, raw, defaultPort string) health.Status {
	address, err := normalizeEndpoint(raw, defaultPort)
	if err != nil {
		return health.StatusDown
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return health.StatusDown
	}
	_ = conn.Close()
	return health.StatusOK
}

func normalizeEndpoint(raw, defaultPort string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty endpoint")
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		host := parsed.Hostname()
		if host == "" {
			return "", fmt.Errorf("missing host")
		}
		port := parsed.Port()
		if port == "" {
			port = defaultPort
		}
		if port == "" {
			return "", fmt.Errorf("missing port")
		}
		return net.JoinHostPort(host, port), nil
	}
	if host, port, err := net.SplitHostPort(raw); err == nil {
		if host == "" {
			return "", fmt.Errorf("missing host")
		}
		return net.JoinHostPort(host, port), nil
	}
	if defaultPort == "" {
		return "", fmt.Errorf("missing port")
	}
	return net.JoinHostPort(raw, defaultPort), nil
}

func writeHealthReport(w http.ResponseWriter, report health.Result) {
	w.Header().Set("Content-Type", "application/json")
	if health.Status(report.Status) != health.StatusDown {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	_ = json.NewEncoder(w).Encode(report)
}

func writeMetrics(w http.ResponseWriter, startedAt time.Time) {
	info := version.Get()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintln(w, "# HELP bookdb_build_info BookDB build information")
	fmt.Fprintln(w, "# TYPE bookdb_build_info gauge")
	fmt.Fprintf(w, "bookdb_build_info{version=%q,commit=%q,channel=%q,go=%q,platform=%q} 1\n", info.Version, info.Commit, info.Channel, info.Go, info.Platform)
	fmt.Fprintln(w, "# HELP bookdb_process_uptime_seconds Process uptime")
	fmt.Fprintln(w, "# TYPE bookdb_process_uptime_seconds gauge")
	fmt.Fprintf(w, "bookdb_process_uptime_seconds %.0f\n", time.Since(startedAt).Seconds())
}

func authSettings(cfg *config.Config) auth.Settings {
	return auth.Settings{
		LocalEnabled: cfg.Auth.LocalEnabled,
		OIDC: auth.OIDCSettings{
			Enabled:         cfg.Auth.OIDC.Enabled,
			Issuer:          cfg.Auth.OIDC.Issuer,
			ClientID:        cfg.Auth.OIDC.ClientID,
			ClientSecret:    cfg.Auth.OIDC.ClientSecret,
			ClientSecretRef: cfg.Auth.OIDC.ClientSecretRef,
			Audience:        cfg.Auth.OIDC.Audience,
			Discovery:       cfg.Auth.OIDC.Discovery,
		},
	}
}

func authModePayload(cfg *config.Config) map[string]any {
	settings := authSettings(cfg)
	return map[string]any{
		"mode": auth.ResolveMode(settings),
		"oidc": auth.PublicOIDC(settings),
	}
}

// apiMacKey derives the server-side key used to HMAC API key secrets. It is
// sourced from BOOKDB_API_KEY_MAC (hex or raw) and falls back to a
// development-only constant. Production deployments must set a strong,
// secret value; the fallback is for local development only.
func apiMacKey(cfg *config.Config) []byte {
	if v := os.Getenv("BOOKDB_API_KEY_MAC"); v != "" {
		if decoded, err := hex.DecodeString(v); err == nil && len(decoded) >= 32 {
			return decoded
		}
		return []byte(v)
	}
	// Development-only fallback. Never use in production.
	return []byte("bookdb-dev-api-key-mac-key-0000000000")
}

// newRateLimiter creates a shared Valkey-backed rate limiter for the API.
// It returns nil when Valkey is not configured or unreachable, which disables
// rate limiting (the API still works, just without the shared quota).
func newRateLimiter(ctx context.Context, cfg *config.Config) *ratelimit.Limiter {
	if strings.TrimSpace(cfg.Valkey.URL) == "" {
		return nil
	}
	client, err := ratelimit.NewValkeyClient(ctx, cfg.Valkey.URL)
	if err != nil {
		return nil
	}
	limit := 100 // requests per window per key
	window := time.Minute
	return ratelimit.New(client, cfg.Valkey.KeyPrefix, limit, window)
}

// runKey handles the `bookdb key create` subcommand.
func runKey(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "create" {
		fmt.Fprintln(stderr, "Usage: bookdb key create --name <name> --scope <scope>")
		return 2
	}
	fs := flag.NewFlagSet("key create", flag.ExitOnError)
	name := fs.String("name", "dev-key", "Key name")
	scope := fs.String("scope", "catalog:read", "Key scope")
	_ = fs.Parse(args[1:])

	cfg, _, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	dsn := cfg.Database.DSNDirect
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.Database.DSN
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	db, err := database.Open(ctx, dsn, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb key: %v\n", err)
		return 1
	}
	defer db.Close()

	store := apikey.NewStore(db, apiMacKey(cfg))
	issued, err := store.Create(ctx, *name, []string{*scope}, nil, nil)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb key: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Key ID:     %s\n", issued.KeyID)
	fmt.Fprintf(stdout, "Name:       %s\n", issued.Name)
	fmt.Fprintf(stdout, "Scopes:     %s\n", strings.Join(issued.Scopes, ", "))
	fmt.Fprintf(stdout, "Secret:     %s\n", issued.Secret)
	fmt.Fprintf(stdout, "\nSave this secret now — it will not be shown again.\n")
	return 0
}

// runFixtures loads the S1 synthetic fixture catalog into the database, or
// verifies it read-only with --verify.
func runFixtures(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fixtures", flag.ExitOnError)
	verify := fs.Bool("verify", false, "Read-only verification of the fixture graph (no writes)")
	_ = fs.Parse(args)

	cfg, _, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	dsn := cfg.Database.DSNDirect
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.Database.DSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Open(ctx, dsn, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb fixtures: %v\n", err)
		return 1
	}
	defer db.Close()

	if *verify {
		rep, err := catalog.VerifyFixtures(ctx, db)
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
		if err != nil {
			fmt.Fprintf(stderr, "bookdb fixtures --verify: %v\n", err)
			return 1
		}
		fmt.Fprintln(stderr, "bookdb fixtures --verify: OK")
		return 0
	}

	if err := catalog.LoadFixtures(ctx, db); err != nil {
		fmt.Fprintf(stderr, "bookdb fixtures: %v\n", err)
		return 1
	}
	fmt.Fprintln(stderr, "bookdb fixtures: loaded successfully")
	return 0
}
