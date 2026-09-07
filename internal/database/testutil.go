package database

import (
	"context"
	"fmt"
)

// PrepareCleanTestDatabase resets the migration bookkeeping and recreates the
// requested application schema so integration tests can start from a clean
// PostgreSQL database without assuming the canonical schema already exists.
func PrepareCleanTestDatabase(ctx context.Context, db DB, schema string) error {
	if schema == "" {
		schema = DefaultApplicationSchema
	}
	quotedSchema, err := quoteIdent(schema)
	if err != nil {
		return err
	}

	if err := ensureTrackingTable(ctx, db, DefaultTrackingTable); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin test reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM public.bookdb_migrations`); err != nil {
		return fmt.Errorf("database: clear migration state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quotedSchema)); err != nil {
		return fmt.Errorf("database: drop test schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, quotedSchema)); err != nil {
		return fmt.Errorf("database: create test schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: commit test reset: %w", err)
	}
	return nil
}
