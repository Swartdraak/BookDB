package database

import (
	"context"
	"database/sql"
	"fmt"
)

// TestResetLockKey is the PostgreSQL advisory lock key that serializes
// one disposable database: migration-state clear + DROP/CREATE SCHEMA
// resets AND the migration DDL that (re)applies the schema after a reset.
//
// Two distinct hazards are serialized (issue #68):
//
//  1. Reset vs query: one package's DROP/CREATE SCHEMA interleaving with
//     another package's mid-flight queries ->
//     `pq: relation "bookdb.<table>" does not exist (42P01)`.
//
//  2. Migration vs reset: migration DDL (CREATE SCHEMA / CREATE TABLE)
//     racing with another package's reset ->
//     `pq: referenced schema was concurrently dropped (42704)`.
//
// It is deliberately a DIFFERENT key from the migration lock used by
// RunUp (734211990). A reset-with-migrations holds BOTH locks — the test
// lock first, then the migration lock while RunUp applies DDL — so a
// reset is a strict superset of a plain migration. All callers therefore
// acquire the locks in the same order (test -> migration), which keeps
// the pair deadlock-free.
const TestResetLockKey = 734212991

// lockedTx is a transaction whose Begin also holds the schema-mutation
// advisory lock on the SAME connection the transaction uses. Holding the
// lock on the transaction's own connection is what makes the critical
// section atomic: PostgreSQL cannot run DDL in a session that has been
// dropped out from under, and a concurrent reset on a DIFFERENT
// connection blocks on the lock for the entire duration of our
// transaction (reset + commit), not just the single reset statement.
type lockedTx struct {
	tx   *sql.Tx
	conn *sql.Conn
}

func (l *lockedTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return l.tx.ExecContext(ctx, query, args...)
}

func (l *lockedTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return l.tx.QueryContext(ctx, query, args...)
}

func (l *lockedTx) QueryRowContext(ctx context.Context, query string, args ...any) Row {
	return l.tx.QueryRowContext(ctx, query, args...)
}

func (l *lockedTx) Commit() error {
	if err := l.tx.Commit(); err != nil {
		return err
	}
	if _, uerr := l.conn.ExecContext(context.Background(), fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, TestResetLockKey)); uerr != nil {
		// Best effort.
		_ = uerr
	}
	_ = l.conn.Close()
	return nil
}

func (l *lockedTx) Rollback() error {
	if err := l.tx.Rollback(); err != nil {
		return err
	}
	if _, uerr := l.conn.ExecContext(context.Background(), fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, TestResetLockKey)); uerr != nil {
		_ = uerr
	}
	_ = l.conn.Close()
	return nil
}

// withTestLock runs fn while holding the schema-mutation advisory lock on
// a dedicated connection checked out of the stdlib pool. The locked
// connection is passed to fn so a critical section that needs a
// transaction can begin it on the SAME session that owns the lock. This is
// what makes the critical section atomic and deadlock-free: PostgreSQL
// advisory locks are per-session, so a second connection trying to
// re-acquire the same key would block forever (issue #68, WIP deadlock).
// The lock is released explicitly before the connection returns to the
// pool.
func withTestLock(ctx context.Context, db *sql.DB, fn func(conn *sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("database: check out test-lock session: %w", err)
	}
	defer func() {
		if _, uerr := conn.ExecContext(context.Background(), fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, TestResetLockKey)); uerr != nil {
			_ = uerr
		}
		_ = conn.Close()
	}()
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`SELECT pg_advisory_lock(%d)`, TestResetLockKey)); err != nil {
		return fmt.Errorf("database: acquire test lock: %w", err)
	}
	return fn(conn)
}

// runTestReset performs the reset statements (migration-state clear +
// DROP/CREATE SCHEMA) inside a single transaction. The CREATE TABLE for
// the migration tracking table runs BEFORE the DELETE because
// DROP SCHEMA ... CASCADE drops every object in the schema, including
// bookdb_migrations — a fresh reset must recreate the table before the
// DELETE can find it (issue #68).
func runTestReset(ctx context.Context, db DB, quotedSchema string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin test reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version text PRIMARY KEY,
		name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`, DefaultTrackingTable)); err != nil {
		return fmt.Errorf("database: ensure migration table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM public.bookdb_migrations`); err != nil {
		return fmt.Errorf("database: clear migration state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quotedSchema)); err != nil {
		return fmt.Errorf("database: drop test schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, quotedSchema)); err != nil {
		return fmt.Errorf("database: create test schema: %w", err)
	}
	return tx.Commit()
}

// PrepareCleanStdlibTestDatabase resets the migration bookkeeping and
// recreates the requested application schema, serialized across packages
// by the test lock held on the reset transaction's own connection. It is
// the cross-package, advisory-locked variant of PrepareCleanTestDatabase.
// Migration re-application is NOT done here; use
// PrepareCleanAndMigrate for a clean, fully-migrated schema.
func PrepareCleanStdlibTestDatabase(ctx context.Context, db *sql.DB, schema string) error {
	if schema == "" {
		schema = DefaultApplicationSchema
	}
	quotedSchema, err := quoteIdent(schema)
	if err != nil {
		return err
	}
	return withTestLock(ctx, db, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			version text PRIMARY KEY,
			name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`, DefaultTrackingTable)); err != nil {
			return fmt.Errorf("database: ensure migration table: %w", err)
		}
		return runLockedReset(ctx, conn, quotedSchema)
	})
}

// runLockedReset performs the reset statements inside a single
// advisory-locked transaction whose connection owns the lock (see
// beginLockedTx). The transaction is begun on conn, which is the SAME
// session that holds TestResetLockKey — no second connection is checked
// out and no second lock acquisition happens. This is what makes the
// reset atomic and deadlock-free across concurrent packages (issue #68).
func runLockedReset(ctx context.Context, conn *sql.Conn, quotedSchema string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, TestResetLockKey))
		_ = conn.Close()
		return fmt.Errorf("database: begin locked reset tx: %w", err)
	}
	lt := &lockedTx{tx: tx, conn: conn}
	// Ensure the migration tracking table exists BEFORE deleting from it.
	// DROP SCHEMA ... CASCADE drops every object in the schema, including
	// bookdb_migrations, so a fresh reset must recreate the table before
	// the DELETE can find it (issue #68, cross-package race: one package's
	// DROP/CASCADE removes the table another package's DELETE targets).
	if _, err := lt.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version text PRIMARY KEY,
		name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`, DefaultTrackingTable)); err != nil {
		_ = lt.Rollback()
		return fmt.Errorf("database: ensure migration table: %w", err)
	}
	if _, err := lt.ExecContext(ctx, `DELETE FROM public.bookdb_migrations`); err != nil {
		_ = lt.Rollback()
		return fmt.Errorf("database: clear migration state: %w", err)
	}
	if _, err := lt.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quotedSchema)); err != nil {
		_ = lt.Rollback()
		return fmt.Errorf("database: drop test schema: %w", err)
	}
	if _, err := lt.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, quotedSchema)); err != nil {
		_ = lt.Rollback()
		return fmt.Errorf("database: create test schema: %w", err)
	}
	if err := lt.Commit(); err != nil {
		return fmt.Errorf("database: commit test reset: %w", err)
	}
	return nil
}

// PrepareCleanAndMigrate resets the requested application schema and
// (re)applies all migrations so neither the reset nor the migration DDL
// can interleave with another package's queries or resets (issue #68,
// hazards 1 and 2 above).
//
// This is the entry point test helpers should use when they need a clean,
// fully-migrated schema. The schema-mutation lock (TestResetLockKey) is
// held on a dedicated connection for the reset (reset + commit); the
// migration (RunUp) runs on the normal pool and acquires the MIGRATION
// lock (734211990) internally — a different key, so no nested acquisition
// of the same lock on a second connection (the WIP deadlock). All callers
// acquire locks in the same order (test -> migration), keeping the pair
// deadlock-free.
func PrepareCleanAndMigrate(ctx context.Context, db *sql.DB, schema string) (StatusReport, error) {
	if schema == "" {
		schema = DefaultApplicationSchema
	}
	quotedSchema, err := quoteIdent(schema)
	if err != nil {
		return StatusReport{}, err
	}
	var report StatusReport
	err = withTestLock(ctx, db, func(conn *sql.Conn) error {
		if err := runLockedReset(ctx, conn, quotedSchema); err != nil {
			return err
		}
		rep, err := RunUp(ctx, db)
		if err != nil {
			return err
		}
		report = rep
		return nil
	})
	return report, err
}
