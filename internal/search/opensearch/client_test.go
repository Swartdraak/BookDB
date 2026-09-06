package opensearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIndexNameAndAliasName(t *testing.T) {
	if got := IndexName("Works", 1); got != "bookdb_works_v1" {
		t.Fatalf("unexpected index name %q", got)
	}
	if got := AliasName("Works"); got != "bookdb_works" {
		t.Fatalf("unexpected alias name %q", got)
	}
}

func TestCheckHitsClusterHealth(t *testing.T) {
	var sawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(server.URL, time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if sawPath != "/_cluster/health" {
		t.Fatalf("unexpected path %q", sawPath)
	}
}

func TestNewRejectsBadURLs(t *testing.T) {
	if _, err := New("not-a-url", time.Second); err == nil {
		t.Fatal("expected error for bad url")
	}
}
