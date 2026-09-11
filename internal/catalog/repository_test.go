package catalog

import (
	"context"
	"errors"
	"testing"
)

func TestGetWork_ReturnsFixture(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	work, err := repo.GetWork(context.Background(), WorkIDDune)
	if err != nil {
		t.Fatalf("GetWork: %v", err)
	}
	if work.WorkID != WorkIDDune {
		t.Fatalf("expected work %s, got %s", WorkIDDune, work.WorkID)
	}
	if work.CanonicalTitle != "Dune" {
		t.Fatalf("expected title Dune, got %q", work.CanonicalTitle)
	}
	if work.LanguageCode == nil || *work.LanguageCode != "en" {
		t.Fatalf("expected language en, got %v", work.LanguageCode)
	}
}

func TestGetWork_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	_, err := repo.GetWork(context.Background(), "00000000-0000-4000-8000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetEdition_ReturnsFixture(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	edition, err := repo.GetEdition(context.Background(), EdIDDunePrint)
	if err != nil {
		t.Fatalf("GetEdition: %v", err)
	}
	if edition.EditionID != EdIDDunePrint {
		t.Fatalf("expected edition %s, got %s", EdIDDunePrint, edition.EditionID)
	}
	if edition.ISBN13 == nil || *edition.ISBN13 != "9780441172719" {
		t.Fatalf("expected ISBN 9780441172719, got %v", edition.ISBN13)
	}
	if edition.Format == nil || *edition.Format != "print" {
		t.Fatalf("expected format print, got %v", edition.Format)
	}
}

func TestGetEdition_MissingISBN(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	// Audio editions have no ISBN.
	edition, err := repo.GetEdition(context.Background(), EdIDDuneAudioA)
	if err != nil {
		t.Fatalf("GetEdition: %v", err)
	}
	if edition.ISBN13 != nil {
		t.Fatalf("expected nil ISBN for audio edition, got %v", *edition.ISBN13)
	}
}

func TestListEditionsForWork(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	editions, err := repo.ListEditionsForWork(context.Background(), WorkIDDune)
	if err != nil {
		t.Fatalf("ListEditionsForWork: %v", err)
	}
	// Dune has print, ebook, two audio, and the Korean print edition.
	if len(editions) != 5 {
		t.Fatalf("expected 5 editions for Dune, got %d", len(editions))
	}
}

func TestListWorksForPerson(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	works, err := repo.ListWorksForPerson(context.Background(), PersonIDFrank)
	if err != nil {
		t.Fatalf("ListWorksForPerson: %v", err)
	}
	if len(works) != 1 {
		t.Fatalf("expected 1 work for Frank Herbert, got %d", len(works))
	}
	if works[0].WorkID != WorkIDDune {
		t.Fatalf("expected Dune, got %s", works[0].WorkID)
	}
}

func TestTwoSameNamePeopleAreDistinct(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	janeA, err := repo.GetPerson(context.Background(), PersonIDJane)
	if err != nil {
		t.Fatalf("GetPerson Jane A: %v", err)
	}
	janeB, err := repo.GetPerson(context.Background(), PersonIDJaneB)
	if err != nil {
		t.Fatalf("GetPerson Jane B: %v", err)
	}
	if janeA.DisplayName != janeB.DisplayName {
		t.Fatalf("expected same display name, got %q and %q", janeA.DisplayName, janeB.DisplayName)
	}
	if janeA.PersonID == janeB.PersonID {
		t.Fatal("expected distinct person IDs for same-name people")
	}

	// Each Jane authored a different work.
	worksA, err := repo.ListWorksForPerson(context.Background(), PersonIDJane)
	if err != nil {
		t.Fatalf("ListWorksForPerson Jane A: %v", err)
	}
	worksB, err := repo.ListWorksForPerson(context.Background(), PersonIDJaneB)
	if err != nil {
		t.Fatalf("ListWorksForPerson Jane B: %v", err)
	}
	if len(worksA) != 1 || len(worksB) != 1 {
		t.Fatalf("expected 1 work each, got %d and %d", len(worksA), len(worksB))
	}
	if worksA[0].WorkID == worksB[0].WorkID {
		t.Fatal("expected distinct works for same-name people")
	}
}

func TestTwoSameLanguageNarrationsAreDistinct(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	// Both narrations are English expressions of the same work but distinct.
	exprA, err := repo.GetExpression(context.Background(), ExprIDDuneAudioA)
	if err != nil {
		t.Fatalf("GetExpression A: %v", err)
	}
	exprB, err := repo.GetExpression(context.Background(), ExprIDDuneAudioB)
	if err != nil {
		t.Fatalf("GetExpression B: %v", err)
	}
	if exprA.WorkID != exprB.WorkID {
		t.Fatalf("expected same work, got %s and %s", exprA.WorkID, exprB.WorkID)
	}
	if exprA.LanguageCode != exprB.LanguageCode {
		t.Fatalf("expected same language, got %s and %s", exprA.LanguageCode, exprB.LanguageCode)
	}
	if exprA.ExpressionID == exprB.ExpressionID {
		t.Fatal("expected distinct expression IDs for same-language narrations")
	}
}

// TestTranslationIsExpressionOfSameWork asserts the S1 correction: the Korean
// translation of Dune is an Expression of the single Dune Work, not a second
// canonical Work. It also confirms the translator credit and that the two
// same-title works remain distinct.
func TestTranslationIsExpressionOfSameWork(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)
	ctx := context.Background()

	// The Korean expression belongs to the Dune Work.
	exprKO, err := repo.GetExpression(ctx, ExprIDDuneKO)
	if err != nil {
		t.Fatalf("GetExpression KO: %v", err)
	}
	if exprKO.WorkID != WorkIDDune {
		t.Fatalf("expected Korean expression to belong to Dune work %s, got %s", WorkIDDune, exprKO.WorkID)
	}
	if exprKO.LanguageCode != "ko" {
		t.Fatalf("expected Korean expression language ko, got %q", exprKO.LanguageCode)
	}

	// The retired second Work must not exist.
	if _, err := repo.GetWork(ctx, "11111111-1111-4111-8111-111111111112"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected retired Korean work to be absent, got %v", err)
	}

	// The Dune Work now has 5 editions (print, ebook, 2 audio, Korean print).
	editions, err := repo.ListEditionsForWork(ctx, WorkIDDune)
	if err != nil {
		t.Fatalf("ListEditionsForWork: %v", err)
	}
	if len(editions) != 5 {
		t.Fatalf("expected 5 editions for Dune (incl. Korean), got %d", len(editions))
	}

	// The Korean expression carries a translator credit.
	credits, err := repo.ListCreditsForTarget(ctx, "expression", ExprIDDuneKO)
	if err != nil {
		t.Fatalf("ListCreditsForTarget: %v", err)
	}
	foundTranslator := false
	for _, c := range credits {
		if c.Role == "translator" && c.PersonID != nil && *c.PersonID == PersonIDTranslator {
			foundTranslator = true
		}
	}
	if !foundTranslator {
		t.Fatalf("expected a translator credit on the Korean expression, got %+v", credits)
	}

	// The two same-title works remain distinct.
	seaA, err := repo.GetWork(ctx, WorkIDSameNameA)
	if err != nil {
		t.Fatalf("GetWork Silent Sea A: %v", err)
	}
	seaB, err := repo.GetWork(ctx, WorkIDSameNameB)
	if err != nil {
		t.Fatalf("GetWork Silent Sea B: %v", err)
	}
	if seaA.WorkID == seaB.WorkID {
		t.Fatal("expected distinct works for same-title hard negatives")
	}
}

func TestResolve_ResolvesUniqueIdentifier(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	res, err := repo.Resolve(context.Background(), "isbn13", "9780441013597")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Status != "resolved" {
		t.Fatalf("expected resolved, got %s", res.Status)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(res.Candidates))
	}
	if res.Candidates[0].TargetID != EdIDDuneEbook {
		t.Fatalf("expected edition %s, got %s", EdIDDuneEbook, res.Candidates[0].TargetID)
	}
}

func TestResolve_AmbiguousIdentifier(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	// 9780441172719 is claimed by both the print and ebook editions.
	res, err := repo.Resolve(context.Background(), "isbn13", "9780441172719")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Status != "ambiguous" {
		t.Fatalf("expected ambiguous, got %s", res.Status)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(res.Candidates))
	}
}

func TestResolve_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	res, err := repo.Resolve(context.Background(), "isbn13", "9999999999999")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Status != "not_found" {
		t.Fatalf("expected not_found, got %s", res.Status)
	}
}

func TestResolve_WorkLevelIdentifier(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	res, err := repo.Resolve(context.Background(), "openlibrary", "OL1234567W")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Status != "resolved" {
		t.Fatalf("expected resolved, got %s", res.Status)
	}
	if res.Candidates[0].TargetType != "work" || res.Candidates[0].TargetID != WorkIDDune {
		t.Fatalf("expected work Dune, got %s %s", res.Candidates[0].TargetType, res.Candidates[0].TargetID)
	}
}

func TestLoadFixtures_IsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Use a dedicated schema so the assertion is isolated from other tests
	// sharing the database.
	schema := "bookdb_idem_test"
	if _, err := db.ExecContext(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`); err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	ddl := []string{
		`CREATE TABLE ` + schema + `.credits (credit_id uuid, person_id uuid, organization_id uuid, role text NOT NULL, target_type text NOT NULL, target_id uuid NOT NULL, display_order integer NOT NULL DEFAULT 0, credited_as text, CONSTRAINT uq_credits UNIQUE (person_id, organization_id, role, target_type, target_id, display_order))`,
		`CREATE TABLE ` + schema + `.identifiers (identifier_id uuid, namespace text NOT NULL, normalized_value text NOT NULL, raw_value text, target_type text NOT NULL, target_id uuid NOT NULL, status text NOT NULL DEFAULT 'candidate', CONSTRAINT uq_identifiers UNIQUE (namespace, normalized_value, target_type, target_id))`,
	}
	for _, stmt := range ddl {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	// The fixture upsert pattern uses WHERE NOT EXISTS against the unique
	// constraints. Verify that re-inserting the same logical row does not
	// create a duplicate. (ON CONFLICT DO NOTHING does not work here because
	// NULLs in the unique constraint columns are not considered equal.)
	creditSQL := `INSERT INTO ` + schema + `.credits (person_id, role, target_type, target_id) SELECT $1, 'author', 'work', $2 WHERE NOT EXISTS (SELECT 1 FROM ` + schema + `.credits WHERE person_id = $1 AND role = 'author' AND target_type = 'work' AND target_id = $2)`
	if _, err := db.ExecContext(ctx, creditSQL, PersonIDFrank, WorkIDDune); err != nil {
		t.Fatalf("insert credit 1: %v", err)
	}
	if _, err := db.ExecContext(ctx, creditSQL, PersonIDFrank, WorkIDDune); err != nil {
		t.Fatalf("insert credit 2: %v", err)
	}
	var credits int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+schema+`.credits`).Scan(&credits); err != nil {
		t.Fatalf("count credits: %v", err)
	}
	if credits != 1 {
		t.Fatalf("expected 1 credit after idempotent upsert, got %d", credits)
	}

	identSQL := `INSERT INTO ` + schema + `.identifiers (namespace, normalized_value, target_type, target_id) SELECT $1, $2, 'edition', $3 WHERE NOT EXISTS (SELECT 1 FROM ` + schema + `.identifiers WHERE namespace = $1 AND normalized_value = $2 AND target_type = 'edition' AND target_id = $3)`
	if _, err := db.ExecContext(ctx, identSQL, "isbn13", "9780441172719", EdIDDunePrint); err != nil {
		t.Fatalf("insert identifier 1: %v", err)
	}
	if _, err := db.ExecContext(ctx, identSQL, "isbn13", "9780441172719", EdIDDunePrint); err != nil {
		t.Fatalf("insert identifier 2: %v", err)
	}
	var idents int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+schema+`.identifiers`).Scan(&idents); err != nil {
		t.Fatalf("count identifiers: %v", err)
	}
	if idents != 1 {
		t.Fatalf("expected 1 identifier after idempotent upsert, got %d", idents)
	}
}
