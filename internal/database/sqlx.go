package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver.
)

// Open opens a PostgreSQL connection pool using the pure-Go lib/pq driver.
// The DSN is a standard postgres:// URL. The returned *sql.DB satisfies the
// package DB interface, so it can be passed directly to the Migrator and the
// health checker.
func Open(ctx context.Context, dsn string, maxOpen, maxIdle int, maxLifetime time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	if maxLifetime > 0 {
		db.SetConnMaxLifetime(maxLifetime)
	}
	// Fail fast if the server is not reachable.
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return db, nil
}

// RunUp applies all pending embedded migrations to a concrete *sql.DB. It is
// the entry point used by the `bookdb migrate` command and readiness tooling,
// where a real database/sql handle is available rather than the narrow DB
// interface used by unit tests.
func RunUp(ctx context.Context, db *sql.DB) (StatusReport, error) {
	return DefaultMigrator().Up(ctx, &sqlMigrator{db: db})
}

// sqlMigrator adapts a concrete *sql.DB to the narrow DB interface used by the
// Migrator. The standard library's *sql.DB does not satisfy DB directly because
// its BeginTx returns *sql.Tx rather than the package Transaction interface.
type sqlMigrator struct{ db *sql.DB }

func (m *sqlMigrator) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

func (m *sqlMigrator) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

func (m *sqlMigrator) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	return m.db.QueryRowContext(ctx, query, args...)
}

func (m *sqlMigrator) BeginTx(ctx context.Context, opts *sql.TxOptions) (Transaction, error) {
	tx, err := m.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx}, nil
}

type sqlTx struct{ tx *sql.Tx }

func (t *sqlTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *sqlTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *sqlTx) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *sqlTx) Commit() error   { return t.tx.Commit() }
func (t *sqlTx) Rollback() error { return t.tx.Rollback() }
