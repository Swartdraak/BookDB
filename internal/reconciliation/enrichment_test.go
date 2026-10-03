// enrichment_test.go — unit tests for the S3 enrichment and cross-source
// candidate pipeline (issue #55). HTTP is stubbed with httptest; no test
// performs real network I/O.
package reconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubServer serves OL-style and Wikidata-style fixtures.
func stubServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /works/OL18315W.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent header")
		}
		fmt.Fprint(w, `{
			"key": "/works/OL18315W",
			"description": {"type": "/type/text", "value": "A test work description."},
			"subjects": ["Testing", "Go programming"],
			"first_publish_date": "2020",
			"covers": [12345],
			"authors": [{"author": {"key": "/authors/OL17720A"}}]
		}`)
	})
	mux.HandleFunc("GET /works/OLMISSINGW.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("GET /authors/OL17720A.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"key": "/authors/OL17720A",
			"name": "Test Author",
			"birth_date": "1965-08-03",
			"death_date": "",
			"alternate_names": ["T. Author"],
			"portrait": "42",
			"website": "https://example.org/author"
		}`)
	})
	mux.HandleFunc("GET /wiki/Special:EntityData/Q892.json", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"entities": {
				"Q892": {
					"type": "item",
					"id": "Q892",
					"labels": {
						"en": {"language": "en", "value": "J. R. R. Tolkien"},
						"fr": {"language": "fr", "value": "Tolkien"}
					},
					"claims": {
						"P569": [{
							"type": "stmt",
							"mainsnak": {"snaktype": "value", "property": "P569", "datavalue": {"value": {"time": "+1892-01-03T00:00:00Z", "timezone": 0, "precision": 11, "calendarmodel": "http://www.w3.org/2001/XMLSchema#gregorian"}, "type": "time"}, "qualifiers": {}, "rank": "normal"}
						}],
						"P570": [{
							"type": "stmt",
							"mainsnak": {"snaktype": "value", "property": "P570", "datavalue": {"value": {"time": "+1973-09-02T00:00:00Z", "timezone": 0, "precision": 11, "calendarmodel": "http://www.w3.org/2001/XMLSchema#gregorian"}, "type": "time"}, "qualifiers": {}, "rank": "normal"}
						}]
					}
				}
			}
		}`)
	})
	mux.HandleFunc("GET /w/api.php", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "J. R. R. Tolkien" {
			t.Errorf("unexpected search %q", r.URL.Query().Get("search"))
		}
		fmt.Fprint(w, `{"search": [{"id": "Q892", "label": "J. R. R. Tolkien"}]}`)
	})
	// 429 once then 200: retry policy check.
	flaky := 0
	mux.HandleFunc("GET /works/OLFLAKYW.json", func(w http.ResponseWriter, r *http.Request) {
		if flaky == 0 {
			flaky++
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"key": "/works/OLFLAKYW", "subjects": ["Flaky"]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchWorkParsesSupplementaryFields(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL, MaxRetries: 2})
	w, err := c.FetchWork(context.Background(), "OL18315W")
	if err != nil {
		t.Fatalf("FetchWork: %v", err)
	}
	if w.Description != "A test work description." {
		t.Errorf("description = %q", w.Description)
	}
	if len(w.Subjects) != 2 || w.Subjects[0] != "Testing" {
		t.Errorf("subjects = %v", w.Subjects)
	}
	if w.FirstPub != "2020" {
		t.Errorf("first_publish_date = %q", w.FirstPub)
	}
	if w.CoverID != 12345 {
		t.Errorf("cover_i = %d", w.CoverID)
	}
	if len(w.AuthorKeys) != 1 || w.AuthorKeys[0] != "OL17720A" {
		t.Errorf("author_keys = %v", w.AuthorKeys)
	}
}

func TestFetchWorkNotFound(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL})
	_, err := c.FetchWork(context.Background(), "OLMISSINGW")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestFetchAuthorParsesSupplementaryFields(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL})
	a, err := c.FetchAuthor(context.Background(), "OL17720A")
	if err != nil {
		t.Fatalf("FetchAuthor: %v", err)
	}
	if a.Name != "Test Author" || a.BirthDate != "1965-08-03" {
		t.Errorf("name/birth = %q/%q", a.Name, a.BirthDate)
	}
	if len(a.Aliases) != 1 || a.Aliases[0] != "T. Author" {
		t.Errorf("aliases = %v", a.Aliases)
	}
	if a.PortraitID != 42 {
		t.Errorf("portrait_i = %d", a.PortraitID)
	}
}

func TestFetchEntityParsesLabelsAndDates(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{WDBase: srv.URL})
	e, err := c.FetchEntity(context.Background(), "Q892")
	if err != nil {
		t.Fatalf("FetchEntity: %v", err)
	}
	if e.QID != "Q892" || len(e.Labels) != 2 {
		t.Fatalf("qid=%q labels=%v", e.QID, e.Labels)
	}
	found := map[string]string{}
	for _, l := range e.Labels {
		found[l.Language] = l.Value
	}
	if found["en"] != "J. R. R. Tolkien" || found["fr"] != "Tolkien" {
		t.Errorf("labels = %v", found)
	}
	var birth struct {
		Time string `json:"time"`
	}
	if err := json.Unmarshal(e.BirthRaw, &birth); err != nil || birth.Time != "+1892-01-03T00:00:00Z" {
		t.Errorf("birth = %s (err %v)", e.BirthRaw, err)
	}
}

func TestSearchCandidates(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{WDBase: srv.URL})
	cands, err := c.SearchCandidates(context.Background(), "J. R. R. Tolkien", 5)
	if err != nil {
		t.Fatalf("SearchCandidates: %v", err)
	}
	if len(cands) != 1 || cands[0].QID != "Q892" {
		t.Fatalf("candidates = %v", cands)
	}
}

func TestGetJSONRetriesOn429(t *testing.T) {
	srv := stubServer(t)
	c := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL, MaxRetries: 2})
	w, err := c.FetchWork(context.Background(), "OLFLAKYW")
	if err != nil {
		t.Fatalf("FetchWork flaky: %v", err)
	}
	if len(w.Subjects) != 1 || w.Subjects[0] != "Flaky" {
		t.Errorf("subjects = %v", w.Subjects)
	}
}

func TestGetJSONNoRetryOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	c := NewEnrichmentClient(EnrichmentConfig{OLBase: srv.URL, MaxRetries: 3})
	_, err := c.FetchWork(context.Background(), "OLANYW")
	if err == nil || strings.Contains(err.Error(), "after 4 attempts") {
		t.Fatalf("want immediate 403 error, got %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("want HTTP 403 in error, got %v", err)
	}
}

func TestNormalizeTitleStable(t *testing.T) {
	got := NormalizeTitle("  The  Hobbit: An Unexpected Journey ")
	want := "the hobbit: an unexpected journey"
	if got != want {
		t.Fatalf("NormalizeTitle = %q, want %q", got, want)
	}
}

func TestLooksLikeAuthorKey(t *testing.T) {
	cases := map[string]bool{
		"OL17720A": true,
		"OL18315W": false,
		"OL123M":   false,
		"OL":       false,
		"OL12A":    true,
		"OL1234XA": false,
	}
	for in, want := range cases {
		if got := looksLikeAuthorKey(in); got != want {
			t.Errorf("looksLikeAuthorKey(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestInferEntityType(t *testing.T) {
	cases := map[struct{ title, key string }]string{
		{"The Hobbit", "OL18315W"}: "work",
		{"", "OL17720A"}:           "person",
		{"", "OL18315W"}:           "",
	}
	for in, want := range cases {
		if got := inferEntityType(in.title, in.key, "uuid"); got != want {
			t.Errorf("inferEntityType(%q,%q) = %q, want %q", in.title, in.key, got, want)
		}
	}
}

func TestSharedIdentifierValues(t *testing.T) {
	a := entityRecord{}
	a.SourceRecord.EntityID = "11111111-1111-1111-1111-111111111111"
	a.Title = "The Hobbit"
	b := entityRecord{}
	b.SourceRecord.EntityID = "22222222-2222-2222-2222-222222222222"
	b.Title = "The Hobbit"
	// Shared author key:
	b.Authors = []string{"OL17720A"}
	a.Authors = []string{"/authors/OL17720A"}
	shared := sharedIdentifierValues("OL18315W", a, "WD99", b)
	found := false
	for _, s := range shared {
		if s == "OL17720A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("shared = %v, want to contain OL17720A", shared)
	}
}
