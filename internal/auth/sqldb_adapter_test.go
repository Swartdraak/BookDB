package auth

import (
	"context"
	"database/sql"

	"github.com/bookdb/bookdb/internal/database"
)

// Thin adapters exposing stdlib *sql.DB / *sql.Tx through the database
// package's narrow interfaces, for test setup only (PrepareCleanTestDatabase
// accepts the database.DB interface; the stdlib types satisfy the same
// surface structurally).
type sqlDBAdapter struct{ db *sql.DB }

func (a *sqlDBAdapter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return a.db.ExecContext(ctx, query, args...)
}
func (a *sqlDBAdapter) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
func (a *sqlDBAdapter) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return a.db.QueryRowContext(ctx, query, args...)
}
func (a *sqlDBAdapter) BeginTx(ctx context.Context, opts *sql.TxOptions) (database.Transaction, error) {
	tx, err := a.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTxAdapter{tx}, nil
}

type sqlTxAdapter struct{ tx *sql.Tx }

func (a *sqlTxAdapter) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return a.tx.ExecContext(ctx, query, args...)
}
func (a *sqlTxAdapter) QueryContext(ctx context.Context, query string, args ...any) (database.Rows, error) {
	rows, err := a.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
func (a *sqlTxAdapter) QueryRowContext(ctx context.Context, query string, args ...any) database.Row {
	return a.tx.QueryRowContext(ctx, query, args...)
}
func (a *sqlTxAdapter) Commit() error   { return a.tx.Commit() }
func (a *sqlTxAdapter) Rollback() error { return a.tx.Rollback() }
