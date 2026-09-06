package s3

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/testsupport/devservices"
)

func TestComposeS3PutGetDelete(t *testing.T) {
	endpoint := devservices.EnvOrDefault("BOOKDB_S3_ENDPOINT", "http://127.0.0.1:8333")
	accessKey := devservices.EnvOrDefault("BOOKDB_S3_ACCESS_KEY", "bookdb")
	secretKey := devservices.EnvOrDefault("BOOKDB_S3_SECRET_KEY", "bookdb-dev-secret-do-not-use-in-prod")
	bucket := devservices.EnvOrDefault("BOOKDB_S3_BUCKET", "bookdb-assets")

	devservices.RequireTCP(t, "127.0.0.1:8333")

	store, err := New(Options{
		Endpoint:       endpoint,
		AccessKey:      accessKey,
		SecretKey:      secretKey,
		Region:         "us-east-1",
		Bucket:         bucket,
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if err := store.Check(ctx); err != nil {
		t.Fatalf("Check: %v", err)
	}

	key := "foundation/test-object.txt"
	info, err := store.Put(ctx, key, bytes.NewReader([]byte("bookdb")), int64(len("bookdb")), PutOptions{
		ContentType: "text/plain",
		Checksum:    &Checksum{Algorithm: "sha256", Value: "abc123"},
	})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if info.Checksum == nil || info.Checksum.Value != "abc123" {
		t.Fatalf("unexpected checksum info %#v", info.Checksum)
	}

	rc, got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "bookdb" {
		t.Fatalf("unexpected object payload %q", string(data))
	}
	if got.Checksum == nil || got.Checksum.Value != "abc123" {
		t.Fatalf("unexpected returned checksum %#v", got.Checksum)
	}

	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}