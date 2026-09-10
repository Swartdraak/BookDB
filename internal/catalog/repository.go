// Package catalog implements the S1 canonical catalog: typed persistence for
// works, expressions, editions, people, organizations, credits, identifiers
// and edition contents, plus a namespace-aware identifier resolver.
//
// PostgreSQL owns canonical identity and publication. Source records are
// evidence. Unknown metadata stays unknown; no invented authors, ISBNs or
// dates are introduced.
package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Work is a canonical intellectual creation. Titles are labels, not
// identifiers; the stable UUID is the identity.
type Work struct {
	WorkID          string    `json:"work_id"`
	CanonicalTitle  string    `json:"canonical_title"`
	NormalizedTitle string    `json:"normalized_title"`
	LanguageCode    *string   `json:"language_code,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Expression is a realization of a work: a translation, revision, abridgment
// or a distinct performance. Same title/language does not establish identity.
type Expression struct {
	ExpressionID    string    `json:"expression_id"`
	WorkID          string    `json:"work_id"`
	LanguageCode    string    `json:"language_code"`
	ExpressionTitle string    `json:"expression_title"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Edition is a published manifestation carrying one or more expressions.
// ISBN is not required and is not the identity authority.
type Edition struct {
	EditionID       string     `json:"edition_id"`
	ExpressionID    string     `json:"expression_id"`
	EditionTitle    string     `json:"edition_title"`
	Format          *string    `json:"format,omitempty"`
	ISBN13          *string    `json:"isbn13,omitempty"`
	PublicationDate *time.Time `json:"publication_date,omitempty"`
	PublisherName   *string    `json:"publisher_name,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Person is a human contributor identity. Display name is not unique.
type Person struct {
	PersonID    string    `json:"person_id"`
	DisplayName string    `json:"display_name"`
	SortName    *string   `json:"sort_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Organization is a publisher, imprint, studio or other credited organization.
type Organization struct {
	OrganizationID   string    `json:"organization_id"`
	DisplayName      string    `json:"display_name"`
	SortName         *string   `json:"sort_name,omitempty"`
	OrganizationKind *string   `json:"organization_kind,omitempty"`
	ParentID         *string   `json:"parent_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Credit links a person or organization to a target with a controlled role.
type Credit struct {
	CreditID       string  `json:"credit_id"`
	PersonID       *string `json:"person_id,omitempty"`
	OrganizationID *string `json:"organization_id,omitempty"`
	Role           string  `json:"role"`
	TargetType     string  `json:"target_type"`
	TargetID       string  `json:"target_id"`
	DisplayOrder   int     `json:"display_order"`
	CreditedAs     *string `json:"credited_as,omitempty"`
}

// Identifier is a namespaced identifier assertion. The lookup index is
// conflict-capable: multiple claims may exist for the same normalized value.
type Identifier struct {
	IdentifierID    string  `json:"identifier_id"`
	Namespace       string  `json:"namespace"`
	NormalizedValue string  `json:"normalized_value"`
	RawValue        *string `json:"raw_value,omitempty"`
	TargetType      string  `json:"target_type"`
	TargetID        string  `json:"target_id"`
	Status          string  `json:"status"`
}

// EditionContent is an ordered junction between an edition and an expression.
type EditionContent struct {
	ContentID    string `json:"content_id"`
	EditionID    string `json:"edition_id"`
	ExpressionID string `json:"expression_id"`
	Position     int    `json:"position"`
}

// Repository is the persistence surface for the canonical catalog.
type Repository interface {
	GetWork(ctx context.Context, workID string) (Work, error)
	GetExpression(ctx context.Context, expressionID string) (Expression, error)
	GetEdition(ctx context.Context, editionID string) (Edition, error)
	GetPerson(ctx context.Context, personID string) (Person, error)
	GetOrganization(ctx context.Context, organizationID string) (Organization, error)
	ListEditionsForWork(ctx context.Context, workID string) ([]Edition, error)
	ListWorksForPerson(ctx context.Context, personID string) ([]Work, error)
	ListCreditsForTarget(ctx context.Context, targetType, targetID string) ([]Credit, error)
	ListIdentifiersForTarget(ctx context.Context, targetType, targetID string) ([]Identifier, error)
	Resolve(ctx context.Context, namespace, value string) (Resolution, error)
}

// ErrNotFound is returned when a canonical entity does not exist.
var ErrNotFound = fmt.Errorf("catalog: not found")

// ErrAmbiguous is returned when a resolve request matches multiple distinct
// canonical targets.
var ErrAmbiguous = fmt.Errorf("catalog: ambiguous")

// Resolution is the outcome of a namespace-aware identifier lookup.
type Resolution struct {
	Status     string   `json:"status"` // resolved | ambiguous | not_found
	Candidates []Target `json:"candidates,omitempty"`
}

// Target is a single candidate in a resolution.
type Target struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Status     string `json:"status"`
}

// SQLRepository is a Repository backed by a *sql.DB.
type SQLRepository struct {
	db *sql.DB
}

// NewSQLRepository returns a Repository over the given database handle.
func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) GetWork(ctx context.Context, workID string) (Work, error) {
	var w Work
	var lang sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT work_id, canonical_title, normalized_title, language_code, created_at, updated_at
		FROM bookdb.works WHERE work_id = $1`, workID).
		Scan(&w.WorkID, &w.CanonicalTitle, &w.NormalizedTitle, &lang, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return Work{}, ErrNotFound
	}
	if err != nil {
		return Work{}, fmt.Errorf("catalog: get work: %w", err)
	}
	if lang.Valid {
		w.LanguageCode = &lang.String
	}
	return w, nil
}

func (r *SQLRepository) GetExpression(ctx context.Context, expressionID string) (Expression, error) {
	var e Expression
	err := r.db.QueryRowContext(ctx, `
		SELECT expression_id, work_id, language_code, expression_title, created_at, updated_at
		FROM bookdb.expressions WHERE expression_id = $1`, expressionID).
		Scan(&e.ExpressionID, &e.WorkID, &e.LanguageCode, &e.ExpressionTitle, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return Expression{}, ErrNotFound
	}
	if err != nil {
		return Expression{}, fmt.Errorf("catalog: get expression: %w", err)
	}
	return e, nil
}

func (r *SQLRepository) GetEdition(ctx context.Context, editionID string) (Edition, error) {
	var e Edition
	var format, isbn, publisher sql.NullString
	var pubDate sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT edition_id, expression_id, edition_title, format, isbn13, publication_date, publisher_name, created_at, updated_at
		FROM bookdb.editions WHERE edition_id = $1`, editionID).
		Scan(&e.EditionID, &e.ExpressionID, &e.EditionTitle, &format, &isbn, &pubDate, &publisher, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return Edition{}, ErrNotFound
	}
	if err != nil {
		return Edition{}, fmt.Errorf("catalog: get edition: %w", err)
	}
	if format.Valid {
		e.Format = &format.String
	}
	if isbn.Valid {
		e.ISBN13 = &isbn.String
	}
	if publisher.Valid {
		e.PublisherName = &publisher.String
	}
	if pubDate.Valid {
		e.PublicationDate = &pubDate.Time
	}
	return e, nil
}

func (r *SQLRepository) GetPerson(ctx context.Context, personID string) (Person, error) {
	var p Person
	var sortName sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT person_id, display_name, sort_name, created_at, updated_at
		FROM bookdb.people WHERE person_id = $1`, personID).
		Scan(&p.PersonID, &p.DisplayName, &sortName, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return Person{}, ErrNotFound
	}
	if err != nil {
		return Person{}, fmt.Errorf("catalog: get person: %w", err)
	}
	if sortName.Valid {
		p.SortName = &sortName.String
	}
	return p, nil
}

func (r *SQLRepository) GetOrganization(ctx context.Context, organizationID string) (Organization, error) {
	var o Organization
	var sortName, kind, parent sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT organization_id, display_name, sort_name, organization_kind, parent_id, created_at, updated_at
		FROM bookdb.organizations WHERE organization_id = $1`, organizationID).
		Scan(&o.OrganizationID, &o.DisplayName, &sortName, &kind, &parent, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return Organization{}, ErrNotFound
	}
	if err != nil {
		return Organization{}, fmt.Errorf("catalog: get organization: %w", err)
	}
	if sortName.Valid {
		o.SortName = &sortName.String
	}
	if kind.Valid {
		o.OrganizationKind = &kind.String
	}
	if parent.Valid {
		o.ParentID = &parent.String
	}
	return o, nil
}

func (r *SQLRepository) ListEditionsForWork(ctx context.Context, workID string) ([]Edition, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.edition_id, e.expression_id, e.edition_title, e.format, e.isbn13, e.publication_date, e.publisher_name, e.created_at, e.updated_at
		FROM bookdb.editions e
		JOIN bookdb.expressions x ON x.expression_id = e.expression_id
		WHERE x.work_id = $1
		ORDER BY e.edition_id`, workID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list editions for work: %w", err)
	}
	defer rows.Close()

	editions := make([]Edition, 0)
	for rows.Next() {
		var e Edition
		var format, isbn, publisher sql.NullString
		var pubDate sql.NullTime
		if err := rows.Scan(&e.EditionID, &e.ExpressionID, &e.EditionTitle, &format, &isbn, &pubDate, &publisher, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("catalog: scan edition: %w", err)
		}
		if format.Valid {
			e.Format = &format.String
		}
		if isbn.Valid {
			e.ISBN13 = &isbn.String
		}
		if publisher.Valid {
			e.PublisherName = &publisher.String
		}
		if pubDate.Valid {
			e.PublicationDate = &pubDate.Time
		}
		editions = append(editions, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: editions rows: %w", err)
	}
	return editions, nil
}

func (r *SQLRepository) ListWorksForPerson(ctx context.Context, personID string) ([]Work, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT w.work_id, w.canonical_title, w.normalized_title, w.language_code, w.created_at, w.updated_at
		FROM bookdb.works w
		JOIN bookdb.credits c ON c.target_type = 'work' AND c.target_id = w.work_id
		WHERE c.person_id = $1
		ORDER BY w.work_id`, personID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list works for person: %w", err)
	}
	defer rows.Close()

	works := make([]Work, 0)
	for rows.Next() {
		var w Work
		var lang sql.NullString
		if err := rows.Scan(&w.WorkID, &w.CanonicalTitle, &w.NormalizedTitle, &lang, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("catalog: scan work: %w", err)
		}
		if lang.Valid {
			w.LanguageCode = &lang.String
		}
		works = append(works, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: works rows: %w", err)
	}
	return works, nil
}

func (r *SQLRepository) ListCreditsForTarget(ctx context.Context, targetType, targetID string) ([]Credit, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT credit_id, person_id, organization_id, role, target_type, target_id, display_order, credited_as
		FROM bookdb.credits
		WHERE target_type = $1 AND target_id = $2
		ORDER BY display_order, credit_id`, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list credits: %w", err)
	}
	defer rows.Close()

	credits := make([]Credit, 0)
	for rows.Next() {
		var c Credit
		var person, org, creditedAs sql.NullString
		if err := rows.Scan(&c.CreditID, &person, &org, &c.Role, &c.TargetType, &c.TargetID, &c.DisplayOrder, &creditedAs); err != nil {
			return nil, fmt.Errorf("catalog: scan credit: %w", err)
		}
		if person.Valid {
			c.PersonID = &person.String
		}
		if org.Valid {
			c.OrganizationID = &org.String
		}
		if creditedAs.Valid {
			c.CreditedAs = &creditedAs.String
		}
		credits = append(credits, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: credits rows: %w", err)
	}
	return credits, nil
}

func (r *SQLRepository) ListIdentifiersForTarget(ctx context.Context, targetType, targetID string) ([]Identifier, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT identifier_id, namespace, normalized_value, raw_value, target_type, target_id, status
		FROM bookdb.identifiers
		WHERE target_type = $1 AND target_id = $2
		ORDER BY namespace, normalized_value`, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list identifiers: %w", err)
	}
	defer rows.Close()

	ids := make([]Identifier, 0)
	for rows.Next() {
		var i Identifier
		var raw sql.NullString
		if err := rows.Scan(&i.IdentifierID, &i.Namespace, &i.NormalizedValue, &raw, &i.TargetType, &i.TargetID, &i.Status); err != nil {
			return nil, fmt.Errorf("catalog: scan identifier: %w", err)
		}
		if raw.Valid {
			i.RawValue = &raw.String
		}
		ids = append(ids, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: identifiers rows: %w", err)
	}
	return ids, nil
}

// Resolve performs a namespace-aware identifier lookup. It returns:
//   - Status "resolved" with a single candidate when exactly one distinct
//     canonical target matches.
//   - Status "ambiguous" with all candidates when multiple distinct targets
//     match (records are never merged to satisfy a lookup).
//   - Status "not_found" when nothing matches.
func (r *SQLRepository) Resolve(ctx context.Context, namespace, value string) (Resolution, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT target_type, target_id, status
		FROM bookdb.identifiers
		WHERE namespace = $1 AND normalized_value = $2
		ORDER BY target_type, target_id`, namespace, value)
	if err != nil {
		return Resolution{}, fmt.Errorf("catalog: resolve: %w", err)
	}
	defer rows.Close()

	candidates := make([]Target, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.TargetType, &t.TargetID, &t.Status); err != nil {
			return Resolution{}, fmt.Errorf("catalog: scan resolution: %w", err)
		}
		key := t.TargetType + ":" + t.TargetID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, t)
	}
	if err := rows.Err(); err != nil {
		return Resolution{}, fmt.Errorf("catalog: resolution rows: %w", err)
	}

	switch len(candidates) {
	case 0:
		return Resolution{Status: "not_found"}, nil
	case 1:
		return Resolution{Status: "resolved", Candidates: candidates}, nil
	default:
		return Resolution{Status: "ambiguous", Candidates: candidates}, nil
	}
}
