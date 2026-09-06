package opensearch

import (
	"context"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/testsupport/devservices"
)

func TestComposeOpenSearchConnectivity(t *testing.T) {
	url := devservices.EnvOrDefault("BOOKDB_OPENSEARCH_URL", "http://127.0.0.1:9200")
	devservices.RequireHTTP(t, url, "/_cluster/health")

	client, err := New(url, 5*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Check(context.Background()); err != nil {
		t.Fatalf("Check: %v", err)
	}
}
