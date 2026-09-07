package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/auth"
	"github.com/bookdb/bookdb/internal/config"
	"github.com/bookdb/bookdb/internal/health"
	"github.com/bookdb/bookdb/internal/logging"
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
	fmt.Fprintln(stderr, "bookdb migrate is intentionally skeletal in M0; database migrations are not wired yet")
	return 2
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
	dependencyRegistry := newDependencyRegistry(cfg)

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

func newDependencyRegistry(cfg *config.Config) *health.Registry {
	registry := health.NewRegistry()
	registry.Register("database", health.CheckFunc(func(ctx context.Context) health.Status {
		return probeEndpoint(ctx, cfg.Database.DSN, "5432")
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
