package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Fixture UUIDs are fixed so repeated loads are idempotent and tests can
// reference stable identities. They are clearly synthetic test data.
const (
	WorkIDDune       = "11111111-1111-4111-8111-111111111111"
	WorkIDDuneKorean = "11111111-1111-4111-8111-111111111112"
	WorkIDSameNameA  = "11111111-1111-4111-8111-111111111113"
	WorkIDSameNameB  = "11111111-1111-4111-8111-111111111114"

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

	PersonIDFrank     = "44444444-4444-4444-8444-444444444441"
	PersonIDJane      = "44444444-4444-4444-8444-444444444442"
	PersonIDJaneB     = "44444444-4444-4444-8444-444444444443"
	PersonIDNarratorA = "44444444-4444-4444-8444-444444444444"
	PersonIDNarratorB = "44444444-4444-4444-8444-444444444445"

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

	// Works.
	if err := upsertWork(ctx, tx, WorkIDDune, "Dune", "dune", "en"); err != nil {
		return err
	}
	if err := upsertWork(ctx, tx, WorkIDDuneKorean, "Dune (Korean translation)", "dune korean translation", "ko"); err != nil {
		return err
	}
	if err := upsertWork(ctx, tx, WorkIDSameNameA, "The Silent Sea", "the silent sea", "en"); err != nil {
		return err
	}
	if err := upsertWork(ctx, tx, WorkIDSameNameB, "The Silent Sea", "the silent sea", "en"); err != nil {
		return err
	}

	// Expressions. Two same-language narrations of Dune must remain distinct.
	if err := upsertExpression(ctx, tx, ExprIDDuneEN, WorkIDDune, "en", "Dune"); err != nil {
		return err
	}
	if err := upsertExpression(ctx, tx, ExprIDDuneKO, WorkIDDuneKorean, "ko", "Dune (Korean)"); err != nil {
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

	// People: two distinct people with the same display name.
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

	// Organizations.
	if err := upsertOrganization(ctx, tx, OrgIDAce, "Ace Books", "Ace Books", "publisher", nil); err != nil {
		return err
	}
	if err := upsertOrganization(ctx, tx, OrgIDPenguin, "Penguin", "Penguin", "publisher", nil); err != nil {
		return err
	}

	// Editions. Print/ebook/audio variants; one edition has no ISBN.
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

	// Credits: author, narrators, publisher.
	if err := upsertCredit(ctx, tx, strPtr(PersonIDFrank), nil, "author", "work", WorkIDDune, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDJane), nil, "author", "work", WorkIDSameNameA, 0, nil); err != nil {
		return err
	}
	if err := upsertCredit(ctx, tx, strPtr(PersonIDJaneB), nil, "author", "work", WorkIDSameNameB, 0, nil); err != nil {
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
