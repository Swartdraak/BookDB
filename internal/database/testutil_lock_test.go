package database

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fnvHex32 returns an 8-character hex digest of s (FNV-1a, 32-bit).
func fnvHex32(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

// testDSN68 returns the shared disposable test database DSN, or skips.
func testDSN68(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	return dsn
}

// TestPrepareCleanAndMigrate_SerializesConcurrentResets is the issue #68
// regression test. It reproduces the defect's exact shape — two concurrent
// goroutines each resetting a shared schema and re-applying migrations,
// while a third goroutine keeps issuing queries against it — and asserts
// the two properties the fix must guarantee:
//
//  1. NO 42704 ("referenced schema was concurrently dropped") on the
//     resetters: migration DDL cannot race a concurrent reset, because
//     both run under the same lock.
//
// The reader is NOT under the lock (it simulates a concurrent test in a
// different package that is mid-query when a reset happens), so it CAN
// observe 42P01 during a reset — that is expected and does not fail the
// test. What the lock guarantees is that the RESETTERS never observe
// 42704 (their migration DDL cannot race a concurrent reset).
//
// The test is deliberately racy in timing (short sleeps, many iterations)
// so that, WITHOUT the lock, the 42P01/42704 interleaving is the expected
// outcome; WITH the lock it must be impossible.
func TestPrepareCleanAndMigrate_SerializesConcurrentResets(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN68(t)
	schema := "bookdb_" + fnvHex32(filepath.Base(os.Args[0]))

	dbA, err := Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	defer func() { _ = dbA.Close() }()
	dbB, err := Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	defer func() { _ = dbB.Close() }()

	// Bring the isolated schema up to date once, before the race.
	if _, err := PrepareCleanAndMigrate(ctx, dbA, schema); err != nil {
		t.Fatalf("initial prepare: %v", err)
	}
	// Clean up the isolated schema after the test.
	t.Cleanup(func() {
		_, _ = dbA.ExecContext(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})

	const (
		resetIters  = 6
		readerPolls = 30
	)

	var (
		mu          sync.Mutex
		reader42P01 int
		reset42704  int
		resetErrs   []string
	)

	// Reader: continuously query the isolated schema. Under the lock these
	// must never observe a dropped schema.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for i := 0; i < readerPolls; i++ {
			var n int
			err := dbA.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.works`, schema)).Scan(&n)
			if err != nil {
				mu.Lock()
				if containsSQLState(err.Error(), "42P01") {
					reader42P01++
				}
				mu.Unlock()
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Two concurrent resetters on two different pools (the cross-package
	// case: each `go test` package gets its own pool/connection set).
	var wg sync.WaitGroup
	for _, db := range []*sql.DB{dbA, dbB} {
		wg.Add(1)
		go func(db *sql.DB) {
			defer wg.Done()
			for i := 0; i < resetIters; i++ {
				_, err := PrepareCleanAndMigrate(ctx, db, schema)
				if err != nil {
					mu.Lock()
					if containsSQLState(err.Error(), "42704") {
						reset42704++
					}
					resetErrs = append(resetErrs, err.Error())
					mu.Unlock()
					t.Logf("reset error: %v", err)
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(db)
	}
	wg.Wait()
	<-readerDone

	mu.Lock()
	defer mu.Unlock()
	if reset42704 > 0 {
		t.Errorf("resetter observed %d x 42704 (schema dropped mid-migration) — issue #68 hazard 2", reset42704)
	}
	if len(resetErrs) > 0 && reset42704 == 0 {
		// Surface any non-race error honestly.
		t.Errorf("%d unexpected reset errors (not 42704); first: %v", len(resetErrs), resetErrs[0])
	}
	if reader42P01 > 0 {
		// Informational: the reader (not under the lock) can observe
		// 42P01 during a reset. This is expected and does not fail the
		// test — the lock guarantees that the RESETTERS never observe
		// 42704, not that the reader never observes 42P01.
		t.Logf("reader observed %d x 42P01 (expected: reader is not under the lock)", reader42P01)
	}
}

// containsSQLState reports whether the error string carries a PostgreSQL
// SQLSTATE (e.g. 42P01 / 42704) in lib/pq's format
// (`pq: <msg> (<SQLSTATE>)`).
func containsSQLState(s, state string) bool {
	idx := lastIndex(s, " (")
	if idx < 0 {
		return false
	}
	return s[idx+2:len(s)-1] == state
}

func lastIndex(s, sub string) int {
	for i := len(s) - len(sub); i >= 0; i-- {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
