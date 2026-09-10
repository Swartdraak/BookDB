package apikey

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/database"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := database.RunUp(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db := openTestDB(t)
	return NewStore(db, []byte("test-mac-key-000000000000000000000000"))
}

func TestCreateAndVerify(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	issued, err := store.Create(ctx, "test-key", []string{"catalog:read"}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if issued.Secret == "" {
		t.Fatal("expected a non-empty secret")
	}
	if len(issued.Secret) != 64 {
		t.Fatalf("expected 64 hex chars (256 bits), got %d", len(issued.Secret))
	}

	key, err := store.Verify(ctx, issued.Secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if key.KeyID != issued.KeyID {
		t.Fatalf("expected key %s, got %s", issued.KeyID, key.KeyID)
	}
	if len(key.Scopes) != 1 || key.Scopes[0] != "catalog:read" {
		t.Fatalf("expected scope catalog:read, got %v", key.Scopes)
	}
}

func TestVerify_InvalidSecret(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	issued, err := store.Create(ctx, "test-key", []string{"catalog:read"}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = issued

	_, err = store.Verify(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for invalid secret, got %v", err)
	}
}

func TestVerify_RevokedKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	issued, err := store.Create(ctx, "test-key", []string{"catalog:read"}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := store.Revoke(ctx, issued.KeyID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = store.Verify(ctx, issued.Secret)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for revoked key, got %v", err)
	}
}

func TestVerify_ExpiredKey(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	expired := time.Now().UTC().Add(-time.Hour)
	issued, err := store.Create(ctx, "test-key", []string{"catalog:read"}, nil, &expired)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = store.Verify(ctx, issued.Secret)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for expired key, got %v", err)
	}
}

func TestSecretNotStoredInClear(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	issued, err := store.Create(ctx, "test-key", []string{"catalog:read"}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The stored key_hash must not equal the plaintext secret.
	var storedHash string
	if err := store.db.QueryRowContext(ctx, `SELECT key_hash FROM bookdb.api_keys WHERE key_id = $1`, issued.KeyID).Scan(&storedHash); err != nil {
		t.Fatalf("query key_hash: %v", err)
	}
	if storedHash == issued.Secret {
		t.Fatal("secret must not be stored in clear")
	}
	// The lookup prefix must not be the full secret.
	if issued.LookupPrefix == issued.Secret {
		t.Fatal("lookup prefix must not be the full secret")
	}
}
