package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Fixture UUIDs are fixed so repeated loads are idempotent and tests can
// reference stable identities. They are clearly synthetic test data.
//
// Identity model (S1 correction): the Korean translation of Dune is an
// Expression of the single Dune Work, not a second canonical Work. A
// translation is a realization of the same intellectual creation; it must not
// be modelled as a duplicate Work. Genuinely different works that share a
// title (The Silent Sea A/B) and different people that share a name (Jane Doe
// A/B) remain separate.
const (
	WorkIDDune      = "11111111-1111-4111-8111-111111111111"
	WorkIDSameNameA = "11111111-1111-4111-8111-111111111113"
	WorkIDSameNameB = "11111111-1111-4111-8111-111111111114"

	ExprIDDuneEN     = "22222222-2222-4222-8222-222222222221"
	ExprIDDuneKO     = "22222222-2222-4222-8222-222222222222"
	ExprIDDuneAudioA = "22222222-2222-4222-8222-222222222223"
	ExprIDDuneAudioB = "22222222-2222-4222-8222-222222222224"
	ExprIDSameNameA  = "22222222-2222-4222-8222-222222222225"
	ExprIDSameNameB  = "22222222-2222-4222-8222-222222222226"

	EdIDDunePrint  = "33333333-3333-4333-8333-333333333331"
	EdIDDuneEbook  = "33333333-3333-4333-8333-333333333332"
	EdIDDuneAudioA = "33333333-3333-4333-8333-333333333333"
	EdIDDuneAudioB = "33333333-3333-4333-8333-333333333334"
	EdIDDuneKO     = "33333333-3333-4333-8333-333333333335"
	EdIDSameNameA  = "33333333-3333-4333-8333-333333333336"
	EdIDSameNameB  = "33333333-3333-4333-8333-333333333337"

	PersonIDFrank      = "44444444-4444-4444-8444-444444444441"
	PersonIDJane       = "44444444-4444-4444-8444-444444444442"
	PersonIDJaneB      = "44444444-4444-4444-8444-444444444443"
	PersonIDNarratorA  = "44444444-4444-4444-8444-444444444444"
	PersonIDNarratorB  = "44444444-4444-4444-8444-444444444445"
	PersonIDTranslator = "44444444-4444-4444-8444-444444444446"

	OrgIDAce     = "55555555-5555-4555-8555-555555555551"
	OrgIDPenguin = "55555555-5555-4555-8555-555555555552"
)

// LoadFixtures inserts the S1 synthetic fixture catalog. It is idempotent:
// re-running it does not change entity IDs or counts. All values are clearly
// synthetic test data, not real source metadata.
func LoadFixtures(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("catalog: begin fixture tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Targeted, idempotent repair of the S1 fixture correction: the Korean
	// translation of Dune was previously modelled as a second canonical Work
	// (…112). It is now an Expression of the Dune Work. Retire the stale
	// second-Work row if a database still holds it. This is a targeted repair
	// of this known fixture identity, not a general merge policy, and it
	// deletes only that specific synthetic row.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM bookdb.works
		WHERE work_id = '11111111-1111-4111-8111-111111111112'
		  AND canonical_title = 'Dune (Korean translation)'`); err != nil {
		return fmt.Errorf("catalog: retire stale Korean work: %w", err)
	}

	// Works. Dune is a single Work; its Korean translation is an Expression
	// of it (below), not a second Work. The two "The Silent Sea" works are
	// genuinely different works that share a title and stay separate.
	if err := upsertWork(ctx, tx, WorkIDDune, "Dune", "dune", "en"); err != nil {
		return err
	}
	if err := upsertWork(ctx, tx, WorkIDSameNameA, "The Silent Sea", "the silent sea", "en"); err != nil {
		return err
	}
	if err := upsertWork(ctx, tx, WorkIDSameNameB, "The Silent Sea", "the silent sea", "en"); err != nil {
		return err
	}

	// Expressions. The Korean translation is an Expression of the Dune Work.
	// Two same-language narrations of Dune must remain distinct.
	if err := upsertExpression(ctx, tx, ExprIDDuneEN, WorkIDDune, "en", "Dune"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDDuneKO, WorkIDDune, "ko", "Dune (Korean)"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDDuneAudioA, WorkIDDune, "en", "Dune (narration A)"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDDuneAudioB, WorkIDDune, "en", "Dune (narration B)"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDSameNameA, WorkIDSameNameA, "en", "The Silent Sea"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDSameNameB, WorkIDSameNameB, "en", "The Silent Sea"); err != nil {
		return err
	}

	// People: two distinct people with the same display name, plus the
	// translator of the Korean expression.
	if err := upsertPerson(ctx, tx, PersonIDFrank, "Frank Herbert", "Herbert, Frank"); err != nil {
		return err
	}
	if err := upsertPerson(ctx, tx, PersonIDJane, "Jane Doe", "Doe, Jane"); err != nil {
		return err
	}
	if err := upsertPerson(ctx, tx, PersonIDJaneB, "Jane Doe", "Doe, Jane"); err != nil {
		return err
	}
	if err := upsertPerson(ctx, tx, PersonIDNarratorA, "Narrator Alpha", "Alpha, Narrator"); err != nil {
		return err
	}
	if err := upsertPerson(ctx, tx, PersonIDNarratorB, "Narrator Beta", "Beta, Narrator"); err != nil {
		return err
	}
	if err := upsertPerson(ctx, tx, PersonIDTranslator, "Korean Translator", "Translator, Korean"); err != nil {
		return err
	}

	// Organizations.
	if err := upsertOrganization(ctx, tx, OrgIDAce, "Ace Books", "Ace Books", "publisher", nil); err != nil {
		return err
	}
	if err := upsertOrganization(ctx, tx, OrgIDPenguin, "Penguin", "Penguin", "publisher", nil); err != nil {
		return err
	}

	// Editions. Print/ebook/audio variants; one edition has no ISBN. The
	// Korean print edition is a manifestation of the Korean expression.
	if err := upsertEdition(ctx, tx, EdIDDunePrint, ExprIDDuneEN, "Dune (print)", strPtr("print"), strPtr("9780441172719"), date(1965, 8, 1), strPtr("Ace Books")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDDuneEbook, ExprIDDuneEN, "Dune (ebook)", strPtr("ebook"), strPtr("9780441013597"), date(1999, 1, 1), strPtr("Penguin")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDDuneAudioA, ExprIDDuneAudioA, "Dune (audio A)", strPtr("audio"), nil, nil, strPtr("Penguin")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDDuneAudioB, ExprIDDuneAudioB, "Dune (audio B)", strPtr("audio"), nil, nil, strPtr("Penguin")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDDuneKO, ExprIDDuneKO, "Dune (Korean print)", strPtr("print"), strPtr("9788900000001"), date(1990, 3, 1), strPtr("Korean Publisher")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDSameNameA, ExprIDSameNameA, "The Silent Sea (A)", strPtr("print"), strPtr("9780000000001"), date(2001, 1, 1), strPtr("Ace Books")); err != nil {
		return err
	}
	if err := upsertEdition(ctx, tx, EdIDSameNameB, ExprIDSameNameB, "The Silent Sea (B)", strPtr("print"), strPtr("9780000000002"), date(2005, 1, 1), strPtr("Penguin")); err != nil {
		return err
	}

	// Edition contents: each edition contains its expression.
	if err := upsertEditionContent(ctx, tx, EdIDDunePrint, ExprIDDuneEN, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDDuneEbook, ExprIDDuneEN, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDDuneAudioA, ExprIDDuneAudioA, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDDuneAudioB, ExprIDDuneAudioB, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDDuneKO, ExprIDDuneKO, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDSameNameA, ExprIDSameNameA, 0); err != nil {
		return err
	}
	if err := upsertEditionContent(ctx, tx, EdIDSameNameB, ExprIDSameNameB, 0); err != nil {
		return err
	}

	// Credits: author, translator, narrators, publisher.
	if err := upsertCredit(ctx, tx, strPtr(PersonIDFrank), nil, "author", "work", WorkIDDune, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDJane), nil, "author", "work", WorkIDSameNameA, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDJaneB), nil, "author", "work", WorkIDSameNameB, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDTranslator), nil, "translator", "expression", ExprIDDuneKO, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDNarratorA), nil, "narrator", "expression", ExprIDDuneAudioA, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDNarratorB), nil, "narrator", "expression", ExprIDDuneAudioB, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, nil, strPtr(OrgIDAce), "publisher", "edition", EdIDDunePrint, 0, nil); err != nil {
		return err
	}

	// Identifiers: ISBNs (edition-scoped), a conflicting ISBN claim, and a
	// missing-ISBN case (audio editions have no ISBN identifier).
	if err := upsertIdentifier(ctx, tx, "isbn13", "9780441172719", "978-0-441-17271-9", "edition", EdIDDunePrint, "resolved"); err != nil {
		return err
	}
	if err := upsertIdentifier(ctx, tx, "isbn13", "9780441013597", "978-0-441-01359-7", "edition", EdIDDuneEbook, "resolved"); err != nil {
		return err
	}
	if err := upsertIdentifier(ctx, tx, "isbn13", "9788900000001", "978-89-0000000-1", "edition", EdIDDuneKO, "resolved"); err != nil {
		return err
	}
	if err := upsertIdentifier(ctx, tx, "isbn13", "9780000000001", "978-0-000000000-1", "edition", EdIDSameNameA, "resolved"); err != nil {
		return err
	}
	if err := upsertIdentifier(ctx, tx, "isbn13", "9780000000002", "978-0-000000000-2", "edition", EdIDSameNameB, "resolved"); err != nil {
		return err
	}
	// Conflicting identifier claim: the same ISBN asserted for a different
	// edition. Both claims are preserved; resolution returns ambiguity.
	if err := upsertIdentifier(ctx, tx, "isbn13", "9780441172719", "978-0-441-17271-9", "edition", EdIDDuneEbook, "conflict"); err != nil {
		return err
	}
	// A work-level identifier (source ID) for Dune.
	if err := upsertIdentifier(ctx, tx, "openlibrary", "OL1234567W", "OL1234567W", "work", WorkIDDune, "resolved"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("catalog: commit fixtures: %w", err)
	}
	return nil
}

func upsertWork(ctx context.Context, tx *sql.Tx, id, title, normalized, lang string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.works (work_id, canonical_title, normalized_title, language_code)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (work_id) DO UPDATE SET
			canonical_title = EXCLUDED.canonical_title,
			normalized_title = EXCLUDED.normalized_title,
			language_code = EXCLUDED.language_code`,
		id, title, normalized, lang)
	if err != nil {
		return fmt.Errorf("catalog: upsert work %s: %w", id, err)
	}
	return nil
}

func upsertExpression(ctx context.Context, tx *sql.Tx, id, workID, lang, title string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.expressions (expression_id, work_id, language_code, expression_title)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (expression_id) DO UPDATE SET
			work_id = EXCLUDED.work_id,
			language_code = EXCLUDED.language_code,
			expression_title = EXCLUDED.expression_title`,
		id, workID, lang, title)
	if err != nil {
		return fmt.Errorf("catalog: upsert expression %s: %w", id, err)
	}
	return nil
}

func upsertPerson(ctx context.Context, tx *sql.Tx, id, display, sort string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.people (person_id, display_name, sort_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (person_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			sort_name = EXCLUDED.sort_name`,
		id, display, sort)
	if err != nil {
		return fmt.Errorf("catalog: upsert person %s: %w", id, err)
	}
	return nil
}

func upsertOrganization(ctx context.Context, tx *sql.Tx, id, display, sort, kind string, parent *string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.organizations (organization_id, display_name, sort_name, organization_kind, parent_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (organization_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			sort_name = EXCLUDED.sort_name,
			organization_kind = EXCLUDED.organization_kind,
			parent_id = EXCLUDED.parent_id`,
		id, display, sort, kind, parent)
	if err != nil {
		return fmt.Errorf("catalog: upsert organization %s: %w", id, err)
	}
	return nil
}

func upsertEdition(ctx context.Context, tx *sql.Tx, id, exprID, title string, format, isbn *string, pubDate *time.Time, publisher *string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.editions (edition_id, expression_id, edition_title, format, isbn13, publication_date, publisher_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (edition_id) DO UPDATE SET
			expression_id = EXCLUDED.expression_id,
			edition_title = EXCLUDED.edition_title,
			format = EXCLUDED.format,
			isbn13 = EXCLUDED.isbn13,
			publication_date = EXCLUDED.publication_date,
			publisher_name = EXCLUDED.publisher_name`,
		id, exprID, title, format, isbn, pubDate, publisher)
	if err != nil {
		return fmt.Errorf("catalog: upsert edition %s: %w", id, err)
	}
	return nil
}

func upsertEditionContent(ctx context.Context, tx *sql.Tx, editionID, exprID string, position int) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.edition_contents (edition_id, expression_id, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (edition_id, expression_id, position) DO NOTHING`,
		editionID, exprID, position)
	if err != nil {
		return fmt.Errorf("catalog: upsert edition content: %w", err)
	}
	return nil
}

func upsertCredit(ctx context.Context, tx *sql.Tx, personID, orgID *string, role, targetType, targetID string, order int, creditedAs *string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.credits (person_id, organization_id, role, target_type, target_id, display_order, credited_as)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE NOT EXISTS (
			SELECT 1 FROM bookdb.credits
			WHERE person_id IS NOT DISTINCT FROM $1
			  AND organization_id IS NOT DISTINCT FROM $2
			  AND role = $3
			  AND target_type = $4
			  AND target_id = $5
			  AND display_order = $6
		)`,
		personID, orgID, role, targetType, targetID, order, creditedAs)
	if err != nil {
		return fmt.Errorf("catalog: upsert credit: %w", err)
	}
	return nil
}

func upsertIdentifier(ctx context.Context, tx *sql.Tx, namespace, normalized, raw, targetType, targetID, status string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.identifiers (namespace, normalized_value, raw_value, target_type, target_id, status)
		SELECT $1, $2, $3, $4, $5, $6
		WHERE NOT EXISTS (
			SELECT 1 FROM bookdb.identifiers
			WHERE namespace = $1
			  AND normalized_value = $2
			  AND target_type = $4
			  AND target_id = $5
		)`,
		namespace, normalized, raw, targetType, targetID, status)
	if err != nil {
		return fmt.Errorf("catalog: upsert identifier: %w", err)
	}
	return nil
}

func date(y, m int, d int) *time.Time {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return &t
}

// strPtr returns a pointer to the given string, for nullable fixture fields.
func strPtr(s string) *string { return &s }

// FixtureReport is the read-only verification result for the S1 fixture graph.
// It reports relationships and counts so a human can inspect correctness
// without inferring it from row counts alone.
type FixtureReport struct {
	Counts        map[string]int `json:"counts"`
	Relationships []string       `json:"relationships"`
	Errors        []string       `json:"errors,omitempty"`
}

// VerifyFixtures performs a read-only verification of the S1 fixture graph.
// It reports graph relationships and counts, and returns a non-nil error (with
// the broken assertions listed) when any expected relationship is missing. It
// never mutates the database.
func VerifyFixtures(ctx context.Context, db *sql.DB) (FixtureReport, error) {
	var rep FixtureReport
	rep.Counts = make(map[string]int)
	fail := func(format string, args ...any) {
		rep.Errors = append(rep.Errors, fmt.Sprintf(format, args...))
	}
	ok := func(rel string) { rep.Relationships = append(rep.Relationships, rel) }

	count := func(table string) int {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.`+table).Scan(&n); err != nil {
			fail("count %s: %v", table, err)
			return 0
		}
		rep.Counts[table] = n
		return n
	}

	count("works")
	count("expressions")
	count("editions")
	count("people")
	count("organizations")
	count("credits")
	count("identifiers")
	count("edition_contents")

	// 1. The Korean translation is an Expression of the Dune Work, not a
	//    second Work.
	var koWork string
	err := db.QueryRowContext(ctx, `SELECT work_id FROM bookdb.expressions WHERE expression_id = $1`, ExprIDDuneKO).Scan(&koWork)
	if err != nil {
		fail("Korean expression %s not found: %v", ExprIDDuneKO, err)
	} else if koWork != WorkIDDune {
		fail("Korean expression %s belongs to work %s, expected %s", ExprIDDuneKO, koWork, WorkIDDune)
	} else {
		ok("Korean translation is an Expression of the Dune Work")
	}
	var stale int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.works WHERE work_id = '11111111-1111-4111-8111-111111111112'`).Scan(&stale); err == nil {
		if stale != 0 {
			fail("retired second Korean Work still present (%d rows)", stale)
		} else {
			ok("retired second Korean Work is absent")
		}
	}

	// 2. Distinct same-language narrations of Dune remain separate.
	var narrA, narrB string
	if err := db.QueryRowContext(ctx, `SELECT work_id FROM bookdb.expressions WHERE expression_id = $1`, ExprIDDuneAudioA).Scan(&narrA); err == nil {
		if err := db.QueryRowContext(ctx, `SELECT work_id FROM bookdb.expressions WHERE expression_id = $1`, ExprIDDuneAudioB).Scan(&narrB); err == nil {
			if narrA == WorkIDDune && narrB == WorkIDDune && ExprIDDuneAudioA != ExprIDDuneAudioB {
				ok("two same-language narrations are distinct expressions of Dune")
			} else {
				fail("narrations not both distinct expressions of Dune (A=%s B=%s)", narrA, narrB)
			}
		}
	}

	// 3. Hard negatives: same-title works and same-name people stay separate.
	var seaA, seaB string
	if err := db.QueryRowContext(ctx, `SELECT work_id FROM bookdb.works WHERE work_id = $1`, WorkIDSameNameA).Scan(&seaA); err == nil {
		if err := db.QueryRowContext(ctx, `SELECT work_id FROM bookdb.works WHERE work_id = $1`, WorkIDSameNameB).Scan(&seaB); err == nil {
			if seaA != seaB {
				ok("same-title works (The Silent Sea A/B) remain distinct")
			} else {
				fail("same-title works were merged")
			}
		}
	}
	var janeA, janeB string
	if err := db.QueryRowContext(ctx, `SELECT display_name FROM bookdb.people WHERE person_id = $1`, PersonIDJane).Scan(&janeA); err == nil {
		if err := db.QueryRowContext(ctx, `SELECT display_name FROM bookdb.people WHERE person_id = $1`, PersonIDJaneB).Scan(&janeB); err == nil {
			if janeA == janeB && PersonIDJane != PersonIDJaneB {
				ok("same-name people (Jane Doe A/B) remain distinct")
			} else {
				fail("same-name people not distinct or not same-named")
			}
		}
	}

	// 4. Identifier ambiguity: the conflicting ISBN resolves to two editions.
	var amb int
	if err := db.QueryRowContext(ctx, `SELECT count(DISTINCT target_id) FROM bookdb.identifiers WHERE namespace='isbn13' AND normalized_value='9780441172719'`).Scan(&amb); err == nil {
		if amb == 2 {
			ok("conflicting ISBN 9780441172719 is explicit ambiguity (2 editions)")
		} else {
			fail("conflicting ISBN expected 2 distinct targets, got %d", amb)
		}
	}

	// 5. Reload idempotency: re-running LoadFixtures must not change counts.
	before := map[string]int{}
	for k, v := range rep.Counts {
		before[k] = v
	}
	if err := LoadFixtures(ctx, db); err != nil {
		fail("reload fixtures: %v", err)
	} else {
		changed := false
		for table, n := range rep.Counts {
			var after int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM bookdb.`+table).Scan(&after); err == nil && after != n {
				fail("reload changed %s count %d -> %d", table, n, after)
				changed = true
			}
		}
		if !changed {
			ok("fixture reload is idempotent (counts unchanged)")
		}
	}

	if len(rep.Errors) > 0 {
		return rep, fmt.Errorf("catalog: fixture verification failed with %d error(s)", len(rep.Errors))
	}
	return rep, nil
}
