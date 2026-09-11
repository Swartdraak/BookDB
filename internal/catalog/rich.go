package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// SeriesMembership links a work into an ordered series.
type SeriesMembership struct {
	SeriesID     string `json:"series_id"`
	SeriesTitle  string `json:"series_title"`
	SeriesOrder  *int   `json:"series_order,omitempty"`
	SeriesWorkID string `json:"work_id"`
}

// AudioPerformance captures narrator/producer metadata for audio expressions.
type AudioPerformance struct {
	PerformanceID string  `json:"performance_id"`
	ExpressionID  string  `json:"expression_id"`
	NarratorID    *string `json:"narrator_id,omitempty"`
	NarratorName  *string `json:"narrator_name,omitempty"`
	ProducerID    *string `json:"producer_id,omitempty"`
	ProducerName  *string `json:"producer_name,omitempty"`
	IsAbridged    bool    `json:"is_abridged"`
	DurationSec   *int    `json:"duration_sec,omitempty"`
	SampleURL     *string `json:"sample_url,omitempty"`
}

// Asset is a catalog-linked media asset such as a cover or portrait.
type Asset struct {
	AssetID     string    `json:"asset_id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	AssetKind   string    `json:"asset_kind"`
	URL         string    `json:"url"`
	Eligibility string    `json:"eligibility"`
	SourceName  *string   `json:"source_name,omitempty"`
	SourceKey   *string   `json:"source_key,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// RichEdition augments an edition with accessibility and expression credits.
type RichEdition struct {
	Edition
	Accessibility []string `json:"accessibility"`
	Credits       []Credit `json:"credits"`
}

// RichWork bundles S5 data needed for rich work detail and comparison flows.
type RichWork struct {
	Work             Work               `json:"work"`
	SeriesMembership []SeriesMembership `json:"series_membership"`
	Editions         []RichEdition      `json:"editions"`
	Audio            []AudioPerformance `json:"audio_performances"`
	Assets           []Asset            `json:"assets"`
}

// EditionComparison highlights notable differences between two editions.
type EditionComparison struct {
	Left        RichEdition `json:"left"`
	Right       RichEdition `json:"right"`
	SameWork    bool        `json:"same_work"`
	Differences []string    `json:"differences"`
}

// QualityCoverageByFormatLanguage reports known-field coverage by format/language.
type QualityCoverageByFormatLanguage struct {
	Format           string `json:"format"`
	LanguageCode     string `json:"language_code"`
	Editions         int    `json:"editions"`
	KnownISBN        int    `json:"known_isbn"`
	KnownPublication int    `json:"known_publication_date"`
	KnownPublisher   int    `json:"known_publisher"`
}

// QualityCoverageBySource reports evidence coverage by source.
type QualityCoverageBySource struct {
	SourceName string `json:"source_name"`
	Records    int    `json:"records"`
}

// QualityReport is a factual quality dashboard payload.
type QualityReport struct {
	ByFormatLanguage []QualityCoverageByFormatLanguage `json:"by_format_language"`
	BySource         []QualityCoverageBySource         `json:"by_source"`
}

func (r *SQLRepository) GetRichWork(ctx context.Context, workID string) (RichWork, error) {
	work, err := r.GetWork(ctx, workID)
	if err != nil {
		return RichWork{}, err
	}

	editions, err := r.ListEditionsForWork(ctx, workID)
	if err != nil {
		return RichWork{}, fmt.Errorf("catalog: rich work editions: %w", err)
	}

	series, err := r.listSeriesMembership(ctx, workID)
	if err != nil {
		return RichWork{}, err
	}
	performances, err := r.listAudioPerformances(ctx, workID)
	if err != nil {
		return RichWork{}, err
	}
	assets, err := r.listAssets(ctx, "work", workID, true)
	if err != nil {
		return RichWork{}, err
	}

	editionDetails := make([]RichEdition, 0, len(editions))
	for _, ed := range editions {
		acc, err := r.listEditionAccessibility(ctx, ed.EditionID)
		if err != nil {
			return RichWork{}, err
		}
		credits, err := r.ListCreditsForTarget(ctx, "expression", ed.ExpressionID)
		if err != nil {
			return RichWork{}, fmt.Errorf("catalog: list expression credits: %w", err)
		}
		editionDetails = append(editionDetails, RichEdition{Edition: ed, Accessibility: acc, Credits: credits})
	}

	return RichWork{
		Work:             work,
		SeriesMembership: series,
		Editions:         editionDetails,
		Audio:            performances,
		Assets:           assets,
	}, nil
}

func (r *SQLRepository) CompareEditions(ctx context.Context, leftID, rightID string) (EditionComparison, error) {
	left, err := r.GetEdition(ctx, leftID)
	if err != nil {
		return EditionComparison{}, err
	}
	right, err := r.GetEdition(ctx, rightID)
	if err != nil {
		return EditionComparison{}, err
	}

	leftAccess, err := r.listEditionAccessibility(ctx, leftID)
	if err != nil {
		return EditionComparison{}, err
	}
	rightAccess, err := r.listEditionAccessibility(ctx, rightID)
	if err != nil {
		return EditionComparison{}, err
	}

	leftCredits, err := r.ListCreditsForTarget(ctx, "expression", left.ExpressionID)
	if err != nil {
		return EditionComparison{}, fmt.Errorf("catalog: left expression credits: %w", err)
	}
	rightCredits, err := r.ListCreditsForTarget(ctx, "expression", right.ExpressionID)
	if err != nil {
		return EditionComparison{}, fmt.Errorf("catalog: right expression credits: %w", err)
	}

	sameWork, err := r.sameWorkForEditions(ctx, leftID, rightID)
	if err != nil {
		return EditionComparison{}, err
	}

	differences := make([]string, 0, 8)
	appendIfDifferent := func(name string, a, b *string) {
		av := "unknown"
		bv := "unknown"
		if a != nil {
			av = *a
		}
		if b != nil {
			bv = *b
		}
		if av != bv {
			differences = append(differences, name)
		}
	}
	appendIfDifferent("format", left.Format, right.Format)
	appendIfDifferent("isbn13", left.ISBN13, right.ISBN13)
	appendIfDifferent("publisher_name", left.PublisherName, right.PublisherName)

	leftDate := "unknown"
	rightDate := "unknown"
	if left.PublicationDate != nil {
		leftDate = left.PublicationDate.Format("2006-01-02")
	}
	if right.PublicationDate != nil {
		rightDate = right.PublicationDate.Format("2006-01-02")
	}
	if leftDate != rightDate {
		differences = append(differences, "publication_date")
	}
	if !equalStringSets(leftAccess, rightAccess) {
		differences = append(differences, "accessibility")
	}

	return EditionComparison{
		Left:        RichEdition{Edition: left, Accessibility: leftAccess, Credits: leftCredits},
		Right:       RichEdition{Edition: right, Accessibility: rightAccess, Credits: rightCredits},
		SameWork:    sameWork,
		Differences: differences,
	}, nil
}

func (r *SQLRepository) QualityReport(ctx context.Context) (QualityReport, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(e.format, 'unknown') AS format,
		       x.language_code,
		       COUNT(*) AS editions,
		       COUNT(e.isbn13) AS known_isbn,
		       COUNT(e.publication_date) AS known_pub,
		       COUNT(e.publisher_name) AS known_publisher
		FROM bookdb.editions e
		JOIN bookdb.expressions x ON x.expression_id = e.expression_id
		GROUP BY COALESCE(e.format, 'unknown'), x.language_code
		ORDER BY format, x.language_code`)
	if err != nil {
		return QualityReport{}, fmt.Errorf("catalog: quality by format/language: %w", err)
	}
	defer rows.Close()

	byFormat := make([]QualityCoverageByFormatLanguage, 0)
	for rows.Next() {
		var item QualityCoverageByFormatLanguage
		if err := rows.Scan(&item.Format, &item.LanguageCode, &item.Editions, &item.KnownISBN, &item.KnownPublication, &item.KnownPublisher); err != nil {
			return QualityReport{}, fmt.Errorf("catalog: scan quality row: %w", err)
		}
		byFormat = append(byFormat, item)
	}
	if err := rows.Err(); err != nil {
		return QualityReport{}, fmt.Errorf("catalog: quality rows: %w", err)
	}

	sourceRows, err := r.db.QueryContext(ctx, `
		SELECT source_name, COUNT(*) AS records
		FROM bookdb.source_records
		GROUP BY source_name
		ORDER BY source_name`)
	if err != nil {
		return QualityReport{}, fmt.Errorf("catalog: quality by source: %w", err)
	}
	defer sourceRows.Close()

	bySource := make([]QualityCoverageBySource, 0)
	for sourceRows.Next() {
		var item QualityCoverageBySource
		if err := sourceRows.Scan(&item.SourceName, &item.Records); err != nil {
			return QualityReport{}, fmt.Errorf("catalog: scan source row: %w", err)
		}
		bySource = append(bySource, item)
	}
	if err := sourceRows.Err(); err != nil {
		return QualityReport{}, fmt.Errorf("catalog: source rows: %w", err)
	}

	return QualityReport{ByFormatLanguage: byFormat, BySource: bySource}, nil
}

func (r *SQLRepository) listSeriesMembership(ctx context.Context, workID string) ([]SeriesMembership, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sm.series_id, s.canonical_title, sm.position, sm.work_id
		FROM bookdb.series_members sm
		JOIN bookdb.series s ON s.series_id = sm.series_id
		WHERE sm.work_id = $1
		ORDER BY sm.position NULLS LAST, sm.series_id`, workID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list series membership: %w", err)
	}
	defer rows.Close()

	items := make([]SeriesMembership, 0)
	for rows.Next() {
		var m SeriesMembership
		var order sql.NullInt64
		if err := rows.Scan(&m.SeriesID, &m.SeriesTitle, &order, &m.SeriesWorkID); err != nil {
			return nil, fmt.Errorf("catalog: scan series membership: %w", err)
		}
		if order.Valid {
			o := int(order.Int64)
			m.SeriesOrder = &o
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: series membership rows: %w", err)
	}
	return items, nil
}

func (r *SQLRepository) listAudioPerformances(ctx context.Context, workID string) ([]AudioPerformance, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ap.performance_id,
		       ap.expression_id,
		       ap.narrator_id,
		       p.display_name,
		       ap.producer_id,
		       o.display_name,
		       ap.is_abridged,
		       ap.duration_sec,
		       ap.sample_url
		FROM bookdb.audio_performances ap
		JOIN bookdb.expressions e ON e.expression_id = ap.expression_id
		LEFT JOIN bookdb.people p ON p.person_id = ap.narrator_id
		LEFT JOIN bookdb.organizations o ON o.organization_id = ap.producer_id
		WHERE e.work_id = $1
		ORDER BY ap.expression_id, ap.performance_id`, workID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list audio performances: %w", err)
	}
	defer rows.Close()

	items := make([]AudioPerformance, 0)
	for rows.Next() {
		var item AudioPerformance
		var narratorID, narratorName, producerID, producerName, sampleURL sql.NullString
		var duration sql.NullInt64
		if err := rows.Scan(&item.PerformanceID, &item.ExpressionID, &narratorID, &narratorName, &producerID, &producerName, &item.IsAbridged, &duration, &sampleURL); err != nil {
			return nil, fmt.Errorf("catalog: scan audio performance: %w", err)
		}
		if narratorID.Valid {
			item.NarratorID = &narratorID.String
		}
		if narratorName.Valid {
			item.NarratorName = &narratorName.String
		}
		if producerID.Valid {
			item.ProducerID = &producerID.String
		}
		if producerName.Valid {
			item.ProducerName = &producerName.String
		}
		if duration.Valid {
			d := int(duration.Int64)
			item.DurationSec = &d
		}
		if sampleURL.Valid {
			item.SampleURL = &sampleURL.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: audio performance rows: %w", err)
	}
	return items, nil
}

func (r *SQLRepository) listEditionAccessibility(ctx context.Context, editionID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT feature
		FROM bookdb.edition_accessibility
		WHERE edition_id = $1
		ORDER BY feature`, editionID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list edition accessibility: %w", err)
	}
	defer rows.Close()
	features := make([]string, 0)
	for rows.Next() {
		var feature string
		if err := rows.Scan(&feature); err != nil {
			return nil, fmt.Errorf("catalog: scan accessibility: %w", err)
		}
		features = append(features, feature)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: accessibility rows: %w", err)
	}
	return features, nil
}

func (r *SQLRepository) listAssets(ctx context.Context, entityType, entityID string, publicOnly bool) ([]Asset, error) {
	query := `
		SELECT asset_id, entity_type, entity_id, asset_kind, url, eligibility, source_name, source_key, created_at
		FROM bookdb.assets
		WHERE entity_type = $1 AND entity_id = $2`
	if publicOnly {
		query += ` AND eligibility = 'public'`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list assets: %w", err)
	}
	defer rows.Close()

	items := make([]Asset, 0)
	for rows.Next() {
		var item Asset
		var sourceName, sourceKey sql.NullString
		if err := rows.Scan(&item.AssetID, &item.EntityType, &item.EntityID, &item.AssetKind, &item.URL, &item.Eligibility, &sourceName, &sourceKey, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("catalog: scan asset: %w", err)
		}
		if sourceName.Valid {
			item.SourceName = &sourceName.String
		}
		if sourceKey.Valid {
			item.SourceKey = &sourceKey.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: asset rows: %w", err)
	}
	return items, nil
}

func (r *SQLRepository) sameWorkForEditions(ctx context.Context, leftID, rightID string) (bool, error) {
	var leftWork string
	err := r.db.QueryRowContext(ctx, `
		SELECT x.work_id
		FROM bookdb.editions e
		JOIN bookdb.expressions x ON x.expression_id = e.expression_id
		WHERE e.edition_id = $1`, leftID).Scan(&leftWork)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("catalog: left edition work: %w", err)
	}

	var rightWork string
	err = r.db.QueryRowContext(ctx, `
		SELECT x.work_id
		FROM bookdb.editions e
		JOIN bookdb.expressions x ON x.expression_id = e.expression_id
		WHERE e.edition_id = $1`, rightID).Scan(&rightWork)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("catalog: right edition work: %w", err)
	}
	return leftWork == rightWork, nil
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}
