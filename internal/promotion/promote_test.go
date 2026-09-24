package promotion

import (
	"context"
	"database/sql"
	"testing"

	"github.com/bookdb/bookdb/internal/ingestion"
)

func testRecord(t *testing.T, key, typ, title string) *Record {
	t.Helper()
	raw := `{"key":"/works/` + key + `"}`
	if typ == "edition" {
		raw = `{"key":"/editions/` + key + `"}`
	}
	if typ == "author" {
		raw = `{"key":"/authors/` + key + `"}`
	}
	return &Record{
		SourceKey:  key,
		SourceType: typ,
		Title:      title,
		RawJSON:    []byte(raw),
	}
}

func TestPromoteWorkCreatesCanonicalEntity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	rec := testRecord(t, "OL900001W", "work", "The Test Work")
	out, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if out.ChangeType != "created" {
		t.Errorf("expected created, got %q", out.ChangeType)
	}
	if out.EntityType != "work" || out.EntityID == "" {
		t.Errorf("bad entity: %v", out)
	}
	if out.OutboxEvent <= 0 {
		t.Errorf("expected canonical outbox event, got %d", out.OutboxEvent)
	}
	if out.Revision != 1 {
		t.Errorf("expected revision 1, got %d", out.Revision)
	}

	// Work row exists with the asserted title.
	var title, norm string
	err = db.QueryRowContext(ctx,
		`SELECT canonical_title, normalized_title FROM bookdb.works WHERE work_id = $1`, out.EntityID).Scan(&title, &norm)
	if err != nil {
		t.Fatalf("work row: %v", err)
	}
	if title != "The Test Work" || norm != "the test work" {
		t.Errorf("work title: %q / %q", title, norm)
	}

	// Revision recorded.
	var rev int64
	if err := db.QueryRowContext(ctx,
		`SELECT revision FROM bookdb.canonical_revisions WHERE entity_type = 'work' AND entity_id = $1`,
		out.EntityID).Scan(&rev); err != nil || rev != 1 {
		t.Errorf("revision: rev=%d err=%v", rev, err)
	}

	// Identifier recorded as candidate (auto-created entity).
	var status string
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM bookdb.identifiers WHERE namespace = 'openlibrary' AND normalized_value = 'OL900001W' AND target_type = 'work' AND target_id = $1`,
		out.EntityID).Scan(&status); err != nil {
		t.Fatalf("identifier: %v", err)
	}
	if status != "candidate" {
		t.Errorf("identifier status: %q", status)
	}

	// Provenance recorded for title.
	var prov int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.field_provenance WHERE entity_type = 'work' AND entity_id = $1 AND field_name = 'title'`,
		out.EntityID).Scan(&prov); err != nil || prov != 1 {
		t.Errorf("provenance: n=%d err=%v", prov, err)
	}

	// Change feed entry.
	var changes int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_type = 'work' AND entity_id = $1 AND change_type = 'created'`,
		out.EntityID).Scan(&changes); err != nil || changes != 1 {
		t.Errorf("change_feed: n=%d err=%v", changes, err)
	}

	// Canonical outbox event with the entity id.
	var eventCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.outbox WHERE aggregate_type = 'work' AND aggregate_id = $1 AND event_type = 'work.promoted' AND published_at IS NULL`,
		out.EntityID).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Errorf("canonical outbox: n=%d err=%v", eventCount, err)
	}
}

func TestPromoteWorkIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	rec := testRecord(t, "OL900002W", "work", "Idempotent Work")
	first, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote 1: %v", err)
	}
	second, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote 2: %v", err)
	}
	if second.ChangeType != "" {
		t.Errorf("expected no-op second promote, got %q", second.ChangeType)
	}
	if second.EntityID != first.EntityID {
		t.Errorf("entity id changed on replay: %q vs %q", second.EntityID, first.EntityID)
	}
	// No duplicate revision.
	var rev int64
	if err := db.QueryRowContext(ctx,
		`SELECT revision FROM bookdb.canonical_revisions WHERE entity_type = 'work' AND entity_id = $1`,
		first.EntityID).Scan(&rev); err != nil || rev != 1 {
		t.Errorf("revision after replay: %d (err %v)", rev, err)
	}
	var changes int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.change_feed WHERE entity_type = 'work' AND entity_id = $1`,
		first.EntityID).Scan(&changes); err != nil || changes != 1 {
		t.Errorf("change_feed after replay: %d (err %v)", changes, err)
	}
}

func TestPromoteAuthorCreatesPerson(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	rec := testRecord(t, "OL900003A", "author", "Ada Lovelace Byron")
	out, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if out.EntityType != "person" {
		t.Errorf("entity type: %q", out.EntityType)
	}
	var display, sortName sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT display_name, sort_name FROM bookdb.people WHERE person_id = $1`, out.EntityID).Scan(&display, &sortName); err != nil {
		t.Fatalf("person row: %v", err)
	}
	if display.String != "Ada Lovelace Byron" || sortName.String != "Byron, Ada Lovelace" {
		t.Errorf("person: %q / %q", display.String, sortName.String)
	}
}

func TestPromoteEditionCreatesWorkExpressionEdition(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	raw := `{"key":"/editions/OL900004M","works":[{"key":"/works/OL900004W"}],"isbn_13":["9780261102104"],"publish_date":"1965"}`
	rec := &Record{
		SourceKey:   "OL900004M",
		SourceType:  "edition",
		Title:       "The Edition Work",
		Language:    "eng",
		Format:      "print",
		ISBNs:       []string{"9780261102104"},
		Publisher:   "Ace Books",
		PublishDate: "1965",
		RawJSON:     []byte(raw),
	}
	out, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote edition: %v", err)
	}
	if out.EntityType != "edition" {
		t.Errorf("entity type: %q", out.EntityType)
	}
	// Edition row with asserted fields.
	var format, publisher string
	var isbn sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT format, publisher_name, isbn13 FROM bookdb.editions WHERE edition_id = $1`,
		out.EntityID).Scan(&format, &publisher, &isbn); err != nil {
		t.Fatalf("edition row: %v", err)
	}
	if format != "print" || publisher != "Ace Books" || isbn.String != "9780261102104" {
		t.Errorf("edition fields: %q %q %q", format, publisher, isbn.String)
	}
	// Expression exists under some work, and edition_contents links it.
	var exprCount, contentsCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.expressions e JOIN bookdb.edition_contents c ON c.expression_id = e.expression_id WHERE c.edition_id = $1`,
		out.EntityID).Scan(&exprCount); err != nil || exprCount != 1 {
		t.Errorf("expression via contents: n=%d err=%v", exprCount, err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.edition_contents WHERE edition_id = $1 AND position = 0`,
		out.EntityID).Scan(&contentsCount); err != nil || contentsCount != 1 {
		t.Errorf("edition_contents: n=%d err=%v", contentsCount, err)
	}
}

func TestPromoteEditionBindsToResolvedWork(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	// Promote the work first, then mark its identifier resolved (as a
	// cross-source resolution would).
	workRec := testRecord(t, "OL900010W", "work", "Bound Work")
	workOut, err := p.Promote(ctx, workRec)
	if err != nil {
		t.Fatalf("Promote work: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE bookdb.identifiers SET status = 'resolved' WHERE namespace = 'openlibrary' AND normalized_value = 'OL900010W' AND target_type = 'work'`); err != nil {
		t.Fatalf("resolve work identifier: %v", err)
	}

	raw := `{"key":"/editions/OL900010M","works":[{"key":"/works/OL900010W"}]}`
	rec := &Record{
		SourceKey:  "OL900010M",
		SourceType: "edition",
		Title:      "Bound Work",
		RawJSON:    []byte(raw),
	}
	out, err := p.Promote(ctx, rec)
	if err != nil {
		t.Fatalf("Promote edition: %v", err)
	}
	// The edition's expression must hang off the already-resolved work.
	var workID string
	err = db.QueryRowContext(ctx, `
		SELECT e.work_id FROM bookdb.editions ed
		JOIN bookdb.expressions e ON e.expression_id = ed.expression_id
		WHERE ed.edition_id = $1`, out.EntityID).Scan(&workID)
	if err != nil {
		t.Fatalf("edition->work: %v", err)
	}
	if workID != workOut.EntityID {
		t.Errorf("edition bound to work %q, want %q", workID, workOut.EntityID)
	}
	// No extra provisional work was created.
	var works int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.works`).Scan(&works); err != nil || works != 1 {
		t.Errorf("works count: %d (err %v)", works, err)
	}
}

func TestPromoteRecordRebuildsRawJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	p := NewPromoter(db)
	cleanEntityTables(t, db)

	rec := &ingestion.Record{
		SourceKey:  "OL900020W",
		SourceType: "work",
		Title:      "Rebuilt Raw Work",
		Authors:    []string{"OL1A"},
		Language:   "eng",
	}
	out, err := p.PromoteRecord(ctx, rec)
	if err != nil {
		t.Fatalf("PromoteRecord: %v", err)
	}
	if out.ChangeType != "created" || out.EntityType != "work" {
		t.Errorf("outcome: %+v", out)
	}
}

func TestPromoteRejectsUnsupportedType(t *testing.T) {
	p := NewPromoter(openTestDB(t))
	if _, err := p.Promote(context.Background(), testRecord(t, "OL1", "subject", "X")); err == nil {
		t.Fatal("expected error for unsupported source type")
	}
}

func TestPromoteRejectsMissingKey(t *testing.T) {
	p := NewPromoter(openTestDB(t))
	if _, err := p.Promote(context.Background(), &Record{SourceType: "work", Title: "X"}); err == nil {
		t.Fatal("expected error for missing source key")
	}
}

func TestSortNameFor(t *testing.T) {
	if got := sortNameFor("Ada Lovelace Byron"); got != "Byron, Ada Lovelace" {
		t.Errorf("two-part: %q", got)
	}
	if got := sortNameFor("Jane"); got != "" {
		t.Errorf("single-part: %q", got)
	}
}
