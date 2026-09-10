package catalog

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/bookdb/bookdb/internal/database"
)

// testDSN returns the PostgreSQL DSN for integration tests, or skips the test
// when no database is available.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	return dsn
}

// openTestDB opens a PostgreSQL connection, applies all migrations, and loads
// the S1 fixtures. It skips the test when the database is unreachable.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := testDSN(t)

	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := database.RunUp(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := LoadFixtures(ctx, db); err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	return db
}
