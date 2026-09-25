package promotion

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/bookdb/bookdb/internal/database"
)

// openTestDB connects to the development database the same way the apikey
// integration tests do: BOOKDB_TEST_DATABASE_URL, or the documented local
// dev DSN. When no database is reachable the caller's tests skip, so
// `make test` stays green in gate environments without a database.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:***@127.0.0.1:5432/bookdb?sslmode=disable"
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

// cleanEntityTables empties the canonical/evidence tables the promotion
// pipeline writes to, so each test starts from a deterministic state. The
// local dev database is disposable (compose-reset semantics).
func cleanEntityTables(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"edition_contents",
		"editions",
		"expressions",
		"field_provenance",
		"canonical_revisions",
		"identifiers",
		"change_feed",
		"outbox",
		"people",
		"works",
		"source_records",
	}
	for _, tbl := range tables {
		if _, err := db.ExecContext(ctx, `TRUNCATE bookdb.`+tbl+` RESTART IDENTITY CASCADE`); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}
