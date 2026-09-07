// Package logging provides BookDB's structured logging foundation.
//
// BookDB uses the standard library `log/slog` so that logging behavior is
// predictable, dependency-light, and easy to route to different backends.
// All BookDB roles emit structured (JSON or key=value) logs suitable for
// aggregation and the observability pipeline. Log records must NEVER include
// secrets, credentials, or full source-record payloads (see the global agent
// contract: "Secrets never enter Git/logs/support bundles").
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type contextKey struct{}

var loggerKey = contextKey{}

// Level maps a string to a slog level.
type Level string

// Supported level names.
const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Options configures the logger.
type Options struct {
	// Level is a case-insensitive level name. Defaults to "info" when empty.
	Level string
	// Format is "json" or "text" (slog TextHandler). Defaults to "json".
	Format string
	// Writer receives log output. Defaults to os.Stderr when nil.
	Writer io.Writer
}

// Default is the shared process-level slog logger. It is replaced by New when
// a role initializes logging.
var Default = newDefault()

func newDefault() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// New builds a configured *slog.Logger from Options.
func New(opts Options) (*slog.Logger, error) {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stderr
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(normalizeLevel(opts.Level))); err != nil {
		return nil, fmt.Errorf("logging: unknown level %q: %w", opts.Level, err)
	}

	var h slog.Handler
	wopts := &slog.HandlerOptions{Level: lvl}
	switch normalizeFormat(opts.Format) {
	case "text":
		h = slog.NewTextHandler(writer, wopts)
	default:
		h = slog.NewJSONHandler(writer, wopts)
	}
	return slog.New(h), nil
}

func normalizeLevel(s string) string {
	switch s {
	case "":
		return string(LevelInfo)
	default:
		return toLower(s)
	}
}

func normalizeFormat(s string) string {
	switch toLower(s) {
	case "text":
		return "text"
	default:
		return "json"
	}
}

// toLower is a tiny local lowercaser to avoid importing strings for this
// (deliberately) dependency-light package.
func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// With is a convenience that returns a Default logger with additional
// attributes, for use before a role wires its own logger.
func With(args ...any) *slog.Logger { return Default.With(args...) }

// LoggerFrom retrieves a logger from a context, falling back to Default.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if lg, ok := ctx.Value(loggerKey).(*slog.Logger); ok && lg != nil {
		return lg
	}
	return Default
}

// NewContext returns a context carrying the given logger.
func NewContext(ctx context.Context, lg *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, lg)
}
