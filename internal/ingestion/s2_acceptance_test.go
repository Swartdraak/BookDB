package ingestion

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// S2-IMPORT acceptance regression tests — ingestion-only (issue #21)
//
// These tests cover the S2-IMPORT acceptance criteria that can be verified
// with the ingestion package alone (no promotion — to avoid the
// ingestion→promotion→ingestion import cycle). The full pipeline tests
// (ingest + promote + catalog) live in internal/runtime/s2_acceptance_test.go.
//
// Criteria covered here:
//   1. Malformed parser fixtures: deliberately malformed records are handled
//      gracefully without panicking (acceptance criterion 1).
//   2. Unicode title parsing: Unicode titles round-trip through the parser
//      correctly (acceptance criterion 6, Unicode input).
//   4a. Upstream offline resilience: source_records are durable and
//      retrievable after ingestion (acceptance criterion 4a, DB-level).
//   6. Scheduled occurrence key: the (source_name, snapshot_id) uniqueness
//      constraint prevents duplicate manifest rows (acceptance criterion 6).
// ---------------------------------------------------------------------------

// TestS2MalformedParserFixtures verifies that deliberately malformed records
// are handled gracefully — rejected or skipped without panicking, and the
// accounting invariant holds.
func TestS2MalformedParserFixtures(t *testing.T) {
	fixtures := []struct {
		name string
		line string
	}{
		{"empty line", ""},
		{"whitespace only", "   \t  "},
		{"not JSON", "this is not json at all"},
		{"truncated JSON", `{"key": "/works/OL21MALW", "title": "trunc`},
		{"empty JSON object", "{}"},
		{"JSON array (not object)", `[1, 2, 3]`},
		{"JSON with wrong key type", `{"key": 12345, "title": "wrong"}`},
		{"unknown key prefix", `{"key": "/mystery/OL21X", "title": "Mystery"}`},
		{"no key at all", `{"title": "No Key"}`},
		{"work with no title", `{"key": "/works/OL21NOTITLE", "title": ""}`},
		{"valid TSV dump line", `/type/work\t/works/OL21TSVW\t1\t2024-01-01T00:00:00Z\t{"key": "/works/OL21TSVW", "title": "Valid TSV"}`},
		{"corrupt TSV (missing columns)", `/type/work\t/works/OL21BADTSV\t1`},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("ParseOpenLibraryLine panicked on %q: %v", fx.line, r)
					}
				}()
				_, _ = ParseOpenLibraryLine(fx.line)
			}()
		})
	}

	// Full pipeline: malformed records are rejected, accounting holds.
	ctx := context.Background()
	db := openReplayTestDB(t)
	ing := NewIngestor(db)

	snap := Snapshot{ID: "s2-malformed", Hash: "synth", RecordCount: 5, RetrievedAt: time.Now().UTC()}
	job, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	mixed := []Record{
		synthRecord("OL21MIX1W", "work", "Valid Work One"),
		{SourceKey: "", SourceType: "work", Title: "No Key Work"},
		synthRecord("OL21MIX2W", "work", "Valid Work Two"),
		{SourceKey: "OL21MIX3W", SourceType: "work", Title: ""},
		synthRecord("OL21MIX4W", "work", "Valid Work Three"),
	}

	for i := range mixed {
		_, err := ing.ProcessRecord(ctx, job.JobID, mixed[i])
		if err != nil {
			t.Fatalf("ProcessRecord[%d]: %v", i, err)
		}
	}

	if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(len(mixed)), checkpointPayload(int64(len(mixed)))); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if err := ing.CompleteJob(ctx, job.JobID); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	state, err := ing.GetJob(ctx, job.JobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if state.Accepted != 3 {
		t.Errorf("accepted=%d, want 3", state.Accepted)
	}
	if state.Rejected != 2 {
		t.Errorf("rejected=%d, want 2", state.Rejected)
	}
	if state.Processed != 5 {
		t.Errorf("processed=%d, want 5", state.Processed)
	}
	if err := ing.VerifyAccounting(ctx, job.JobID); err != nil {
		t.Errorf("VerifyAccounting: %v (job: %+v)", err, state)
	}
}

// TestS2UnicodeTitles verifies that Unicode work titles are correctly parsed
// by ParseOpenLibraryLine and round-trip through the full ingestion pipeline
// into source_records with their Unicode content intact.
func TestS2UnicodeTitles(t *testing.T) {
	ctx := context.Background()
	db := openReplayTestDB(t)
	ing := NewIngestor(db)

	// Build JSONL lines with Unicode titles (as the CLI would read them).
	unicodeCases := []struct {
		key   string
		title string
	}{
		{"OL21UNI01W", "The Æthelred Chronicle — A Tale of ᚠᚢᚦᚨᚱᚲ"},
		{"OL21UNI02W", "Café au Lait: A Guide to Çà and Ño"},
		{"OL21UNI03W", "The Ωmega Protocol — Ünïcødé Tëst"},
		{"OL21UNI04W", "한국어 작품: 한국어 제목"},
		{"OL21UNI05W", "日本語の作品：日本語タイトル"},
	}

	recs := make([]Record, 0, len(unicodeCases))
	for _, uc := range unicodeCases {
		// Build the raw JSON line exactly as an OL dump would have it.
		raw := map[string]string{
			"key":   "/works/" + uc.key,
			"title": uc.title,
		}
		lineBytes, _ := json.Marshal(raw)
		line := string(lineBytes)
		parsed, err := ParseOpenLibraryLine(line)
		if err != nil {
			t.Fatalf("ParseOpenLibraryLine(%q): %v", line, err)
		}
		if parsed == nil {
			t.Fatalf("ParseOpenLibraryLine returned nil for %q", line)
		}
		if parsed.Title != uc.title {
			t.Fatalf("parsed title=%q, want %q (Unicode parse failed)", parsed.Title, uc.title)
		}
		if parsed.SourceKey != uc.key {
			t.Fatalf("parsed key=%q, want %q", parsed.SourceKey, uc.key)
		}
		if parsed.SourceType != "work" {
			t.Fatalf("parsed type=%q, want work", parsed.SourceType)
		}
		recs = append(recs, *parsed)
	}

	snap := Snapshot{ID: "s2-unicode", Hash: "synth", RecordCount: len(recs), RetrievedAt: time.Now().UTC()}
	job, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}

	for i := range recs {
		outcome, err := ing.ProcessRecord(ctx, job.JobID, recs[i])
		if err != nil {
			t.Fatalf("ProcessRecord[%d]: %v", i, err)
		}
		if outcome != "accepted" {
			t.Fatalf("ProcessRecord[%d] outcome=%q, want accepted", i, outcome)
		}
	}

	if err := ing.UpdateCheckpoint(ctx, job.JobID, int64(len(recs)), checkpointPayload(int64(len(recs)))); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if err := ing.CompleteJob(ctx, job.JobID); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	// Verify each Unicode title is stored correctly in source_records.
	for i, rec := range recs {
		var payload []byte
		err := db.QueryRowContext(ctx,
			`SELECT payload FROM bookdb.source_records WHERE source_name = $1 AND source_key = $2`,
			SourceName, rec.SourceKey).
			Scan(&payload)
		if err != nil {
			t.Fatalf("source_record for %s: %v", rec.SourceKey, err)
		}
		var stored map[string]any
		if err := json.Unmarshal(payload, &stored); err != nil {
			t.Fatalf("unmarshal payload for %s: %v", rec.SourceKey, err)
		}
		if got, ok := stored["title"].(string); !ok || got != rec.Title {
			t.Errorf("stored title[%d] for %s = %q, want %q (Unicode round-trip failed)",
				i, rec.SourceKey, got, rec.Title)
		}
	}

	// Verify accounting.
	state, _ := ing.GetJob(ctx, job.JobID)
	if state.Accepted != int64(len(recs)) {
		t.Errorf("accepted=%d, want %d (all Unicode titles should be accepted)", state.Accepted, len(recs))
	}
	if err := ing.VerifyAccounting(ctx, job.JobID); err != nil {
		t.Errorf("VerifyAccounting: %v", err)
	}
}

// TestS2UpstreamOfflineResilience verifies that after ingestion, the
// source_records are durable and retrievable — the catalog does not depend
// on the source file being present. (The full catalog-lookup test with
// promotion lives in internal/runtime/s2_acceptance_test.go.)
func TestS2UpstreamOfflineResilience(t *testing.T) {
	ctx := context.Background()
	db := openReplayTestDB(t)
	ing := NewIngestor(db)

	// Ingest 3 records.
	recs := []Record{
		synthRecord("OL21OFS1W", "work", "The Offline Catalog"),
		synthRecord("OL21OFS2W", "work", "Self-Contained Lookup"),
		synthRecord("OL21OFS3A", "author", "Author Offline"),
	}
	snap := Snapshot{ID: "s2-offline", Hash: "synth", RecordCount: len(recs), RetrievedAt: time.Now().UTC()}
	job, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	for i := range recs {
		if _, err := ing.ProcessRecord(ctx, job.JobID, recs[i]); err != nil {
			t.Fatalf("ProcessRecord[%d]: %v", i, err)
		}
	}
	if err := ing.CompleteJob(ctx, job.JobID); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	// Verify source_records are present.
	var srCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.source_records WHERE source_name = $1`, SourceName).
		Scan(&srCount); err != nil {
		t.Fatalf("count source_records: %v", err)
	}
	if srCount != len(recs) {
		t.Fatalf("source_records count=%d, want %d", srCount, len(recs))
	}

	// Verify each source record is retrievable with its content_hash.
	for _, rec := range recs {
		var sourceName, sourceKey, contentHash string
		var payload []byte
		err := db.QueryRowContext(ctx,
			`SELECT source_name, source_key, content_hash, payload
			 FROM bookdb.source_records WHERE source_name = $1 AND source_key = $2`,
			SourceName, rec.SourceKey).
			Scan(&sourceName, &sourceKey, &contentHash, &payload)
		if err != nil {
			t.Fatalf("source_record for %s not retrievable: %v", rec.SourceKey, err)
		}
		wantHash := hashRecord(rec)
		if contentHash != wantHash {
			t.Errorf("content_hash for %s = %s, want %s (durable evidence corrupted)",
				rec.SourceKey, contentHash, wantHash)
		}
		_ = payload
	}

	// The source_manifest is also durable.
	var manifestCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.source_manifests WHERE source_name = $1 AND snapshot_id = $2`,
		SourceName, snap.ID).
		Scan(&manifestCount); err != nil {
		t.Fatalf("count source_manifests: %v", err)
	}
	if manifestCount != 1 {
		t.Errorf("source_manifests count=%d, want 1", manifestCount)
	}
}

// TestS2DuplicateJobOccurrenceKey verifies that the (source_name, snapshot_id)
// uniqueness constraint on source_manifests prevents duplicate manifest rows
// when two jobs are started for the same snapshot (the "scheduled occurrence
// key" from issue #21 acceptance criterion 6).
func TestS2DuplicateJobOccurrenceKey(t *testing.T) {
	ctx := context.Background()
	db := openReplayTestDB(t)
	ing := NewIngestor(db)

	snap := Snapshot{ID: "s2-dedup-key", Hash: "synth", RecordCount: 3, RetrievedAt: time.Now().UTC()}

	job1, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob 1: %v", err)
	}
	job2, err := ing.StartJob(ctx, snap)
	if err != nil {
		t.Fatalf("StartJob 2: %v", err)
	}

	if job1.JobID == job2.JobID {
		t.Fatal("two jobs for the same snapshot must have distinct job_ids")
	}

	// The manifest must be deduplicated (UNIQUE constraint).
	var manifestCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.source_manifests WHERE source_name = $1 AND snapshot_id = $2`,
		SourceName, snap.ID).
		Scan(&manifestCount); err != nil {
		t.Fatalf("count source_manifests: %v", err)
	}
	if manifestCount != 1 {
		t.Errorf("source_manifests count=%d, want 1 (duplicate occurrence key must dedupe)", manifestCount)
	}

	// Both job rows exist.
	var jobCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.ingestion_jobs WHERE source_name = $1 AND snapshot_id = $2`,
		SourceName, snap.ID).
		Scan(&jobCount); err != nil {
		t.Fatalf("count ingestion_jobs: %v", err)
	}
	if jobCount != 2 {
		t.Errorf("ingestion_jobs count=%d, want 2 (each run is tracked)", jobCount)
	}
}
