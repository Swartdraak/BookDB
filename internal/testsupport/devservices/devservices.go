// Package devservices contains small helpers for tests that exercise the
// development compose services when they are available.
package devservices

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// EnvOrDefault returns the environment value when set, or the fallback.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RequireTCP skips the test unless the address is reachable.
func RequireTCP(t *testing.T, address string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Skipf("service not reachable at %s: %v", address, err)
	}
	_ = conn.Close()
}

// RequireHTTP skips the test unless the endpoint responds successfully.
func RequireHTTP(t *testing.T, rawURL, path string) {
	t.Helper()
	if rawURL == "" {
		t.Skip("HTTP service URL is empty")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Skipf("invalid url %q: %v", rawURL, err)
	}
	clone := *parsed
	clone.Path = path
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clone.String(), nil)
	if err != nil {
		t.Skipf("request build failed: %v", err)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Skipf("service not reachable at %s: %v", clone.String(), err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		t.Skipf("service returned non-usable status %s", resp.Status)
	}
}
