package ingestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bookdb/bookdb/internal/database"
)

// openReplayTestDB connects to the test database the same way the other
// DB-backed suites do (BOOKDB_TEST_DATABASE_URL, or the documented local dev
// DSN), applies the real migrations, and resets the bookdb schema so each
// test case starts from a clean, deterministic state (per-package fresh-DB
// convention, docs/bookdb/08-testing.md).
func openReplayTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := database.RunUp(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	// Reset the bookdb schema (fresh-DB semantics per test case).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin reset: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `DROP SCHEMA IF EXISTS bookdb CASCADE`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("drop test schema: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA bookdb`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM public.bookdb_migrations`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("clear migration state: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit reset: %v", err)
	}
	if _, err := database.RunUp(ctx, db); err != nil {
		t.Fatalf("re-apply migrations on clean schema: %v", err)
	}
	return db
}

// synthRecord builds a deterministic synthetic Open Library record.
// It is clearly synthetic test data (issue #62 reproduction fixture).
func synthRecord(key, sourceType, title string) Record {
	raw, _ := json.Marshal(map[string]string{
		"key":   "/" + map[string]string{"work": "works", "edition": "editions", "author": "authors"}[sourceType] + "/" + key,
		"title": title,
	})
	return Record{
		SourceKey:  key,
		SourceType: sourceType,
		Title:      title,
		RawJSON:    raw,
	}
}

// checkpointPayload is a minimal resume checkpoint mirroring the CLI's
// position tracking (offset into the snapshot).
func checkpointPayload(offset int64) []byte {
	b, _ := json.Marshal(map[string]int64{"offset": offset})
	return b
}

// runIngestPass replays a slice of the snapshot to the Ingestor and advances
// the checkpoint, emulating the CLI ingest loop over a fresh job.
func runIngestPass(ctx context.Context, t *testing.T, ing *Ingestor, jobID uuid.UUID, recs []Record) {
	t.Helper()
	for i := range recs {
		outcome, err := ing.ProcessRecord(ctx, jobID, recs[i])
		if err != nil {
			t.Fatalf("pass %d ProcessRecord(%s): %v", i, recs[i].SourceKey, err)
		}
		_ = outcome
	}
}

// TestProcessRecordReplayAccounting is the table-driven regression for
// issue #62: after a first pass and an interrupted-and-resumed replay, the
// accounting invariant accepted+unchanged+rejected+quarantined == processed
// must hold and VerifyAccounting must return nil.
//
// Reference: assess-logs-20260925f/repro-accounting-probe.log (the probe
// failed at main@94685171 with "0+ 4+ 0+ 0 = 4, expected 2").
func TestProcessRecordReplayAccounting(t *testing.T) {
	type replayCase struct {
		name     string
		resumeAt int               // number of records from the start of the snapshot that were already processed by the interrupted pass
		changed  map[string]string // source_key -> modified title to apply during replay (content change)
		want     map[string]int64  // field name -> expected final counter value
	}
	cases := []replayCase{
		{
			// First pass accepts all 3; the resumed replay sees all 3 already
			// stored, unchanged and already counted -> no re-increment. Final
			// counters equal the original completed job's counters.
			name:     "probe-shape: replay of an unchanged snapshot",
			resumeAt: 3,
			changed:  nil,
			want:     map[string]int64{"processed": 3, "accepted": 3},
		},
		{
			// First pass accepts all 3; the resumed replay updates one record
			// (content change) and replays two unchanged ones. The update must
			// NOT add a second 'accepted' increment and the unchanged replays
			// must NOT add 'unchanged' increments.
			name:     "mixed: replay includes a content-changed record",
			resumeAt: 3,
			changed:  map[string]string{"OL62R2W": "Mixed Title Changed"},
			want:     map[string]int64{"processed": 3, "accepted": 3},
		},
		{
			// First pass interrupted after 1 of 3 records (1 accepted,
			// checkpointed). The resumed replay processes the remaining 2 and
			// replays the first unchanged: accepted=3, unchanged=0 (the
			// replayed first record was already counted as accepted).
			name:     "partial: first pass interrupted after one of three records",
			resumeAt: 1,
			changed:  map[string]string{"OL62P3W": "Partial Title Changed"},
			want:     map[string]int64{"processed": 3, "accepted": 3},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := openReplayTestDB(t)
			ing := NewIngestor(db)

			// Deterministic synthetic snapshot: 3 works (clearly test data).
			snapshot := []Record{
				synthRecord("OL62R1W", "work", "Replay Work One"),
				synthRecord("OL62R2W", "work", "Replay Work Two"),
				synthRecord("OL62P3W", "work", "Partial Work Three"),
			}
			if tc.resumeAt > len(snapshot) {
				t.Fatalf("test case: resumeAt %d must be <= %d", tc.resumeAt, len(snapshot))
			}

			snap := Snapshot{
				ID:          "synth-62-" + tc.name[:1],
				Hash:        "synth",
				RecordCount: len(snapshot),
				RetrievedAt: time.Now().UTC(),
			}
			job, err := ing.StartJob(ctx, snap)
			if err != nil {
				t.Fatalf("StartJob: %v", err)
			}

			// Pass 1 (interrupted): process the records before the checkpoint.
			runIngestPass(ctx, t, ing, job.JobID, snapshot[:tc.resumeAt])
			if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(tc.resumeAt), checkpointPayload(int64(tc.resumeAt))); err != nil {
				t.Fatalf("checkpoint after first pass: %v", err)
			}

			// Resume (replay): reprocess the full snapshot from the beginning,
			// applying the content change when the case says so.
			replay := make([]Record, len(snapshot))
			copy(replay, snapshot)
			for i := range replay {
				if newTitle, ok := tc.changed[replay[i].SourceKey]; ok {
					replay[i] = synthRecord(replay[i].SourceKey, replay[i].SourceType, newTitle)
				}
			}
			runIngestPass(ctx, t, ing, job.JobID, replay)
			if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(len(snapshot)), checkpointPayload(int64(len(snapshot)))); err != nil {
				t.Fatalf("checkpoint after replay: %v", err)
			}

			// The accounting invariant must hold for a resumed job.
			if err := ing.VerifyAccounting(ctx, job.JobID); err != nil {
				state, _ := ing.GetJob(ctx, job.JobID)
				t.Fatalf("VerifyAccounting on resumed job: %v (job: %+v)", err, state)
			}

			state, err := ing.GetJob(ctx, job.JobID)
			if err != nil {
				t.Fatalf("GetJob: %v", err)
			}
			got := map[string]int64{
				"processed":   state.Processed,
				"accepted":    state.Accepted,
				"unchanged":   state.Unchanged,
				"rejected":    state.Rejected,
				"quarantined": state.Quarantined,
			}
			for field, want := range tc.want {
				if got[field] != want {
					t.Errorf("job %s counter %s = %d, want %d (all: %s)",
						job.JobID, field, got[field], want, formatCounters(got))
				}
			}
		})
	}
}

// TestProcessRecordReplayDoesNotDuplicateSourceRecords asserts that replaying
// previously-stored records does not create duplicate source_records rows and
// that a content change is reflected exactly once.
func TestProcessRecordReplayDoesNotDuplicateSourceRecords(t *testing.T) {
	ctx := context.Background()
	db := openReplayTestDB(t)
	ing := NewIngestor(db)

	snapshot := []Record{
		synthRecord("OL62D1W", "work", "Dedup Work One"),
		synthRecord("OL62D2W", "work", "Dedup Work Two"),
	}
	snap := Snapshot{ID: "synth-62-dedup", Hash: "synth", RecordCount: len(snapshot), RetrievedAt: time.Now().UTC()}
	job, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	// First pass over both records, then interrupt.
	runIngestPass(ctx, t, ing, job.JobID, snapshot)
	if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(len(snapshot)), checkpointPayload(int64(len(snapshot)))); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	// Replay: one record unchanged, one changed.
	replay := make([]Record, len(snapshot))
	copy(replay, snapshot)
	replay[1] = synthRecord("OL62D2W", "work", "Dedup Work Two v2")
	runIngestPass(ctx, t, ing, job.JobID, replay)
	if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(len(snapshot)), checkpointPayload(int64(len(snapshot)))); err != nil {
		t.Fatalf("replay checkpoint: %v", err)
	}

	var rows int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM bookdb.source_records
		WHERE source_name = $1 AND source_key IN ('OL62D1W', 'OL62D2W')`, SourceName).Scan(&rows); err != nil {
		t.Fatalf("count source_records: %v", err)
	}
	if rows != 2 {
		t.Fatalf("source_records rows = %d, want 2 (replay must not duplicate)", rows)
	}

	var hash string
	if err := db.QueryRowContext(ctx, `
		SELECT content_hash FROM bookdb.source_records
		WHERE source_name = $1 AND source_key = 'OL62D2W'`, SourceName).Scan(&hash); err != nil {
		t.Fatalf("read content_hash: %v", err)
	}
	if want := hashRecord(replay[1]); hash != want {
		t.Fatalf("changed record content_hash = %s, want %s (change must be reflected once)", hash, want)
	}

	if err := ing.VerifyAccounting(ctx, job.JobID); err != nil {
		state, _ := ing.GetJob(ctx, job.JobID)
		t.Fatalf("VerifyAccounting after dedup replay: %v (job: %+v)", err, state)
	}
}

func formatCounters(m map[string]int64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s=%d", k, m[k])
	}
	return out
}
