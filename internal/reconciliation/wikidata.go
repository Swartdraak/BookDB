// wikidata.go implements the S3 Wikidata enrichment client for issue #55:
// cross-source authority IDs (QIDs) and multilingual names, stored as
// claims in bookdb.wikidata_claims via Reconciler.RecordWikidataClaim and
// carried as identifier evidence (namespace "wikidata", status
// 'candidate' — never an auto-merge, per the matching policy).
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// WDLabel is a localized entity label.
type WDLabel struct {
	Language string `json:"language"`
	Value    string `json:"value"`
}

// WDEntity is the supplementary Wikidata data for one QID.
type WDEntity struct {
	QID      string    `json:"qid"`
	Labels   []WDLabel `json:"labels"`
	BirthRaw json.RawMessage
	DeathRaw json.RawMessage
}

// wdClaim is a single Wikidata claim value.
type wdClaim struct {
	Type     string `json:"type"`
	Mainsnak struct {
		Snaktype  string `json:"snaktype"`
		Property  string `json:"property"`
		Datavalue struct {
			Value struct {
				ID            string `json:"id"`
				Text          string `json:"text"`
				Time          string `json:"time"`
				Timezone      int    `json:"timezone"`
				Precision     int    `json:"precision"`
				CalendarModel string `json:"calendarmodel"`
			} `json:"value"`
			Type string `json:"type"`
		} `json:"datavalue"`
		Qualifiers json.RawMessage `json:"qualifiers"`
	} `json:"mainsnak"`
	Rank string `json:"rank"`
}

// wdEntityData is the response shape of
// Special:EntityData/{QID}.json.
type wdEntityData struct {
	Entities map[string]struct {
		Type   string `json:"type"`
		ID     string `json:"id"`
		Labels map[string]struct {
			Language string `json:"language"`
			Value    string `json:"value"`
		} `json:"labels"`
		Claims map[string][]wdClaim `json:"claims"`
	} `json:"entities"`
}

// FetchEntity fetches Wikidata entity data for a QID (e.g. "Q892").
// ErrNotFound is returned for unknown QIDs.
func (c *EnrichmentClient) FetchEntity(ctx context.Context, qid string) (*WDEntity, error) {
	rawURL := strings.TrimSuffix(c.cfg.WDBase, "/") + "/wiki/Special:EntityData/" + url.PathEscape(qid) + ".json"
	var data wdEntityData
	if err := c.getJSON(ctx, rawURL, &data); err != nil {
		return nil, err
	}
	ent, ok := data.Entities[qid]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, rawURL)
	}
	if ent.Type != "item" {
		return nil, fmt.Errorf("reconciliation: %s is a %s, not an item", qid, ent.Type)
	}
	e := &WDEntity{QID: qid}
	langs := make([]string, 0, len(ent.Labels))
	for l := range ent.Labels {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, l := range langs {
		lbl := ent.Labels[l]
		e.Labels = append(e.Labels, WDLabel{Language: lbl.Language, Value: lbl.Value})
	}
	// Preferred birth/death dates (P569/P570), normal rank if present.
	e.BirthRaw, _ = extractWDDateValue(ent.Claims["P569"])
	e.DeathRaw, _ = extractWDDateValue(ent.Claims["P570"])
	return e, nil
}

// SearchCandidates looks up candidate QIDs for a display name via the
// wbsearchentities API (searches labels in all languages). Returns at most
// limit candidates, best match first.
func (c *EnrichmentClient) SearchCandidates(ctx context.Context, name string, limit int) ([]WDCandidate, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}
	u := strings.TrimSuffix(c.cfg.WDBase, "/") + "/w/api.php"
	q := url.Values{
		"action":   {"wbsearchentities"},
		"language": {"en"},
		"format":   {"json"},
		"origin":   {"*"},
		"search":   {name},
		"limit":    {fmt.Sprintf("%d", limit)},
	}
	rawURL := u + "?" + q.Encode()
	var data struct {
		Search []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"search"`
	}
	if err := c.getJSON(ctx, rawURL, &data); err != nil {
		return nil, err
	}
	out := make([]WDCandidate, 0, len(data.Search))
	for _, s := range data.Search {
		out = append(out, WDCandidate{QID: s.ID, Label: s.Label})
	}
	return out, nil
}

// WDCandidate is a search result from the Wikidata search API.
type WDCandidate struct {
	QID   string `json:"qid"`
	Label string `json:"label"`
}

// extractWDDateValue picks the preferred-rank Wikidata date claim for
// P569/P570 (normal rank when no preferred claim exists) and returns its
// time value as raw JSON.
func extractWDDateValue(claims []wdClaim) (json.RawMessage, bool) {
	if len(claims) == 0 {
		return nil, false
	}
	// Prefer the preferred-rank claim when present.
	c := claims[0]
	for _, cc := range claims[1:] {
		if cc.Rank == "preferred" {
			c = cc
			break
		}
	}
	if c.Mainsnak.Datavalue.Value.Time == "" {
		return nil, false
	}
	raw, _ := json.Marshal(map[string]string{"time": c.Mainsnak.Datavalue.Value.Time})
	return raw, true
}

// EnrichPersonWithWikidata fetches the Wikidata entity for a known QID,
// stores its claims (multilingual labels, birth/death) in
// bookdb.wikidata_claims, and records a 'candidate' identifier row linking
// the QID to the person (cross-source identity evidence; never resolved
// automatically).
func (r *Reconciler) EnrichPersonWithWikidata(ctx context.Context, client *EnrichmentClient, personID uuid.UUID, qid string) (int, error) {
	e, err := client.FetchEntity(ctx, qid)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	stored := 0
	for _, lbl := range e.Labels {
		if lbl.Value == "" {
			continue
		}
		raw, _ := json.Marshal(lbl.Value)
		if err := r.RecordWikidataClaim(ctx, "person", personID, e.QID, "label:"+lbl.Language, raw); err != nil {
			return stored, err
		}
		stored++
	}
	if len(e.BirthRaw) > 0 {
		if err := r.RecordWikidataClaim(ctx, "person", personID, e.QID, "birth_date", e.BirthRaw); err != nil {
			return stored, err
		}
		stored++
	}
	if len(e.DeathRaw) > 0 {
		if err := r.RecordWikidataClaim(ctx, "person", personID, e.QID, "death_date", e.DeathRaw); err != nil {
			return stored, err
		}
		stored++
	}
	// Record the QID as candidate identifier evidence (cross-source
	// identity; matching policy: never auto-merge on this alone).
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO bookdb.identifiers (namespace, normalized_value, raw_value, target_type, target_id, status, evidence)
		VALUES ('wikidata', $1, $1, 'person', $2, 'candidate', $3)
		ON CONFLICT (namespace, normalized_value, target_type, target_id) DO NOTHING`,
		e.QID, personID, `{"via": "wikidata_enrichment"}`); err != nil {
		return stored, fmt.Errorf("reconciliation: wikidata identifier: %w", err)
	}
	return stored, nil
}
