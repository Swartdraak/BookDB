// Package promotion implements the S2 promotion pipeline: source records
// (evidence) are resolved to canonical catalog entities (works, expressions,
// editions, people) with field provenance, canonical revisions and change
// feed entries.
//
// The pipeline follows docs/bookdb/04-ingestion-reconciliation.md:
//
//	Parse -> Persist evidence -> Resolve -> Reconcile -> Publish
//
// Promotion is the Resolve/Reconcile/Publish stage for source records that
// were persisted by the S2 ingestion pipeline. It is deterministic and
// idempotent: replaying the same source record does not create duplicate
// canonical entities or change feed entries.
//
// Identity policy (matching policy in docs/bookdb/04-ingestion-reconciliation.md):
// a source record resolves to an existing canonical entity only via a valid
// shared identifier (bookdb.identifiers, status 'resolved'). Similar titles
// are candidate generation only; the same person name is never an auto-merge.
// Unknown or ambiguous identity creates a separate provisional canonical
// entity with a 'candidate' identifier, and no cross-source merge is
// performed.
package promotion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bookdb/bookdb/internal/ingestion"
	"github.com/bookdb/bookdb/internal/reconciliation"
)

// SourceName is the canonical source name for Open Library records.
const SourceName = ingestion.SourceName

// IdentifierNamespace is the namespace used for source key identifiers.
const IdentifierNamespace = "openlibrary"

// Record is a normalized bibliographic record from a source. It is the
// ingestion package's record type so the stored content hash is
// byte-compatible with bookdb.source_records.
type Record = ingestion.Record

// hashRecord computes the content hash exactly as the ingestion pipeline's
// hashRecord (source_key + source_type + title + raw JSON, sha256, hex), so
// promotion's idempotency check compares against the hash stored in
// bookdb.source_records.
func hashRecord(rec *Record) string {
	h := sha256.New()
	h.Write([]byte(rec.SourceKey))
	h.Write([]byte(rec.SourceType))
	h.Write([]byte(rec.Title))
	h.Write(rec.RawJSON)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Outcome is the result of promoting a single source record.
type Outcome struct {
	EntityType   string `json:"entity_type"` // work, edition, person
	EntityID     string `json:"entity_id"`   // canonical entity id
	ChangeType   string `json:"change_type"` // created or updated
	ChangeID     int64  `json:"change_id"`   // bookdb.change_feed change_id
	Revision     int64  `json:"revision"`    // canonical revision after promote
	OutboxEvent  int64  `json:"outbox_event"`
	Provisional  bool   `json:"provisional"`
	SourceRecord string `json:"source_record"` // source_name/source_key
}

// Promoter performs source-record-to-canonical promotion.
type Promoter struct {
	db *sql.DB
}

// NewPromoter creates a Promoter backed by the given database.
func NewPromoter(db *sql.DB) *Promoter {
	return &Promoter{db: db}
}

// Promote resolves and promotes a single source record into the canonical
// catalog. It is idempotent per (source_name, source_key, content_hash): the
// stored content hash is re-checked and a record whose stored hash already
// matches is a no-op (Outcome.ChangeType == "").
//
// All canonical writes (entity upsert, identifiers, provenance, revisions,
// change feed, canonical outbox event) happen in one transaction.
func (p *Promoter) Promote(ctx context.Context, rec *Record) (*Outcome, error) {
	if rec == nil {
		return nil, fmt.Errorf("promotion: nil record")
	}
	if rec.SourceKey == "" {
		return nil, fmt.Errorf("promotion: record has no source key")
	}
	switch rec.SourceType {
	case "work", "edition", "author":
	default:
		return nil, fmt.Errorf("promotion: unsupported source type %q", rec.SourceType)
	}

	entityType := entityTypeFor(rec)

	// Idempotency: skip when the stored content hash matches this record.
	// The canonical entity this record promoted to is stored in the source
	// record payload (source_record.entity_id), so replay returns a stable
	// mapping even before the idempotency gate runs.
	storedHash, entityID, err := p.sourceRecordState(ctx, SourceName, rec.SourceKey)
	if err != nil {
		return nil, err
	}
	recHash := hashRecord(rec)
	if storedHash != "" && storedHash == recHash {
		rev, _ := p.revision(ctx, entityType, entityID)
		return &Outcome{
			EntityType:   entityType,
			EntityID:     entityID,
			ChangeType:   "",
			Revision:     rev,
			SourceRecord: SourceName + "/" + rec.SourceKey,
		}, nil
	}

	// Resolve identity: a 'resolved' identifier for this source key points at
	// the canonical entity. Otherwise a provisional entity is created.
	targetID, provisional, err := p.resolveTarget(ctx, entityType, rec)
	if err != nil {
		return nil, err
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("promotion: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	changeType := "updated"
	if provisional {
		changeType = "created"
	}

	switch entityType {
	case "work":
		if err := p.upsertWork(ctx, tx, rec, targetID); err != nil {
			return nil, err
		}
	case "edition":
		if err := p.upsertEdition(ctx, tx, rec, targetID); err != nil {
			return nil, err
		}
	case "person":
		if err := p.upsertPerson(ctx, tx, rec, targetID); err != nil {
			return nil, err
		}
	}

	// Persist the promoted entity mapping into the source record payload so
	// idempotent replays can return a stable entity without side effects.
	if err := p.linkSourceRecord(ctx, tx, rec, targetID); err != nil {
		return nil, err
	}

	// Record field provenance for the values this source asserted.
	if err := p.recordProvenance(ctx, tx, entityType, targetID, rec); err != nil {
		return nil, err
	}

	// Record/refresh the source key identifier for this record.
	if err := p.recordIdentifier(ctx, tx, rec, targetID, provisional); err != nil {
		return nil, err
	}

	// Increment the canonical revision.
	newRevision, err := p.bumpRevision(ctx, tx, entityType, targetID)
	if err != nil {
		return nil, err
	}

	// Append the change feed entry.
	payload, _ := json.Marshal(map[string]any{
		"source_name": SourceName,
		"source_key":  rec.SourceKey,
		"source_type": rec.SourceType,
		"title":       rec.Title,
		"provisional": provisional,
	})
	var changeID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.change_feed (entity_type, entity_id, change_type, payload, revision)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING change_id`,
		entityType, targetID, changeType, payload, newRevision).Scan(&changeID)
	if err != nil {
		return nil, fmt.Errorf("promotion: change feed: %w", err)
	}

	// Publish the canonical outbox event for the search projection.
	var outboxID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		entityType, targetID, entityType+".promoted", p.canonicalPayload(rec, targetID)).Scan(&outboxID)
	if err != nil {
		return nil, fmt.Errorf("promotion: canonical outbox: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("promotion: commit: %w", err)
	}

	return &Outcome{
		EntityType:   entityType,
		EntityID:     targetID,
		ChangeType:   changeType,
		ChangeID:     changeID,
		Revision:     newRevision,
		OutboxEvent:  outboxID,
		Provisional:  provisional,
		SourceRecord: SourceName + "/" + rec.SourceKey,
	}, nil
}

// PromoteRecord promotes an ingestion record (as returned by
// ingestion.ReadSnapshot / ParseOpenLibraryLine) to the canonical catalog.
// When the raw JSON line was not retained it is re-serialized so the content
// hash matches a deterministic form (unit tests constructing records).
func (p *Promoter) PromoteRecord(ctx context.Context, rec *ingestion.Record) (*Outcome, error) {
	if rec.RawJSON == nil && rec.SourceKey != "" {
		rec.RawJSON, _ = json.Marshal(rec)
	}
	return p.Promote(ctx, rec)
}

// ---------------------------------------------------------------------------
// Identity resolution
// ---------------------------------------------------------------------------

// resolveTarget returns the canonical entity id for a source record, or a new
// id when the record has not been resolved to an existing entity. Identity is
// only carried by a 'resolved' identifier row for this source key.
func (p *Promoter) resolveTarget(ctx context.Context, entityType string, rec *Record) (string, bool, error) {
	var id string
	err := p.db.QueryRowContext(ctx, `
		SELECT target_id::text FROM bookdb.identifiers
		WHERE namespace = $1 AND normalized_value = $2 AND target_type = $3 AND status = 'resolved'
		ORDER BY target_id
		LIMIT 1`,
		IdentifierNamespace, rec.SourceKey, entityType).Scan(&id)
	if err == sql.ErrNoRows {
		return uuid.New().String(), true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("promotion: resolve target: %w", err)
	}
	return id, false, nil
}

// sourceRecordState returns the stored content hash and, when present, the
// canonical entity id the source record last promoted to (persisted in the
// payload under source_record.entity_id by linkSourceRecord).
func (p *Promoter) sourceRecordState(ctx context.Context, sourceName, sourceKey string) (string, string, error) {
	var hash sql.NullString
	var payload []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT content_hash, payload FROM bookdb.source_records
		WHERE source_name = $1 AND source_key = $2`,
		sourceName, sourceKey).Scan(&hash, &payload)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("promotion: source record state: %w", err)
	}
	entityID := ""
	if len(payload) > 0 {
		var stored struct {
			SourceRecord struct {
				EntityID string `json:"entity_id"`
			} `json:"source_record"`
		}
		if err := json.Unmarshal(payload, &stored); err == nil {
			entityID = stored.SourceRecord.EntityID
		}
	}
	return hash.String, entityID, nil
}

// linkSourceRecord stores the promoted entity id in the source record payload
// (source_record.entity_id), preserving the rest of the stored record, so a
// later replay of the same content hash maps back to the same entity. When
// the source record row is absent (unit-level promotion of a fresh record)
// the mapping is skipped: there is no row to anchor it and no replay risk.
func (p *Promoter) linkSourceRecord(ctx context.Context, tx *sql.Tx, rec *Record, entityID string) error {
	payload, _ := json.Marshal(rec)
	var stored map[string]any
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &stored)
	}
	if stored == nil {
		stored = map[string]any{}
	}
	stored["source_record"] = map[string]any{"entity_id": entityID}
	merged, _ := json.Marshal(stored)

	n, err := tx.ExecContext(ctx, `
		UPDATE bookdb.source_records SET payload = $1
		WHERE source_name = $2 AND source_key = $3`,
		merged, SourceName, rec.SourceKey)
	if err != nil {
		return fmt.Errorf("promotion: link source record: %w", err)
	}
	affected, _ := n.RowsAffected()
	if affected == 1 {
		return nil
	}
	// Row absent: insert the evidence anchor with the mapping embedded.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.source_records (source_name, source_key, content_hash, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_name, source_key) DO NOTHING`,
		SourceName, rec.SourceKey, hashRecord(rec), merged); err != nil {
		return fmt.Errorf("promotion: link source record insert: %w", err)
	}
	return nil
}

// recordIdentifier stores the source key -> canonical entity identifier. A
// provisional (auto-created) entity is recorded as 'candidate' so that later
// resolution (e.g. cross-source matching) is explicit. An entity reached via
// an existing 'resolved' identifier keeps 'resolved'.
func (p *Promoter) recordIdentifier(ctx context.Context, tx *sql.Tx, rec *Record, targetID string, provisional bool) error {
	status := "candidate"
	if !provisional {
		status = "resolved"
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.identifiers (namespace, normalized_value, raw_value, target_type, target_id, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (namespace, normalized_value, target_type, target_id)
		DO UPDATE SET status = EXCLUDED.status, updated_at = now()`,
		IdentifierNamespace, rec.SourceKey, rec.SourceKey, entityTypeFor(rec), targetID, status)
	if err != nil {
		return fmt.Errorf("promotion: record identifier: %w", err)
	}
	return nil
}

func entityTypeFor(rec *Record) string {
	if rec.SourceType == "author" {
		return "person"
	}
	return rec.SourceType
}

// ---------------------------------------------------------------------------
// Canonical entity upserts
// ---------------------------------------------------------------------------

func (p *Promoter) upsertWork(ctx context.Context, tx *sql.Tx, rec *Record, workID string) error {
	var langArg any
	if rec.Language != "" {
		langArg = rec.Language
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.works (work_id, canonical_title, normalized_title, language_code)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (work_id) DO UPDATE SET
			canonical_title = EXCLUDED.canonical_title,
			normalized_title = EXCLUDED.normalized_title,
			language_code = COALESCE(EXCLUDED.language_code, bookdb.works.language_code),
			updated_at = now()`,
		workID, rec.Title, reconciliation.NormalizeTitle(rec.Title), langArg)
	if err != nil {
		return fmt.Errorf("promotion: upsert work: %w", err)
	}
	return nil
}

func (p *Promoter) upsertPerson(ctx context.Context, tx *sql.Tx, rec *Record, personID string) error {
	sortName := sortNameFor(rec.Title)
	var sortArg any
	if sortName != "" {
		sortArg = sortName
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.people (person_id, display_name, sort_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (person_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			sort_name = COALESCE(EXCLUDED.sort_name, bookdb.people.sort_name),
			updated_at = now()`,
		personID, rec.Title, sortArg)
	if err != nil {
		return fmt.Errorf("promotion: upsert person: %w", err)
	}
	return nil
}

func (p *Promoter) upsertEdition(ctx context.Context, tx *sql.Tx, rec *Record, editionID string) error {
	// An edition belongs to a work. Resolution order:
	//  1. the work referenced by the edition's OL work key, if a resolved
	//     identifier for that work key exists;
	//  2. otherwise a provisional work is created for the edition's title.
	workID, err := p.resolveEditionWork(ctx, tx, rec)
	if err != nil {
		return err
	}

	// Expression for this edition under the resolved work. Deterministic per
	// (work, language, title) so re-promotion of changed content reuses it.
	lang := rec.Language
	if lang == "" {
		lang = "en"
	}
	exprID := ""
	err = tx.QueryRowContext(ctx, `
		SELECT expression_id::text FROM bookdb.expressions
		WHERE work_id = $1 AND language_code = $2 AND expression_title = $3
		LIMIT 1`,
		workID, lang, rec.Title).Scan(&exprID)
	if err == sql.ErrNoRows {
		exprID = uuid.New().String()
	} else if err != nil {
		return fmt.Errorf("promotion: lookup expression: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.expressions (expression_id, work_id, language_code, expression_title)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (expression_id) DO UPDATE SET
			work_id = EXCLUDED.work_id,
			language_code = EXCLUDED.language_code,
			expression_title = EXCLUDED.expression_title,
			updated_at = now()`,
		exprID, workID, lang, rec.Title); err != nil {
		return fmt.Errorf("promotion: upsert expression: %w", err)
	}

	format := nullOrString(rec.Format)
	isbn := nullOrString(firstISBN(rec.ISBNs))
	pubDate := parsePublishDate(rec.PublishDate)
	publisher := nullOrString(rec.Publisher)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO bookdb.editions (edition_id, expression_id, edition_title, format, isbn13, publication_date, publisher_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (edition_id) DO UPDATE SET
			expression_id = EXCLUDED.expression_id,
			edition_title = EXCLUDED.edition_title,
			format = COALESCE(EXCLUDED.format, bookdb.editions.format),
			isbn13 = COALESCE(EXCLUDED.isbn13, bookdb.editions.isbn13),
			publication_date = COALESCE(EXCLUDED.publication_date, bookdb.editions.publication_date),
			publisher_name = COALESCE(EXCLUDED.publisher_name, bookdb.editions.publisher_name),
			updated_at = now()`,
		editionID, exprID, rec.Title, format, isbn, pubDate, publisher)
	if err != nil {
		return fmt.Errorf("promotion: upsert edition: %w", err)
	}

	// Edition contents junction (position 0: primary expression).
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.edition_contents (edition_id, expression_id, position)
		VALUES ($1, $2, 0)
		ON CONFLICT (edition_id, expression_id, position) DO NOTHING`,
		editionID, exprID); err != nil {
		return fmt.Errorf("promotion: edition contents: %w", err)
	}
	return nil
}

// resolveEditionWork finds the work an edition record belongs to. It never
// merges on title alone across sources; it uses the explicit OL work key
// when a resolved identifier exists, otherwise it creates a provisional work
// (per the matching policy: unknown identity -> separate provisional entity).
// The provisional work row is upserted inside the caller's transaction so it
// exists before the edition's expression references it.
func (p *Promoter) resolveEditionWork(ctx context.Context, tx *sql.Tx, rec *Record) (string, error) {
	workKey := workKeyFromRecord(rec)
	if workKey != "" {
		var workID string
		err := p.db.QueryRowContext(ctx, `
			SELECT target_id::text FROM bookdb.identifiers
			WHERE namespace = $1 AND normalized_value = $2 AND target_type = 'work' AND status = 'resolved'
			LIMIT 1`,
			IdentifierNamespace, workKey).Scan(&workID)
		if err == nil {
			return workID, nil
		}
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("promotion: resolve edition work: %w", err)
		}
	}
	workID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bookdb.works (work_id, canonical_title, normalized_title)
		VALUES ($1, $2, $3)
		ON CONFLICT (work_id) DO NOTHING`,
		workID, rec.Title, reconciliation.NormalizeTitle(rec.Title)); err != nil {
		return "", fmt.Errorf("promotion: provisional work: %w", err)
	}
	return workID, nil
}

// ---------------------------------------------------------------------------
// Provenance, revisions, payloads, helpers
// ---------------------------------------------------------------------------

// recordProvenance stores field-level provenance for the fields this source
// asserted. Each row is upserted and confirmed with rowcount 1 (insert or
// update); a zero rowcount indicates the write did not land and is an error.
func (p *Promoter) recordProvenance(ctx context.Context, tx *sql.Tx, entityType string, entityID string, rec *Record) error {
	entityUUID, err := uuid.Parse(entityID)
	if err != nil {
		return fmt.Errorf("promotion: entity id is not a uuid: %w", err)
	}
	for name, value := range fieldsFor(rec) {
		raw, _ := json.Marshal(value)
		res, err := tx.ExecContext(ctx, `
			INSERT INTO bookdb.field_provenance (entity_type, entity_id, field_name, source_name, source_key, value)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (entity_type, entity_id, field_name, source_name, source_key)
			DO UPDATE SET value = EXCLUDED.value, selected = true`,
			entityType, entityUUID, name, SourceName, rec.SourceKey, raw)
		if err != nil {
			return fmt.Errorf("promotion: field provenance %q: %w", name, err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return fmt.Errorf("promotion: field provenance %q: %d rows affected, want 1", name, n)
		}
	}
	return nil
}

// fieldsFor extracts the asserted fields for a record, keyed by field name.
func fieldsFor(rec *Record) map[string]any {
	fields := make(map[string]any)
	switch rec.SourceType {
	case "work":
		if rec.Title != "" {
			fields["title"] = rec.Title
		}
		if rec.Language != "" {
			fields["language"] = rec.Language
		}
		if len(rec.Authors) > 0 {
			fields["authors"] = rec.Authors
		}
	case "edition":
		if rec.Title != "" {
			fields["title"] = rec.Title
		}
		if rec.Format != "" {
			fields["format"] = rec.Format
		}
		if len(rec.ISBNs) > 0 {
			fields["isbn13"] = firstISBN(rec.ISBNs)
		}
		if rec.Publisher != "" {
			fields["publisher"] = rec.Publisher
		}
		if rec.PublishDate != "" {
			fields["publish_date"] = rec.PublishDate
		}
	case "author":
		if rec.Title != "" {
			fields["name"] = rec.Title
		}
	}
	return fields
}

// canonicalPayload builds the outbox payload for the search projection. The
// projection reads this payload directly, so it carries the display fields.
func (p *Promoter) canonicalPayload(rec *Record, entityID string) []byte {
	doc := map[string]any{
		"entity_id":  entityID,
		"source_key": rec.SourceKey,
	}
	switch rec.SourceType {
	case "work":
		doc["title"] = rec.Title
		doc["normalized"] = reconciliation.NormalizeTitle(rec.Title)
		if rec.Language != "" {
			doc["language"] = rec.Language
		}
		if len(rec.Authors) > 0 {
			doc["authors"] = rec.Authors
		}
	case "edition":
		doc["title"] = rec.Title
		doc["normalized"] = reconciliation.NormalizeTitle(rec.Title)
		if rec.Format != "" {
			doc["format"] = rec.Format
		}
		if rec.Publisher != "" {
			doc["publisher"] = rec.Publisher
		}
	case "author":
		doc["title"] = rec.Title
		doc["normalized"] = reconciliation.NormalizeTitle(rec.Title)
	}
	raw, _ := json.Marshal(doc)
	return raw
}

func (p *Promoter) bumpRevision(ctx context.Context, tx *sql.Tx, entityType string, entityID string) (int64, error) {
	var revision int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO bookdb.canonical_revisions (entity_type, entity_id, revision)
		VALUES ($1, $2, 1)
		ON CONFLICT (entity_type, entity_id)
		DO UPDATE SET revision = bookdb.canonical_revisions.revision + 1, updated_at = now()
		RETURNING revision`,
		entityType, entityID).Scan(&revision)
	if err != nil {
		return 0, fmt.Errorf("promotion: bump revision: %w", err)
	}
	return revision, nil
}

func (p *Promoter) revision(ctx context.Context, entityType, entityID string) (int64, error) {
	var revision int64
	err := p.db.QueryRowContext(ctx, `
		SELECT revision FROM bookdb.canonical_revisions
		WHERE entity_type = $1 AND entity_id = $2`,
		entityType, entityID).Scan(&revision)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return revision, nil
}

// contentHash returns the stored content hash for a source record.
func (p *Promoter) contentHash(ctx context.Context, sourceName, sourceKey string) (string, error) {
	var hash sql.NullString
	err := p.db.QueryRowContext(ctx, `
		SELECT content_hash FROM bookdb.source_records
		WHERE source_name = $1 AND source_key = $2`,
		sourceName, sourceKey).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("promotion: content hash: %w", err)
	}
	return hash.String, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func firstISBN(isbns []string) string {
	for _, s := range isbns {
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return ""
}

func nullOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parsePublishDate(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// OL dates are imprecise ("1965", "1990-03", "1965-08-15"). Parse the
	// most precise form available; store the date portion only.
	layouts := []string{"2006-01-02", "2006-01", "2006"}
	for _, layout := range layouts {
		n := len(layout)
		if len(s) >= n {
			if t, err := time.Parse(layout, s[:n]); err == nil {
				return t
			}
		}
	}
	return nil
}

// sortNameFor builds a "Last, First" sort name from a display name when the
// name has two or more parts; otherwise it returns empty.
func sortNameFor(displayName string) string {
	parts := strings.Fields(displayName)
	if len(parts) < 2 {
		return ""
	}
	last := parts[len(parts)-1]
	first := strings.Join(parts[:len(parts)-1], " ")
	return last + ", " + first
}

// workKeyFromRecord extracts the Open Library work key an edition references.
// The ingestion Record does not retain the works[] array, so this reads it
// from the raw JSON when present.
func workKeyFromRecord(rec *Record) string {
	if len(rec.RawJSON) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(rec.RawJSON, &m); err != nil {
		return ""
	}
	works, ok := m["works"].([]any)
	if !ok || len(works) == 0 {
		return ""
	}
	wm, ok := works[0].(map[string]any)
	if !ok {
		return ""
	}
	k, _ := wm["key"].(string)
	return strings.TrimPrefix(k, "/works/")
}
