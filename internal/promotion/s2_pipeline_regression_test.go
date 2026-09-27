package promotion

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/ingestion"
)

// TestPromoteFreshIngestRecordNotNoOp is the regression test for the #21
// pipeline defect: ingestion stores the source record with a payload that is
// the deterministic Record JSON (it cannot store the raw dump line, whose
// bytes are not part of the normalized record) while the content hash is
// computed from the raw source line. A Promoter that treated "stored content
// hash matches" as an unconditional no-op therefore returned an empty entity
// id for EVERY record on the first ingest→promote pass, silently dropping
// the entire catalog. The gate must additionally require the source record
// to carry a promoted entity mapping; a fresh ingest (matching hash, no
// mapping) must be promoted, not skipped.
func TestPromoteFreshIngestRecordNotNoOp(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	// Parse a record exactly like the CLI does (RawJSON = raw source line).
	line := `{"key": "/works/OL910001W", "title": "Fresh Ingest Work"}`
	rec, err := ingestion.ParseOpenLibraryLine(line)
	if err != nil || rec == nil || rec.Title == "" {
		t.Fatalf("ParseOpenLibraryLine: err=%v rec=%v", err, rec)
	}

	// Simulate the ingest step: store the source record the way
	// ingestion.ProcessRecord does (Record-JSON payload, raw-line hash).
	ing := ingestion.NewIngestor(db)
	job, err := ing.StartJob(ctx, ingestion.Snapshot{ID: "s2-fresh-ingest", Hash: "synth", RecordCount: 1, RetrievedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	// Isolate this snapshot's manifest from earlier snapshots in the shared
	// DB: reusing a snapshot_id with a different record count would fail the
	// manifest's record_count consistency constraint.
	if _, err := db.ExecContext(ctx,
		`DELETE FROM bookdb.source_manifests WHERE source_name = $1 AND snapshot_id = $2`,
		ingestion.SourceName, "s2-fresh-ingest"); err != nil {
		t.Fatalf("clear manifest: %v", err)
	}
	outcome, err := ing.ProcessRecord(ctx, job.JobID, *rec)
	if err != nil {
		t.Fatalf("ProcessRecord: %v", err)
	}
	if outcome != "accepted" {
		t.Fatalf("ProcessRecord outcome=%q, want accepted", outcome)
	}
	if err := ing.CompleteJob(ctx, job.JobID); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	// The pre-#21-fix behavior: PromoteRecord is a no-op and returns no
	// entity id (the entire S2 catalog silently missing). Post-fix: the
	// record promotes to a real canonical work.
	out, err := p.PromoteRecord(ctx, rec)
	if err != nil {
		t.Fatalf("PromoteRecord after fresh ingest: %v", err)
	}
	if out.EntityID == "" {
		t.Fatalf("PromoteRecord after fresh ingest returned no entity id (promotion was a no-op on a never-promoted record; the #21 ingest→promote pipeline silently drops every record)")
	}
	if out.ChangeType != "created" {
		t.Errorf("change_type=%q, want created", out.ChangeType)
	}
	if out.EntityType != "work" {
		t.Errorf("entity_type=%q, want work", out.EntityType)
	}

	// The canonical work exists with the asserted title.
	var title string
	if err := db.QueryRowContext(ctx,
		`SELECT canonical_title FROM bookdb.works WHERE work_id = $1`, out.EntityID).Scan(&title); err != nil {
		t.Fatalf("work row: %v", err)
	}
	if title != "Fresh Ingest Work" {
		t.Errorf("work title=%q, want %q", title, "Fresh Ingest Work")
	}

	// The source record payload now carries the entity mapping.
	var payload []byte
	if err := db.QueryRowContext(ctx,
		`SELECT payload FROM bookdb.source_records WHERE source_name = $1 AND source_key = $2`,
		ingestion.SourceName, "OL910001W").Scan(&payload); err != nil {
		t.Fatalf("source record: %v", err)
	}
	if !containsSubstr(payload, out.EntityID) {
		t.Errorf("source record payload does not reference promoted entity %s", out.EntityID)
	}

	// A genuine replay (hash match AND mapping present) must now be the
	// no-op, returning the same entity id and no duplicate side effects.
	second, err := p.PromoteRecord(ctx, rec)
	if err != nil {
		t.Fatalf("replay PromoteRecord: %v", err)
	}
	if second.ChangeType != "" {
		t.Errorf("replay change_type=%q, want empty (no-op)", second.ChangeType)
	}
	if second.EntityID != out.EntityID {
		t.Errorf("replay entity=%q, want %q (stable identity)", second.EntityID, out.EntityID)
	}
	var changes int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_id = $1`, out.EntityID).Scan(&changes); err != nil {
		t.Fatalf("count change_feed: %v", err)
	}
	if changes != 1 {
		t.Errorf("change_feed entries=%d, want 1 (replay must not duplicate)", changes)
	}
}

func containsSubstr(haystack []byte, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(string(haystack), needle)
}
