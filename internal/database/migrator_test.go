package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"testing/fstest"
)

func TestLoad_SortsAndParsesFiles(t *testing.T) {
	migrator := Migrator{
		FS: fstest.MapFS{
			"0002_add_placeholder.sql": &fstest.MapFile{Data: []byte("CREATE SCHEMA IF NOT EXISTS bookdb;")},
			"0001_empty_database.sql":  &fstest.MapFile{Data: []byte("CREATE SCHEMA IF NOT EXISTS bookdb;")},
		},
		Directory: ".",
	}

	loaded, err := migrator.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(loaded))
	}
	if loaded[0].Version != "0001" || loaded[1].Version != "0002" {
		t.Fatalf("unexpected load order: %#v", loaded)
	}
	if loaded[0].Name != "empty_database" {
		t.Fatalf("unexpected migration name: %#v", loaded[0])
	}
}

func TestStatus_EmptyDatabaseReportsPending(t *testing.T) {
	migrator := Migrator{
		FS: fstest.MapFS{
			"0001_empty_database.sql": &fstest.MapFile{Data: []byte("CREATE SCHEMA IF NOT EXISTS bookdb;")},
		},
	}

	db := newFakeDB()
	status, err := migrator.Status(context.Background(), db)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.TrackingTablePresent {
		t.Fatal("tracking table should be absent on a clean database")
	}
	if len(status.Pending) != 1 {
		t.Fatalf("expected 1 pending migration, got %d", len(status.Pending))
	}
	if status.CurrentVersion != "" {
		t.Fatalf("unexpected current version %q", status.CurrentVersion)
	}
}

func TestUp_EmptyDatabaseBootstrapsAndRecordsMigration(t *testing.T) {
	migrator := Migrator{
		FS: fstest.MapFS{
			"0001_empty_database.sql": &fstest.MapFile{Data: []byte("CREATE SCHEMA IF NOT EXISTS bookdb;")},
		},
	}

	db := newFakeDB()
	status, err := migrator.Up(context.Background(), db)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if !status.TrackingTablePresent {
		t.Fatal("tracking table should be present after bootstrap")
	}
	if status.CurrentVersion != "0001" {
		t.Fatalf("unexpected current version %q", status.CurrentVersion)
	}
	if len(status.Pending) != 0 {
		t.Fatalf("expected no pending migrations, got %d", len(status.Pending))
	}
	if !db.schemaCreated {
		t.Fatal("bootstrap migration should create the application schema")
	}
	if !db.trackingTableCreated {
		t.Fatal("tracking table should be created before applying migrations")
	}
	if !db.applied["0001"] {
		t.Fatal("migration version 0001 should be recorded as applied")
	}
}

func TestPrepareCleanTestDatabase_ResetsSchemaAndMigrationState(t *testing.T) {
	db := newFakeDB()
	db.trackingTableCreated = true
	db.applied["0001"] = true
	db.schemaCreated = true

	if err := PrepareCleanTestDatabase(context.Background(), db, "bookdb_test"); err != nil {
		t.Fatalf("PrepareCleanTestDatabase: %v", err)
	}
	if !db.trackingTableCreated {
		t.Fatal("tracking table should remain available")
	}
	if len(db.applied) != 0 {
		t.Fatalf("expected migration state to be cleared, got %#v", db.applied)
	}
	if !db.schemaCreated {
		t.Fatal("test schema should be recreated")
	}
}

func TestHealthChecker(t *testing.T) {
	checker := NewHealthChecker(fakePinger{})
	if got := checker.Check(context.Background()); got != "ok" {
		t.Fatalf("expected ok health, got %q", got)
	}
	var nilPinger Pinger
	if got := NewHealthChecker(nilPinger).Check(context.Background()); got != "down" {
		t.Fatalf("expected down health for nil pinger, got %q", got)
	}
}

type fakePinger struct{}

func (fakePinger) PingContext(context.Context) error { return nil }

type fakeDB struct {
	trackingTableCreated bool
	schemaCreated        bool
	applied              map[string]bool
	lastExec             []string
}

func newFakeDB() *fakeDB { return &fakeDB{applied: map[string]bool{}} }

func (db *fakeDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.lastExec = append(db.lastExec, query)
	switch {
	case strings.HasPrefix(strings.TrimSpace(query), "CREATE TABLE IF NOT EXISTS public.bookdb_migrations"):
		db.trackingTableCreated = true
	case strings.HasPrefix(strings.TrimSpace(query), "DELETE FROM public.bookdb_migrations"):
		db.applied = map[string]bool{}
	case strings.HasPrefix(strings.TrimSpace(query), "DROP SCHEMA IF EXISTS"):
		db.schemaCreated = false
	case strings.HasPrefix(strings.TrimSpace(query), "CREATE SCHEMA IF NOT EXISTS"):
		db.schemaCreated = true
	case strings.HasPrefix(strings.TrimSpace(query), "INSERT INTO public.bookdb_migrations"):
		if len(args) >= 1 {
			if version, ok := args[0].(string); ok {
				db.applied[version] = true
			}
		}
	}
	return fakeResult(1), nil
}

func (db *fakeDB) QueryContext(_ context.Context, query string, _ ...any) (Rows, error) {
	switch {
	case strings.Contains(query, "FROM public.bookdb_migrations"):
		return &fakeRows{rows: db.appliedRows()}, nil
	default:
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
}

func (db *fakeDB) QueryRowContext(_ context.Context, query string, _ ...any) Row {
	if strings.Contains(query, "to_regclass") {
		if db.trackingTableCreated {
			return fakeRow{value: "public.bookdb_migrations"}
		}
		return fakeRow{value: nil}
	}
	return fakeRow{err: fmt.Errorf("unexpected row query: %s", query)}
}

func (db *fakeDB) BeginTx(context.Context, *sql.TxOptions) (Transaction, error) {
	return &fakeTx{db: db}, nil
}

func (db *fakeDB) appliedRows() [][]any {
	rows := make([][]any, 0, len(db.applied))
	for version := range db.applied {
		rows = append(rows, []any{version, "empty_database", time.Unix(0, 0)})
	}
	return rows
}

type fakeTx struct{ db *fakeDB }

func (tx *fakeTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.db.ExecContext(ctx, query, args...)
}

func (tx *fakeTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return tx.db.QueryContext(ctx, query, args...)
}

func (tx *fakeTx) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	return tx.db.QueryRowContext(ctx, query, args...)
}

func (tx *fakeTx) Commit() error   { return nil }
func (tx *fakeTx) Rollback() error { return nil }

type fakeRow struct {
	value any
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("fakeRow: expected one destination")
	}
	switch target := dest[0].(type) {
	case *sql.NullString:
		if r.value == nil {
			*target = sql.NullString{}
			return nil
		}
		*target = sql.NullString{String: r.value.(string), Valid: true}
		return nil
	case *string:
		if r.value == nil {
			*target = ""
			return nil
		}
		*target = r.value.(string)
		return nil
	default:
		return fmt.Errorf("fakeRow: unsupported destination %T", dest[0])
	}
}

type fakeRows struct {
	rows [][]any
	idx  int
}

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.rows[r.idx-1]
	if len(dest) != len(row) {
		return fmt.Errorf("fakeRows: expected %d destinations, got %d", len(row), len(dest))
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = row[i].(string)
		case *time.Time:
			*target = row[i].(time.Time)
		default:
			return fmt.Errorf("fakeRows: unsupported destination %T", dest[i])
		}
	}
	return nil
}

func (r *fakeRows) Close() error { return nil }
func (r *fakeRows) Err() error   { return nil }

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) { return int64(r), nil }
