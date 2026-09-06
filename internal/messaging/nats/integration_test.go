package natsbus

import (
	"context"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/testsupport/devservices"
	nats "github.com/nats-io/nats.go"
)

func TestConnectAndBootstrapDevStreamAgainstCompose(t *testing.T) {
	url := devservices.EnvOrDefault("BOOKDB_NATS_URL", "nats://127.0.0.1:4222")
	devservices.RequireTCP(t, "127.0.0.1:4222")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := Connect(ctx, url, nats.Timeout(3*time.Second), nats.Name("bookdb-foundation-test"))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	js, err := client.JetStream()
	if err != nil {
		t.Fatalf("JetStream: %v", err)
	}
	if err := BootstrapDevStream(ctx, js, "BOOKDB"); err != nil {
		t.Fatalf("BootstrapDevStream: %v", err)
	}
}
