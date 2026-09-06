package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_JsonFormat(t *testing.T) {
	var buf bytes.Buffer
	lg, err := New(Options{Level: "debug", Format: "json", Writer: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Debug("hello", "key", "value")
	out := buf.String()
	if !strings.Contains(out, `"level":"DEBUG"`) {
		t.Fatalf("expected JSON DEBUG level, got %q", out)
	}
	if !strings.Contains(out, `"key":"value"`) {
		t.Fatalf("expected attributes in JSON, got %q", out)
	}
	// must be valid JSON
	var m map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

func TestNew_DebugLevelFilter(t *testing.T) {
	// A logger at "info" level should not emit Debug records.
	var buf bytes.Buffer
	lg, err := New(Options{Level: "info", Format: "json", Writer: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Debug("should be filtered")
	if buf.Len() != 0 {
		t.Fatalf("DEBUG record should be filtered by INFO logger, got %q", buf.String())
	}
	lg.Info("should be emitted")
	if !strings.Contains(buf.String(), `"level":"INFO"`) {
		t.Fatalf("expected INFO record, got %q", buf.String())
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	lg, err := New(Options{Level: "info", Format: "text", Writer: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	lg.Warn("warning")
	out := buf.String()
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("expected text WARN, got %q", out)
	}
}

func TestNew_UnknownLevelErrors(t *testing.T) {
	if _, err := New(Options{Level: "nope"}); err == nil {
		t.Fatal("expected error for unknown level")
	}
}

func TestContext_RoundTrip(t *testing.T) {
	lg, err := New(Options{Level: "info"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := NewContext(context.Background(), lg)
	got := LoggerFrom(ctx)
	if got != lg {
		t.Fatal("LoggerFrom did not return the stored logger")
	}
	// fallback
	if LoggerFrom(context.Background()) == nil {
		t.Fatal("LoggerFrom should fall back to Default")
	}
}
