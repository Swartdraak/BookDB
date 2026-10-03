// reconcile_pg_test.go — PostgreSQL-backed regression tests for the S3
// reconciliation pipeline (issue #55): enrichment with field-level
// provenance selection, Wikidata claims, and cross-source duplicate
// candidate persistence. Uses the shared disposable test database like the
// promotion package's tests; skips when no PostgreSQL is reachable.
package reconciliation

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/google/uuid"
)

// openTestDB connects to the development database the same way the other
// packages' integration tests do: BOOKDB_TEST_DATABASE_URL, or the documented
// local dev DSN. When no database is reachable the tests skip, so `make test`
// stays green in gate environments without a database.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:***@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := database.PrepareCleanAndMigrate(ctx, db, "bookdb"); err != nil {
		t.Fatalf("prepare clean test schema: %v", err)
	}
	return db
}

func truncateReconTables(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"duplicate_candidates",
		"field_provenance",
		"wikidata_claims",
		"identifiers",
		"canonical_revisions",
		"change_feed",
		"source_records",
		"people",
		"works",
	}
	for _, tbl := range tables {
		if _, err := db.ExecContext(ctx, `TRUNCATE bookdb.`+tbl+` RESTART IDENTITY CASCADE`); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}
}

// insertSourceRecord stores a promoted source record payload the way the
// promotion pipeline does (source_record.entity_id in the payload).
func insertSourceRecord(t *testing.T, db *sql.DB, sourceName, key, entityType, entityID, title string) {
	t.Helper()
	ctx := context.Background()
	payload, _ := json.Marshal(map[string]any{
		"source_record": map[string]any{"entity_id": entityID},
	})
	if title != "" {
		payload, _ = json.Marshal(map[string]any{
			"source_record": map[string]any{"entity_id": entityID},
			"title":         title,
		})
	} else if entityType == "person" {
		payload, _ = json.Marshal(map[string]any{
			"source_record": map[string]any{"entity_id": entityID},
			"name":          title,
		})
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO bookdb.source_records (source_name, source_key, content_hash, payload)
		VALUES ($1, $2, 'hash-' || $3, $4)`, sourceName, key, key, payload)
	if err != nil {
		t.Fatalf("insert source record: %v", err)
	}
}

func TestEnrichWorkRecordsProvenanceAndCanonicalFields(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	truncateReconTables(t, db)

	workID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, 'Original Title', 'original title')`, workID)
	if err != nil {
		t.Fatalf("insert work: %v", err)
	}
	insertSourceRecord(t, db, "openlibrary", "OL18315W", "work", workID.String(), "Original Title")

	srv := fixtureServer(t)
	client := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL})
	r := NewReconciler(db)

	n, err := r.EnrichWork(ctx, client, workID, "OL18315W")
	if err != nil {
		t.Fatalf("EnrichWork: %v", err)
	}
	if n < 3 {
		t.Fatalf("enriched fields = %d, want >= 3 (description, subjects, first_publish_date, cover_i)", n)
	}

	// Field provenance: every enriched field recorded, OL as evidence source.
	var provenanceCount int
	err = db.QueryRowContext(ctx, `
		SELECT count(*) FROM bookdb.field_provenance
		WHERE entity_type = 'work' AND entity_id = $1 AND source_name = 'openlibrary' AND source_key = 'OL18315W'`,
		workID).Scan(&provenanceCount)
	if err != nil {
		t.Fatalf("count provenance: %v", err)
	}
	if provenanceCount < 3 {
		t.Fatalf("provenance rows = %d, want >= 3", provenanceCount)
	}
	var selectedCount int
	err = db.QueryRowContext(ctx, `
		SELECT count(*) FROM bookdb.field_provenance
		WHERE entity_type = 'work' AND entity_id = $1 AND selected = true`, workID).Scan(&selectedCount)
	if err != nil {
		t.Fatalf("count selected: %v", err)
	}
	if selectedCount == 0 {
		t.Fatalf("no selected provenance rows")
	}

	// Idempotency: re-run must not duplicate provenance rows.
	if _, err := r.EnrichWork(ctx, client, workID, "OL18315W"); err != nil {
		t.Fatalf("re-EnrichWork: %v", err)
	}
	err = db.QueryRowContext(ctx, `
		SELECT count(*) FROM bookdb.field_provenance
		WHERE entity_type = 'work' AND entity_id = $1 AND source_name = 'openlibrary' AND source_key = 'OL18315W'`,
		workID).Scan(&provenanceCount)
	if err != nil {
		t.Fatalf("recount provenance: %v", err)
	}
	if provenanceCount < 3 {
		t.Fatalf("provenance rows after re-run = %d, want >= 3 (stable)", provenanceCount)
	}
}

func TestEnrichPersonWithWikidataStoresClaims(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	truncateReconTables(t, db)

	personID := uuid.New()
	_, err := db.ExecContext(ctx, `INSERT INTO bookdb.people (person_id, display_name) VALUES ($1, 'Tolkien')`, personID)
	if err != nil {
		t.Fatalf("insert person: %v", err)
	}

	srv := fixtureServer(t)
	client := NewEnrichmentClient(EnrichmentConfig{WDBase: srv.URL})
	r := NewReconciler(db)

	n, err := r.EnrichPersonWithWikidata(ctx, client, personID, "Q892")
	if err != nil {
		t.Fatalf("EnrichPersonWithWikidata: %v", err)
	}
	// 2 labels (en, fr) + birth + death = 4 claims.
	if n != 4 {
		t.Fatalf("stored claims = %d, want 4", n)
	}

	var claimCount int
	err = db.QueryRowContext(ctx, `
		SELECT count(*) FROM bookdb.wikidata_claims
		WHERE entity_type = 'person' AND entity_id = $1 AND wikidata_qid = 'Q892'`, personID).Scan(&claimCount)
	if err != nil {
		t.Fatalf("count claims: %v", err)
	}
	if claimCount != 4 {
		t.Fatalf("wikidata_claims rows = %d, want 4", claimCount)
	}

	// The acceptance example from issue #55: Tolkien -> Q892 is queryable.
	var labelValue string
	err = db.QueryRowContext(ctx, `
		SELECT value::text FROM bookdb.wikidata_claims
		WHERE entity_id = $1 AND property = 'label:en'`, personID).Scan(&labelValue)
	if err != nil {
		t.Fatalf("label:en: %v", err)
	}
	if labelValue != `"J. R. R. Tolkien"` {
		t.Fatalf("label:en = %q", labelValue)
	}

	// The QID is recorded as a 'candidate' identifier (never auto-resolved).
	var status string
	err = db.QueryRowContext(ctx, `
		SELECT status FROM bookdb.identifiers
		WHERE namespace = 'wikidata' AND normalized_value = 'Q892' AND target_id = $1`, personID).Scan(&status)
	if err != nil {
		t.Fatalf("wikidata identifier: %v", err)
	}
	if status != "candidate" {
		t.Fatalf("identifier status = %q, want candidate", status)
	}
}

func TestCrossSourceDuplicateCandidatesPersisted(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	truncateReconTables(t, db)

	workA := uuid.New()
	workB := uuid.New()
	personA := uuid.New()
	personB := uuid.New()

	// Work pair: same normalized title, different sources.
	_, _ = db.ExecContext(ctx, `INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, 'The Hobbit', 'the hobbit')`, workA)
	_, _ = db.ExecContext(ctx, `INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, 'The Hobbit', 'the hobbit')`, workB)
	insertSourceRecord(t, db, "openlibrary", "OL18315W", "work", workA.String(), "The Hobbit")
	insertSourceRecord(t, db, "wikidata", "Q892-W", "work", workB.String(), "the  HOBBIT")

	// Person pair: shared author identifier (OL key referenced by the WD record).
	_, _ = db.ExecContext(ctx, `INSERT INTO bookdb.people (person_id, display_name) VALUES ($1, 'J. R. R. Tolkien')`, personA)
	_, _ = db.ExecContext(ctx, `INSERT INTO bookdb.people (person_id, display_name) VALUES ($1, 'Tolkien')`, personB)
	// Person A's record: title empty, name set, author key OL17720A.
	insertPersonRecord(t, db, "openlibrary", "OL17720A", personA.String(), "J. R. R. Tolkien")
	// Person B's record: different source, references the same author key.
	insertPersonRecordWithAuthors(t, db, "wikidata", "WD-TOLKIEN", personB.String(), "Tolkien", "OL17720A")

	// A same-name author from a THIRD source: must NOT pair with person A on
	// name alone (names are never an auto-merge trigger; only shared
	// identifiers or normalized titles qualify, and people use names, not
	// titles, so title blocking does not apply to them).
	personC := uuid.New()
	_, _ = db.ExecContext(ctx, `INSERT INTO bookdb.people (person_id, display_name) VALUES ($1, 'Tolkien')`, personC)
	insertPersonRecord(t, db, "wikidata2", "WD-TOLKIEN2", personC.String(), "Tolkien")

	r := NewReconciler(db)
	candidates, err := r.FindDuplicateCandidatesCrossSource(ctx)
	if err != nil {
		t.Fatalf("FindDuplicateCandidatesCrossSource: %v", err)
	}

	// Expect exactly 2 candidates: the work title pair (0.5) and the person
	// shared-identifier pair (1.0). personA/personC (same display name, no
	// shared identifier) must NOT be a candidate.
	if len(candidates) != 2 {
		t.Fatalf("candidates = %d, want 2: %+v", len(candidates), candidates)
	}

	byRule := map[string]float64{}
	for _, c := range candidates {
		byRule[c.IDA+"/"+c.IDB] = c.Confidence
	}
	foundTitle, foundShared := false, false
	for _, c := range candidates {
		switch c.Confidence {
		case 0.5:
			foundTitle = true
			if c.TypeA != "work" {
				t.Errorf("title candidate type = %q, want work", c.TypeA)
			}
		case 1.0:
			foundShared = true
			if c.TypeA != "person" {
				t.Errorf("shared-identifier candidate type = %q, want person", c.TypeA)
			}
		default:
			t.Errorf("unexpected confidence %v", c.Confidence)
		}
		if c.Status != "pending" {
			t.Errorf("candidate status = %q, want pending", c.Status)
		}
	}
	if !foundTitle || !foundShared {
		t.Fatalf("foundTitle=%v foundShared=%v (byRule=%v)", foundTitle, foundShared, byRule)
	}

	// Persisted to bookdb.duplicate_candidates with evidence.
	var dbCount int
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.duplicate_candidates WHERE status = 'pending'`).Scan(&dbCount)
	if err != nil {
		t.Fatalf("count db candidates: %v", err)
	}
	if dbCount != 2 {
		t.Fatalf("duplicate_candidates rows = %d, want 2", dbCount)
	}
	var evidence string
	err = db.QueryRowContext(ctx, `
		SELECT evidence::text FROM bookdb.duplicate_candidates
		WHERE confidence = 1.0 LIMIT 1`).Scan(&evidence)
	if err != nil {
		t.Fatalf("evidence: %v", err)
	}
	if evidence == "" || evidence == "[]" {
		t.Fatalf("shared-identifier candidate has no evidence: %q", evidence)
	}

	// Idempotency: re-running does not duplicate rows.
	if _, err := r.FindDuplicateCandidatesCrossSource(ctx); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.duplicate_candidates`).Scan(&dbCount)
	if err != nil {
		t.Fatalf("recount: %v", err)
	}
	if dbCount != 2 {
		t.Fatalf("duplicate_candidates after re-run = %d, want 2", dbCount)
	}
}

func insertPersonRecord(t *testing.T, db *sql.DB, sourceName, key, entityID, name string) {
	t.Helper()
	insertPersonRecordWithAuthors(t, db, sourceName, key, entityID, name)
}

func insertPersonRecordWithAuthors(t *testing.T, db *sql.DB, sourceName, key, entityID, name string, authors ...string) {
	t.Helper()
	ctx := context.Background()
	doc := map[string]any{
		"source_record": map[string]any{"entity_id": entityID},
		"name":          name,
	}
	if len(authors) > 0 {
		doc["authors"] = authors
	}
	payload, _ := json.Marshal(doc)
	_, err := db.ExecContext(ctx, `
		INSERT INTO bookdb.source_records (source_name, source_key, content_hash, payload)
		VALUES ($1, $2, 'hash-' || $3, $4)`, sourceName, key, key, payload)
	if err != nil {
		t.Fatalf("insert person record: %v", err)
	}
}

// fixtureServer returns an httptest server serving the same OL/Wikidata
// fixtures as the unit tests (stubServer) for the PG-backed tests.
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	return stubServer(t)
}
