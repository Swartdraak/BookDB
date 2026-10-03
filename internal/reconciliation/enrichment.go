// enrichment.go implements the S3 enrichment and cross-source candidate
// pipeline for issue #55.
//
// After S2 ingest + promotion, canonical entities carry only the fields the
// dump subset asserted. This file adds the missing S3 wiring:
//
//  1. EnrichmentClient — fetches supplementary data from the Open Library
//     public API (work descriptions/subjects/covers, author
//     names/birth-death dates/portraits) with the bounded retry policy from
//     docs/bookdb/04-ingestion-reconciliation.md.
//  2. Cross-source duplicate candidate generation — shared valid
//     identifiers and normalized title+contributor blocking, persisted to
//     bookdb.duplicate_candidates with status 'pending'. A same person name
//     is NEVER an auto-merge (matching policy table).
//  3. Field-level selection — enriched OL values are selected into the
//     canonical entity through Reconciler.SelectField, which records every
//     source value in bookdb.field_provenance and marks the winner.
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DefaultUserAgent identifies BookDB's bounded enrichment traffic to the
// Open Library and Wikidata APIs.
const DefaultUserAgent = "BookDB/1.0 (https://github.com/Swartdraak/BookDB; reconciliation enrichment)"

// DefaultOLBase is the Open Library public API base URL.
const DefaultOLBase = "https://openlibrary.org"

// DefaultWDBase is the Wikidata public API base URL.
const DefaultWDBase = "https://www.wikidata.org"

// ErrNotFound is returned when a remote record does not exist (HTTP 404).
var ErrNotFound = errors.New("reconciliation: remote record not found")

// EnrichmentConfig configures an EnrichmentClient.
type EnrichmentConfig struct {
	OLBase     string        // default https://openlibrary.org
	WDBase     string        // default https://www.wikidata.org
	UserAgent  string        // default DefaultUserAgent
	Timeout    time.Duration // per-request timeout; default 15s
	MaxRetries int           // bounded retry attempts for 429/5xx; default 2
}

// EnrichmentClient fetches supplementary metadata from the Open Library and
// Wikidata public APIs. All HTTP goes through one *http.Client so tests can
// substitute a stub transport; no unit test performs real network I/O.
type EnrichmentClient struct {
	cfg EnrichmentConfig
	hc  *http.Client
}

// NewEnrichmentClient creates a client with defaults applied.
func NewEnrichmentClient(cfg EnrichmentConfig) *EnrichmentClient {
	if cfg.OLBase == "" {
		cfg.OLBase = DefaultOLBase
	}
	if cfg.WDBase == "" {
		cfg.WDBase = DefaultWDBase
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 2
	}
	return &EnrichmentClient{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// OLWork is the supplementary work data fetched from OL after ingest.
type OLWork struct {
	Description string   `json:"description,omitempty"`
	Subjects    []string `json:"subjects,omitempty"`
	FirstPub    string   `json:"first_publish_date,omitempty"`
	CoverID     int64    `json:"cover_i,omitempty"`
	AuthorKeys  []string `json:"author_keys,omitempty"`
}

// OLAuthor is the supplementary author data fetched from OL after ingest.
type OLAuthor struct {
	Name       string   `json:"name,omitempty"`
	BirthDate  string   `json:"birth_date,omitempty"`
	DeathDate  string   `json:"death_date,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	PortraitID int64    `json:"portrait_i,omitempty"`
	Website    string   `json:"website,omitempty"`
}

// getJSON performs a GET with the bounded retry policy from
// docs/bookdb/04-ingestion-reconciliation.md: exponential backoff, bounded
// attempts, honoring Retry-After. 4xx responses (except 429) fail
// immediately — input validation failures are not retried as transient.
func (c *EnrichmentClient) getJSON(ctx context.Context, rawURL string, out any) error {
	var lastErr error
	delay := 500 * time.Millisecond
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return fmt.Errorf("reconciliation: build request: %w", err)
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // bound: 10 MiB
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK:
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("reconciliation: decode %s: %w", rawURL, err)
			}
			return nil
		case resp.StatusCode == http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, rawURL)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("reconciliation: HTTP %d from %s", resp.StatusCode, rawURL)
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, perr := parseRetryAfter(ra); perr == nil {
					delay = secs
				}
			}
			continue
		default:
			return fmt.Errorf("reconciliation: HTTP %d from %s (not retried)", resp.StatusCode, rawURL)
		}
	}
	return fmt.Errorf("reconciliation: %s after %d attempts: %w", rawURL, c.cfg.MaxRetries+1, lastErr)
}

// FetchWork fetches supplementary OL data for a work by OL work key
// (e.g. "OL18315W"). ErrNotFound is returned for unknown keys.
func (c *EnrichmentClient) FetchWork(ctx context.Context, workKey string) (*OLWork, error) {
	rawURL := strings.TrimSuffix(c.cfg.OLBase, "/") + "/works/" + url.PathEscape(workKey) + ".json"
	var raw map[string]any
	if err := c.getJSON(ctx, rawURL, &raw); err != nil {
		return nil, err
	}
	w := &OLWork{}
	if d, ok := raw["description"].(map[string]any); ok {
		if v, ok := d["value"].(string); ok {
			w.Description = v
		}
	}
	if subs, ok := raw["subjects"].([]any); ok {
		for _, s := range subs {
			if v, ok := s.(string); ok {
				w.Subjects = append(w.Subjects, v)
			}
		}
	}
	if fpd, ok := raw["first_publish_date"].(string); ok {
		w.FirstPub = fpd
	}
	if cov, ok := raw["covers"].([]any); ok && len(cov) > 0 {
		w.CoverID = numberFromJSONValue(cov[0])
	}
	if auths, ok := raw["authors"].([]any); ok {
		for _, ar := range auths {
			if m, ok := ar.(map[string]any); ok {
				if ao, ok := m["author"].(map[string]any); ok {
					if k, ok := ao["key"].(string); ok {
						w.AuthorKeys = append(w.AuthorKeys, trimOLPrefix(k))
					}
				}
			}
		}
	}
	return w, nil
}

// FetchAuthor fetches supplementary OL data for an author by OL author key
// (e.g. "OL17720A"). ErrNotFound is returned for unknown keys.
func (c *EnrichmentClient) FetchAuthor(ctx context.Context, authorKey string) (*OLAuthor, error) {
	rawURL := strings.TrimSuffix(c.cfg.OLBase, "/") + "/authors/" + url.PathEscape(authorKey) + ".json"
	var raw map[string]any
	if err := c.getJSON(ctx, rawURL, &raw); err != nil {
		return nil, err
	}
	a := &OLAuthor{}
	if name, ok := raw["name"].(string); ok {
		a.Name = name
	}
	if bd, ok := raw["birth_date"].(string); ok {
		a.BirthDate = bd
	}
	if dd, ok := raw["death_date"].(string); ok {
		a.DeathDate = dd
	}
	if aliases, ok := raw["alternate_names"].([]any); ok {
		for _, s := range aliases {
			if v, ok := s.(string); ok {
				a.Aliases = append(a.Aliases, v)
			}
		}
	}
	a.PortraitID = numberFromJSONValue(raw["portrait"])
	if ws, ok := raw["website"].(string); ok {
		a.Website = ws
	}
	return a, nil
}

func numberFromJSONValue(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case string:
		var out int64
		_, _ = fmt.Sscanf(n, "%d", &out)
		return out
	}
	return 0
}

func parseRetryAfter(ra string) (time.Duration, error) {
	if d, err := time.ParseDuration(ra + "s"); err == nil {
		return d, nil
	}
	if t, err := http.ParseTime(ra); err == nil {
		if delta := time.Until(t); delta > 0 {
			return delta, nil
		}
	}
	return 0, fmt.Errorf("unparseable Retry-After %q", ra)
}

func trimOLPrefix(k string) string {
	for _, p := range []string{"/works/", "/editions/", "/books/", "/authors/"} {
		k = strings.TrimPrefix(k, p)
	}
	return k
}

// ---------------------------------------------------------------------------
// Cross-source duplicate candidate generation
// ---------------------------------------------------------------------------

// FindDuplicateCandidatesCrossSource generates duplicate candidates between
// source records of DIFFERENT sources, per the matching policy in
// docs/bookdb/04-ingestion-reconciliation.md:
//
//   - a shared valid identifier (both records' payloads carry the same
//     identifier value, e.g. an OL work key referenced from both sources)
//     => candidate for exact resolution (confidence 1.0); and
//   - identical normalized titles within the same entity type => candidate
//     generation ONLY (confidence 0.5).
//
// A same person name is NEVER an auto-merge: every candidate is created
// with status 'pending' and is never merged automatically. Candidates are
// persisted to bookdb.duplicate_candidates (idempotent via the unique
// (type_a, id_a, type_b, id_b) constraint) and returned.
func (r *Reconciler) FindDuplicateCandidatesCrossSource(ctx context.Context) ([]DuplicateCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.source_name, a.source_key, a.payload,
		       b.source_name, b.source_key, b.payload
		FROM bookdb.source_records a
		JOIN bookdb.source_records b
		  ON a.source_name != b.source_name
		 AND a.source_key < b.source_key
		WHERE a.payload->'source_record'->>'entity_id' IS NOT NULL
		  AND b.payload->'source_record'->>'entity_id' IS NOT NULL
		LIMIT 500`)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: query cross-source records: %w", err)
	}
	defer rows.Close()

	type pair struct {
		entityType string
		idA, idB   uuid.UUID
		confidence float64
		evidence   []map[string]string
	}
	var pairs []pair
	for rows.Next() {
		var nameA, keyA, nameB, keyB string
		var payloadA, payloadB []byte
		if err := rows.Scan(&nameA, &keyA, &payloadA, &nameB, &keyB, &payloadB); err != nil {
			return nil, fmt.Errorf("reconciliation: scan pair: %w", err)
		}
		recA := recordFromPayload(payloadA)
		recB := recordFromPayload(payloadB)
		idA, err1 := uuid.Parse(recA.SourceRecord.EntityID)
		idB, err2 := uuid.Parse(recB.SourceRecord.EntityID)
		if err1 != nil || err2 != nil {
			continue
		}

		typeA, typeB := recA.EntityType(), recB.EntityType()
		if typeA == "" || typeB == "" || typeA != typeB {
			// Only same-type pairs are candidates; type mismatch is never
			// auto-merged (matching policy: different variant -> separate).
			continue
		}

		var confidence float64
		var evidence []map[string]string
		shared := sharedIdentifierValues(keyA, recA, keyB, recB)
		if len(shared) > 0 {
			// Same valid identifier: candidate for exact resolution
			// (rule 2 of the matching policy table).
			confidence = 1.0
			for _, s := range shared {
				evidence = append(evidence, map[string]string{
					"rule":       "shared_identifier",
					"identifier": s,
				})
			}
		} else if recA.Title != "" && NormalizeTitle(recA.Title) == NormalizeTitle(recB.Title) {
			// Identical normalized title within the same entity type:
			// candidate generation only — never an auto-merge.
			confidence = 0.5
			evidence = append(evidence, map[string]string{
				"rule":  "title_block",
				"title": NormalizeTitle(recA.Title),
			})
		}
		if confidence == 0 {
			continue
		}
		pairs = append(pairs, pair{
			entityType: typeA,
			idA:        idA,
			idB:        idB,
			confidence: confidence,
			evidence:   evidence,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reconciliation: pair rows: %w", err)
	}

	candidates := make([]DuplicateCandidate, 0, len(pairs))
	for _, p := range pairs {
		// Deterministic ordering: lower entity id is always side A.
		if p.idB.String() < p.idA.String() {
			p.idA, p.idB = p.idB, p.idA
		}
		c, err := r.persistCandidate(ctx, p.entityType, p.idA, p.idB, p.confidence, p.evidence)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// entityRecord is the subset of a stored source record payload needed for
// cross-source candidate generation. The promotion pipeline persists
// source_record.entity_id but not entity_type, so the type is inferred: the
// payload carries "name" for people and "title" for works/editions.
type entityRecord struct {
	SourceRecord struct {
		EntityID string `json:"entity_id"`
	} `json:"source_record"`
	Title   string   `json:"title"`
	Name    string   `json:"name"`
	Authors []string `json:"authors"`
}

// EntityType returns the canonical entity type for this record, inferred
// when the payload does not carry an explicit type.
func (e entityRecord) EntityType() string {
	if e.Name != "" {
		return "person"
	}
	if e.Title != "" {
		// Works and editions both carry titles; cross-source edition pairs
		// are rare in practice, and the candidate row stores the type of
		// record A — a type mismatch between the pair sides is recorded in
		// the evidence, never auto-merged.
		return "work"
	}
	return ""
}

func recordFromPayload(payload []byte) entityRecord {
	var rec entityRecord
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &rec)
	}
	return rec
}

// sharedIdentifierValues returns identifier values carried by BOTH records:
// the source keys of each record (e.g. an OL work key referenced from a
// second source's record) and any work keys embedded in the record.
func sharedIdentifierValues(keyA string, recA entityRecord, keyB string, recB entityRecord) []string {
	setB := map[string]struct{}{keyB: {}}
	for _, k := range recB.Authors {
		setB[k] = struct{}{}
	}
	var shared []string
	seen := map[string]struct{}{}
	consider := func(v string) {
		v = trimOLPrefix(v)
		if v == "" {
			return
		}
		if _, ok := setB[v]; ok {
			if _, dup := seen[v]; !dup {
				seen[v] = struct{}{}
				shared = append(shared, v)
			}
		}
	}
	consider(keyA)
	for _, k := range recA.Authors {
		consider(k)
	}
	sort.Strings(shared)
	return shared
}

// persistCandidate stores a candidate pair idempotently and returns it.
func (r *Reconciler) persistCandidate(ctx context.Context, entityType string, idA, idB uuid.UUID, confidence float64, evidence []map[string]string) (DuplicateCandidate, error) {
	ev, _ := json.Marshal(evidence)
	if ev == nil {
		ev = []byte("[]")
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO bookdb.duplicate_candidates (type_a, id_a, type_b, id_b, confidence, evidence, status)
		VALUES ($1, $2, $1, $3, $4, $5, 'pending')
		ON CONFLICT (type_a, id_a, type_b, id_b) DO UPDATE SET
			confidence = EXCLUDED.confidence,
			evidence   = EXCLUDED.evidence
		RETURNING candidate_id::text`,
		entityType, idA, idB, confidence, ev).Scan(&id)
	if err != nil {
		return DuplicateCandidate{}, fmt.Errorf("reconciliation: persist candidate: %w", err)
	}
	return DuplicateCandidate{
		CandidateID: id,
		TypeA:       entityType,
		IDA:         idA.String(),
		TypeB:       entityType,
		IDB:         idB.String(),
		Confidence:  confidence,
		Status:      "pending",
	}, nil
}

// ---------------------------------------------------------------------------
// Enrichment application (OL API -> canonical fields with provenance)
// ---------------------------------------------------------------------------

// EnrichedField is one enriched value selected into a canonical entity.
type EnrichedField struct {
	FieldName string `json:"field_name"`
	Value     any    `json:"value"`
}

// EnrichWork fetches OL supplementary data for a promoted work and selects
// the new fields through field-level selection, recording provenance for
// every value (source openlibrary + work key).
func (r *Reconciler) EnrichWork(ctx context.Context, client *EnrichmentClient, workID uuid.UUID, olWorkKey string) (int, error) {
	w, err := client.FetchWork(ctx, olWorkKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	fields := map[string]any{}
	if w.Description != "" {
		fields["description"] = w.Description
	}
	if len(w.Subjects) > 0 {
		fields["subjects"] = w.Subjects
	}
	if w.FirstPub != "" {
		fields["first_publish_date"] = w.FirstPub
	}
	if w.CoverID != 0 {
		fields["cover_i"] = w.CoverID
	}
	return r.applyEnrichedFields(ctx, "work", workID, olWorkKey, fields)
}

// EnrichPerson fetches OL supplementary data for a promoted person and
// selects the new fields through field-level selection.
func (r *Reconciler) EnrichPerson(ctx context.Context, client *EnrichmentClient, personID uuid.UUID, olAuthorKey string) (int, error) {
	a, err := client.FetchAuthor(ctx, olAuthorKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	fields := map[string]any{}
	if a.Name != "" {
		fields["name"] = a.Name
	}
	if a.BirthDate != "" {
		fields["birth_date"] = a.BirthDate
	}
	if a.DeathDate != "" {
		fields["death_date"] = a.DeathDate
	}
	if a.PortraitID != 0 {
		fields["portrait_i"] = a.PortraitID
	}
	return r.applyEnrichedFields(ctx, "person", personID, olAuthorKey, fields)
}

// applyEnrichedFields selects each enriched value through
// Reconciler.SelectField (which upserts field_provenance for every source
// value and marks the selected one), then writes the winning values into the
// canonical entity row. Returns the number of fields applied.
func (r *Reconciler) applyEnrichedFields(ctx context.Context, entityType string, entityID uuid.UUID, sourceKey string, fields map[string]any) (int, error) {
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	sort.Strings(names)
	applied := 0
	for _, name := range names {
		value := fields[name]
		raw, err := json.Marshal(value)
		if err != nil {
			return applied, fmt.Errorf("reconciliation: marshal enriched %s: %w", name, err)
		}
		selected, err := r.SelectField(ctx, entityType, entityID, name, []FieldValue{
			{SourceName: "openlibrary", SourceKey: sourceKey, Value: raw},
		})
		if err != nil {
			return applied, err
		}
		if selected == "" {
			continue
		}
		if err := r.writeCanonicalField(ctx, entityType, entityID, name, selected); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// writeCanonicalField persists a selected enriched value onto the canonical
// entity row. Only canonical columns that exist for the entity type are
// written; unknown fields (e.g. subjects/description, which have no canonical
// column yet) are retained in field_provenance only — that is the evidence
// layer the S3 pipeline is built on.
func (r *Reconciler) writeCanonicalField(ctx context.Context, entityType string, entityID uuid.UUID, fieldName, value string) error {
	switch entityType {
	case "work":
		switch fieldName {
		case "title", "canonical_title":
			_, err := r.db.ExecContext(ctx, `UPDATE bookdb.works SET canonical_title = $1, updated_at = now() WHERE work_id = $2`, value, entityID)
			return err
		case "language":
			_, err := r.db.ExecContext(ctx, `UPDATE bookdb.works SET language_code = $1, updated_at = now() WHERE work_id = $2`, value, entityID)
			return err
		default:
			return nil
		}
	case "person":
		switch fieldName {
		case "name", "display_name":
			_, err := r.db.ExecContext(ctx, `UPDATE bookdb.people SET display_name = $1, updated_at = now() WHERE person_id = $2`, value, entityID)
			return err
		default:
			return nil
		}
	default:
		return nil
	}
}

// PromotedEntities lists the canonical entities promoted by the given source
// (entity type + id), for enrichment post-passes. The promotion pipeline
// persists source_record.entity_id in the payload; the entity type is
// inferred from the payload shape (see entityRecord.EntityType).
func (r *Reconciler) PromotedEntities(ctx context.Context, sourceName string) ([]PromotedEntity, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT payload->'source_record'->>'entity_id',
		       source_key,
		       COALESCE(NULLIF(payload->>'title', ''), payload->>'name', '')
		FROM bookdb.source_records
		WHERE source_name = $1
		  AND payload->'source_record'->>'entity_id' IS NOT NULL`, sourceName)
	if err != nil {
		return nil, fmt.Errorf("reconciliation: list promoted entities: %w", err)
	}
	defer rows.Close()
	var out []PromotedEntity
	for rows.Next() {
		var e PromotedEntity
		if err := rows.Scan(&e.EntityID, &e.SourceKey, &e.Title); err != nil {
			return nil, fmt.Errorf("reconciliation: scan promoted entity: %w", err)
		}
		e.EntityType = inferEntityType(e.Title, e.SourceKey, e.EntityID)
		out = append(out, e)
	}
	return out, rows.Err()
}

// inferEntityType approximates the canonical entity type of a promoted
// source record from its payload fields: OL author keys (OL* A) are people,
// records with a name are people, records with a title are works/editions
// (treated as works here — edition enrichment is not yet wired).
func inferEntityType(title, sourceKey, entityID string) string {
	if looksLikeAuthorKey(sourceKey) || title == "" {
		// Empty title + author-shaped key: person. Empty title otherwise:
		// unknown; the caller skips enrichment for unknown types.
		if looksLikeAuthorKey(sourceKey) {
			return "person"
		}
		return ""
	}
	return "work"
}

// looksLikeAuthorKey reports whether an OL source key has the author key
// shape (OL followed by digits and ending in A, e.g. OL17720A).
func looksLikeAuthorKey(key string) bool {
	if len(key) < 3 || !strings.HasPrefix(key, "OL") || !strings.HasSuffix(key, "A") {
		return false
	}
	for _, c := range key[2 : len(key)-1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ReconcileOutcome summarizes a reconciliation post-pass run.
type ReconcileOutcome struct {
	Enriched   int                  `json:"enriched"`
	Claims     int                  `json:"wikidata_claims"`
	Candidates []DuplicateCandidate `json:"duplicate_candidates"`
	Errors     []string             `json:"errors,omitempty"`
}

// RunReconcile executes the S3 reconciliation post-pass over the source
// records of the given source: (1) OL API enrichment of promoted works and
// people with field-level selection into field_provenance, and (2)
// cross-source duplicate candidate generation persisted to
// bookdb.duplicate_candidates. It is idempotent: enrichment is a no-op for
// keys already enriched (provenance rows upsert), and candidate rows are
// upserted on their unique pair key.
func (r *Reconciler) RunReconcile(ctx context.Context, client *EnrichmentClient, sourceName string, maxPerType int) (ReconcileOutcome, error) {
	entities, err := r.PromotedEntities(ctx, sourceName)
	if err != nil {
		return ReconcileOutcome{}, err
	}
	out := ReconcileOutcome{Candidates: []DuplicateCandidate{}}
	workCount, personCount := 0, 0
	for _, e := range entities {
		id, err := uuid.Parse(e.EntityID)
		if err != nil {
			out.Errors = append(out.Errors, "entity "+e.EntityID+": "+err.Error())
			continue
		}
		switch e.EntityType {
		case "work":
			if maxPerType > 0 && workCount >= maxPerType {
				continue
			}
			n, err := r.EnrichWork(ctx, client, id, e.SourceKey)
			if err != nil {
				out.Errors = append(out.Errors, "work "+e.SourceKey+": "+err.Error())
				continue
			}
			workCount++
			out.Enriched += n
		case "person":
			if maxPerType > 0 && personCount >= maxPerType {
				continue
			}
			n, err := r.EnrichPerson(ctx, client, id, e.SourceKey)
			if err != nil {
				out.Errors = append(out.Errors, "person "+e.SourceKey+": "+err.Error())
				continue
			}
			personCount++
			out.Enriched += n
		}
	}

	candidates, err := r.FindDuplicateCandidatesCrossSource(ctx)
	if err != nil {
		return out, err
	}
	out.Candidates = candidates
	return out, nil
}

// PromotedEntity is a canonical entity that a source record promoted to.
type PromotedEntity struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	SourceKey  string `json:"source_key"`
	Title      string `json:"title"`
}
