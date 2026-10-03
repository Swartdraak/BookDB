package database

import (
	"context"
)

// PrepareCleanTestDatabase resets the migration bookkeeping and recreates
// the requested application schema so integration tests can start from a
// clean PostgreSQL database without assuming the canonical schema already
// exists.
//
// NOTE: this signature takes the narrow DB interface so in-package unit
// tests can exercise the reset logic against a fake. It does NOT take the
// cross-package advisory lock (the interface cannot hand out a dedicated
// stdlib connection for the lock session). Live integration tests share a
// single disposable database across packages, so they MUST use
// PrepareCleanStdlibTestDatabase or PrepareCleanAndMigrate, which wrap
// the same reset in the TestResetLockKey advisory lock and serialize
// concurrent package mutations (issue #68).
func PrepareCleanTestDatabase(ctx context.Context, db DB, schema string) error {
	if schema == "" {
		schema = DefaultApplicationSchema
	}
	quotedSchema, err := quoteIdent(schema)
	if err != nil {
		return err
	}
	return runTestReset(ctx, db, quotedSchema)
}
