package opensearch

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/database"
)

// openSearchTestDB connects to the test database the same way the promotion
// and apikey integration tests do: BOOKDB_TEST_DATABASE_URL, or the
// documented local dev DSN. When no database is reachable the caller's tests
// skip, so `make test` stays green in gate environments without a database.
// Migrations are applied so the outbox / canonical_revisions /
// search_projection_state tables exist.
func openSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb:***@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Clean schema + migrations in ONE advisory-locked critical section so
	// this package's reset and migration DDL cannot interleave with a
	// concurrent `go test` package (issue #68).
	if _, err := database.PrepareCleanAndMigrate(ctx, db, "bookdb"); err != nil {
		t.Fatalf("prepare clean test schema: %v", err)
	}
	return db
}

// osStub is a minimal in-memory OpenSearch server that emulates the exact
// surface the repo client and indexer use: cluster health, index HEAD/PUT,
// alias actions (real OpenSearch semantics: adding an existing alias ->
// 400 AliasAlreadyExistsException, which the indexer explicitly tolerates),
// alias-aware _doc PUTs (replace-on-reindex) and _search
// (case-insensitive substring match over the multi_match query string,
// size applied). It records every doc write so idempotency can be asserted.
type osStub struct {
	*httptest.Server
	mu        sync.Mutex
	aliases   map[string]string            // alias -> concrete index
	docs      map[string]map[string]string // index -> docID -> raw JSON
	docPuts   map[string]int               // alias + "/" + docID -> write count
	indexDocs map[string]bool              // concrete index exists
	log       *strings.Builder             // raw request log for diagnostics
}

func newOSStub(t *testing.T) *osStub {
	t.Helper()
	stub := &osStub{
		aliases:   map[string]string{},
		docs:      map[string]map[string]string{},
		docPuts:   map[string]int{},
		indexDocs: map[string]bool{},
		log:       &strings.Builder{},
	}
	stub.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ := io.ReadAll(r.Body)
		stub.mu.Lock()
		fmt.Fprintf(stub.log, "REQ %s %s BODY=%q\n", r.Method, r.URL.Path, string(rawBody))
		stub.mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/")
		parts := strings.Split(path, "/")

		switch {
		case path == "_cluster/health":
			writeJSONStub(w, 200, map[string]any{"status": "green"})
			return
		case len(parts) == 3 && parts[1] == "_doc":
			aliasOrIndex, docID := parts[0], parts[2]
			var raw map[string]any
			_ = json.Unmarshal(rawBody, &raw)
			stub.mu.Lock()
			index := aliasOrIndex
			if concrete, ok := stub.aliases[aliasOrIndex]; ok {
				index = concrete
			}
			encoded, _ := json.Marshal(raw)
			if stub.docs[index] == nil {
				stub.docs[index] = map[string]string{}
			}
			stub.docs[index][docID] = string(encoded)
			stub.docPuts[aliasOrIndex+"/"+docID]++
			stub.mu.Unlock()
			writeJSONStub(w, 200, map[string]any{"result": "created"})
			return
		case len(parts) == 1 && parts[0] == "_aliases" && r.Method == http.MethodPost,
			len(parts) == 2 && parts[1] == "_aliases" && r.Method == http.MethodPost:
			stub.mu.Lock()
			if len(rawBody) > 0 {
				var body struct {
					Actions []struct {
						Add struct {
							Index string `json:"index"`
							Alias string `json:"alias"`
						} `json:"add"`
					} `json:"actions"`
				}
				_ = json.Unmarshal(rawBody, &body)
				for _, a := range body.Actions {
					if a.Add.Alias == "" {
						continue
					}
					// Real OpenSearch rejects adding an alias that already
					// exists with a 400 AliasAlreadyExistsException; the
					// repo indexer treats that as acceptable (idempotent
					// ensure). Mirror that so the ensure path is tested the
					// same way it behaves against a live cluster.
					if current, ok := stub.aliases[a.Add.Alias]; ok && current == a.Add.Index {
						stub.mu.Unlock()
						writeJSONStub(w, 400, map[string]any{
							"error": map[string]any{
								"type":   "alias_already_exists_exception",
								"reason": "alias [" + a.Add.Alias + "] already exists",
							},
						})
						return
					}
					stub.aliases[a.Add.Alias] = a.Add.Index
				}
			}
			stub.mu.Unlock()
			if len(rawBody) == 0 {
				// Real OpenSearch rejects an alias action with no actions;
				// the repo indexer treats >=300 non-400 as an error.
				writeJSONStub(w, 400, map[string]any{"error": "no actions"})
				return
			}
			writeJSONStub(w, 200, map[string]any{"acknowledged": true})
			return
		case len(parts) == 2 && parts[1] == "_search":
			aliasOrIndex := parts[0]
			var query struct {
				Size  int `json:"size"`
				Inner struct {
					MultiMatch struct {
						Query string `json:"query"`
					} `json:"multi_match"`
				} `json:"query"`
			}
			_ = json.Unmarshal(rawBody, &query)
			stub.mu.Lock()
			index := aliasOrIndex
			if concrete, ok := stub.aliases[aliasOrIndex]; ok {
				index = concrete
			}
			hits := []map[string]any{}
			total := 0
			for docID, encoded := range stub.docs[index] {
				var doc map[string]any
				_ = json.Unmarshal([]byte(encoded), &doc)
				if docMatches(doc, query.Inner.MultiMatch.Query) {
					total++
					if query.Size <= 0 || len(hits) < query.Size {
						hits = append(hits, map[string]any{"_id": docID, "_source": doc})
					}
				}
			}
			stub.mu.Unlock()
			writeJSONStub(w, 200, map[string]any{
				"hits": map[string]any{
					"total": map[string]any{"value": total},
					"hits":  hits,
				},
			})
			return
		default:
			// Index HEAD (existence) and index PUT (creation).
			stub.mu.Lock()
			exists := stub.indexDocs[path]
			if !exists && r.Method == http.MethodPut {
				stub.indexDocs[path] = true
				stub.docs[path] = map[string]string{}
				exists = true
			}
			stub.mu.Unlock()
			if exists {
				writeJSONStub(w, 200, map[string]any{"ok": true})
				return
			}
			writeJSONStub(w, 404, map[string]any{"error": "index_not_found"})
			return
		}
	}))
	t.Cleanup(stub.Close)
	return stub
}

func writeJSONStub(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// docMatches is the stub's naive multi_match emulation: a case-insensitive
// substring match of the query against the text fields the indexer searches
// (title, authors, normalized) plus canonical_title.
func docMatches(doc map[string]any, query string) bool {
	if query == "" {
		return true
	}
	needle := toLower(query)
	hay := ""
	for _, field := range []string{"title", "normalized", "authors", "canonical_title"} {
		if v, ok := doc[field]; ok {
			hay += toLower(fmt.Sprintf("%v", v)) + " "
		}
	}
	return strings.Contains(hay, needle)
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// TestProcessOutboxIndexesEvents covers the committed outbox->indexer
// integration gap: unpublished outbox events are applied to the search
// projection through the real IndexDocument path, marked published, and
// recorded in bookdb.search_projection_state; a re-run over a drained
// outbox is a no-op (idempotent).
func TestProcessOutboxIndexesEvents(t *testing.T) {
	db := openSearchTestDB(t)
	ctx := context.Background()
	stub := newOSStub(t)
	client, err := New(stub.Server.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Clean the tables this test writes to (shared disposable test DB).
	for _, tbl := range []string{"search_projection_state", "canonical_revisions", "outbox"} {
		if _, err := db.ExecContext(ctx, `TRUNCATE bookdb.`+tbl+` RESTART IDENTITY CASCADE`); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}

	workID := "11111111-1111-1111-1111-111111111111"
	payload, _ := json.Marshal(map[string]any{
		"canonical_title": "Outbox Projection Work",
		"normalized":      "outbox projection work",
		"authors":         "Test Author",
	})

	// Seed two unpublished outbox events for one work: a work.promoted with
	// a canonical revision and a work.accepted without one
	// (recordProjectionState must fall back to revision 1 when absent).
	res, err := db.QueryContext(ctx, `
		INSERT INTO bookdb.outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4::jsonb), ($1, $2, $5, $4::jsonb)
		RETURNING id`, "work", workID, "work.promoted", payload, "work.accepted")
	if err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	var eventIDs []int64
	for res.Next() {
		var id int64
		if err := res.Scan(&id); err != nil {
			res.Close()
			t.Fatalf("scan outbox id: %v", err)
		}
		eventIDs = append(eventIDs, id)
	}
	res.Close()
	if len(eventIDs) != 2 {
		t.Fatalf("expected 2 outbox rows, got %d", len(eventIDs))
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO bookdb.canonical_revisions (entity_type, entity_id, revision) VALUES ($1, $2, $3)`,
		"work", workID, 7); err != nil {
		t.Fatalf("seed canonical_revisions: %v", err)
	}

	ix := NewIndexer(client, db)
	if err := ix.EnsureIndex(ctx, "work", 1); err != nil {
		stub.mu.Lock()
		log := stub.log.String()
		stub.mu.Unlock()
		t.Fatalf("EnsureIndex: %v\nstub saw:\n%s", err, log)
	}

	processed, err := ix.ProcessOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessOutbox: %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 events processed, got %d", processed)
	}

	// Both events marked published.
	var unpublished int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatalf("count unpublished: %v", err)
	}
	if unpublished != 0 {
		t.Fatalf("expected 0 unpublished outbox rows, got %d", unpublished)
	}

	// Projection state: one upserted row per (entity_type, entity_id),
	// revision taken from canonical_revisions when present.
	var revision int64
	if err := db.QueryRowContext(ctx,
		`SELECT revision FROM bookdb.search_projection_state WHERE entity_type = $1 AND entity_id = $2`,
		"work", workID).Scan(&revision); err != nil {
		t.Fatalf("search_projection_state missing: %v", err)
	}
	if revision != 7 {
		t.Fatalf("expected revision 7 from canonical_revisions, got %d", revision)
	}

	// The stub received both doc writes under the alias; same doc ID (entity
	// id) means re-indexing replaces, never duplicates.
	stub.mu.Lock()
	workIndex := stub.aliases["bookdb_work"]
	docs := len(stub.docs[workIndex])
	puts := stub.docPuts["bookdb_work/"+workID]
	stub.mu.Unlock()
	if workIndex == "" {
		t.Fatal("alias bookdb_work was not registered on the stub")
	}
	if docs != 1 {
		t.Fatalf("expected exactly 1 doc in the work index, got %d", docs)
	}
	if puts != 2 {
		t.Fatalf("expected 2 doc writes for the entity (one per event), got %d", puts)
	}

	// Search retrieves the indexed doc by title.
	docsFound, err := ix.Search(ctx, "work", "Outbox Projection Work", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docsFound) != 1 {
		t.Fatalf("expected 1 search hit, got %d", len(docsFound))
	}
	if docsFound[0]["canonical_title"] != "Outbox Projection Work" {
		t.Fatalf("unexpected search hit: %v", docsFound[0])
	}

	// Idempotency: a second run over a drained outbox processes nothing and
	// leaves the projection untouched.
	processed2, err := ix.ProcessOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessOutbox (re-run): %v", err)
	}
	if processed2 != 0 {
		t.Fatalf("expected 0 events on re-run, got %d", processed2)
	}
	stub.mu.Lock()
	docsAfter := len(stub.docs[workIndex])
	putsAfter := stub.docPuts["bookdb_work/"+workID]
	stub.mu.Unlock()
	if docsAfter != 1 || putsAfter != 2 {
		t.Fatalf("re-run changed the projection: docs=%d puts=%d (want 1/2)", docsAfter, putsAfter)
	}
}

// TestProcessOutboxMarksUnparseablePayloadsPublished ensures an unparseable
// payload is marked published rather than stalling the batch (poison-pill
// protection) while the batch's other events still project. The payload
// column is JSONB, so the in-schema shape of an unparseable document is
// valid JSON that is not an object (a JSON string): json.Unmarshal into
// map[string]any fails for it.
func TestProcessOutboxMarksUnparseablePayloadsPublished(t *testing.T) {
	db := openSearchTestDB(t)
	ctx := context.Background()
	stub := newOSStub(t)
	client, err := New(stub.Server.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, tbl := range []string{"search_projection_state", "canonical_revisions", "outbox"} {
		if _, err := db.ExecContext(ctx, `TRUNCATE bookdb.`+tbl+` RESTART IDENTITY CASCADE`); err != nil {
			t.Fatalf("truncate %s: %v", tbl, err)
		}
	}

	goodID := "22222222-2222-2222-2222-222222222222"
	badID := "33333333-3333-3333-3333-333333333333"
	goodPayload, _ := json.Marshal(map[string]any{"canonical_title": "Survivor Work", "normalized": "survivor work"})
	if _, err := db.ExecContext(ctx, `
		INSERT INTO bookdb.outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4::jsonb), ($1, $5, $3, $6::jsonb)`,
		"work", goodID, "work.promoted", goodPayload, badID, `"not-a-document"`); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}

	ix := NewIndexer(client, db)
	if err := ix.EnsureIndex(ctx, "work", 1); err != nil {
		t.Fatalf("EnsureIndex: %v", err)
	}

	processed, err := ix.ProcessOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessOutbox: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 good event processed, got %d", processed)
	}
	var unpublished int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM bookdb.outbox WHERE published_at IS NULL`).Scan(&unpublished); err != nil {
		t.Fatalf("count unpublished: %v", err)
	}
	if unpublished != 0 {
		t.Fatalf("unparseable row must be marked published to avoid stalling the batch; %d still unpublished", unpublished)
	}
	stub.mu.Lock()
	workIndex := stub.aliases["bookdb_work"]
	docs := len(stub.docs[workIndex])
	stub.mu.Unlock()
	if docs != 1 {
		t.Fatalf("expected exactly 1 doc (the good event), got %d", docs)
	}
}
