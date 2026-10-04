package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/apikey"
	"github.com/bookdb/bookdb/internal/auth"
	"github.com/google/uuid"
)

// The reconciliation permission-boundary tests (issue #60, follow-up of the
// issue #55 review) pin the authz surface of /api/v1/reconciliation/* at the
// HTTP level, mirroring TestJobAdmin_PermissionBoundaries:
//
//   - no API key                          -> 401 (mutation AND read)
//   - invalid API key                      -> 401
//   - valid key without recon scope        -> 403
//   - valid scoped key, non-admin session  -> 403 (mutations only; reads pass)
//   - valid scoped key + admin session     -> 2xx (mutation AND read)
//
// for BOTH the pre-existing handlers (merge, split, resolve, changes,
// duplicates) and the issue #55 handlers (reconcile, duplicates/generate).

// newTestReconServer builds a reconciliation server plus the key store and a
// reconciliation-scoped test key.
func newTestReconServer(t *testing.T) (*ReconciliationServer, *apikey.Store, string, []byte) {
	t.Helper()
	db := openTestDB(t)
	macKey := []byte("recon-test-mac-key-0000000000000000")
	srv := NewReconciliationServer(db, macKey)
	store := apikey.NewStore(db, macKey)
	issued, err := store.Create(context.Background(), "recon-key", []string{ScopeReconciliation}, nil, nil)
	if err != nil {
		t.Fatalf("Create recon key: %v", err)
	}
	return srv, store, issued.Secret, macKey
}

// insertReconWork inserts a minimal work row (and its canonical revision)
// into the schema loaded by openTestDB, cleaned up on test end. The S1
// fixtures do not seed the S3 identity tables, so the resolve boundary test
// carries its own row to prove the read path reaches the handler.
func insertReconWork(t *testing.T, db *sql.DB, title string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	workID := uuid.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, $2, $3)`,
		workID, title, strings.ToLower(title)); err != nil {
		t.Fatalf("insert recon work: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.identity_redirects WHERE from_id = $1 OR to_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.works WHERE work_id = $1`, workID)
	})
	return workID
}

// doReconRequest issues a request to the reconciliation handler with an
// optional API key and session cookie.
func doReconRequest(t *testing.T, h http.Handler, apikey, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var r *strings.Reader
	if body != "" {
		r = strings.NewReader(body)
	} else {
		r = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if apikey != "" {
		req.Header.Set("X-API-Key", apikey)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestReconciliation_PermissionBoundaries pins the full authz matrix over
// every endpoint in the namespace — pre-existing (merge, split, resolve,
// changes, duplicates) and issue #55 (reconcile, duplicates/generate).
func TestReconciliation_PermissionBoundaries(t *testing.T) {
	db := openTestDB(t)
	macKey := []byte("recon-bound-mac-key-00000000000000")
	srv := NewReconciliationServer(db, macKey)
	h := srv.Handler()
	store := apikey.NewStore(db, macKey)

	issued, err := store.Create(context.Background(), "recon-bound-key", []string{ScopeReconciliation}, nil, nil)
	if err != nil {
		t.Fatalf("Create recon key: %v", err)
	}
	readOnlyKey, err := store.Create(context.Background(), "recon-bound-catalog-key", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create catalog-scope key: %v", err)
	}

	password := "recon-bound-pass-123"
	stamp := itoa(time.Now().UnixNano())
	admin := registerAuthUser(t, db, "rba_admin_"+stamp, string(auth.RoleAdministrator), password)
	contrib := registerAuthUser(t, db, "rba_contrib_"+stamp, string(auth.RoleContributor), password)

	adminCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), admin.Username, password)
	contribCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), contrib.Username, password)

	workID := insertReconWork(t, db, "Recon Boundary Work")

	// The merge subtest merges workID into a SECOND real work row, never a
	// bare UUID. Merge() makes the lower UUID canonical, so with a bare
	// uuid.New() target the canonical side would be dangling ~50% of runs
	// (no works row), and the resolve-read proof below would fail on the
	// dangling redirect. With two real rows the canonical target always
	// exists in the catalog.
	mergeTargetID := insertReconWork(t, db, "Recon Boundary Merge Target")
	mergeBody := `{"entity_type":"work","id_a":"` + workID.String() + `","id_b":"` + mergeTargetID.String() + `","reason":"boundary-test"}`
	// Split targets a FRESH work row so its optimistic-concurrency check
	// (expected_revision) is not disturbed by the merge subtest's revision
	// bump on the merge row.
	splitWorkID := insertReconWork(t, db, "Recon Boundary Split Work")
	splitBody := `{"entity_type":"work","original_id":"` + splitWorkID.String() + `","expected_revision":1,"reason":"boundary-test"}`
	reconcileBody := `{"source":"openlibrary","max_per_type":1}`

	// (method, path, body, adminOnly) for every endpoint in the namespace.
	endpoints := []struct {
		name      string
		method    string
		path      string
		body      string
		adminOnly bool
	}{
		{"merge (pre-existing)", http.MethodPost, "/api/v1/reconciliation/merge", mergeBody, true},
		{"split (pre-existing)", http.MethodPost, "/api/v1/reconciliation/split", splitBody, true},
		{"resolve (pre-existing)", http.MethodGet, "/api/v1/reconciliation/resolve/work/" + workID.String(), "", false},
		{"changes (pre-existing)", http.MethodGet, "/api/v1/reconciliation/changes", "", false},
		{"duplicates (pre-existing)", http.MethodGet, "/api/v1/reconciliation/duplicates/work", "", false},
		{"reconcile (#55)", http.MethodPost, "/api/v1/reconciliation/reconcile", reconcileBody, true},
		{"duplicates/generate (#55)", http.MethodPost, "/api/v1/reconciliation/duplicates/generate", "", true},
	}

	for _, ep := range endpoints {
		ep := ep
		t.Run("no key -> 401: "+ep.name, func(t *testing.T) {
			rec := doReconRequest(t, h, "", ep.method, ep.path, ep.body, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("no-key %s: got %d, want 401: %s", ep.name, rec.Code, rec.Body.String())
			}
		})

		t.Run("invalid key -> 401: "+ep.name, func(t *testing.T) {
			rec := doReconRequest(t, h, "not-a-real-key", ep.method, ep.path, ep.body, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("invalid-key %s: got %d, want 401: %s", ep.name, rec.Code, rec.Body.String())
			}
		})

		t.Run("key without recon scope -> 403: "+ep.name, func(t *testing.T) {
			rec := doReconRequest(t, h, readOnlyKey.Secret, ep.method, ep.path, ep.body, adminCookie)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("catalog-scope key %s: got %d, want 403: %s", ep.name, rec.Code, rec.Body.String())
			}
		})

		if ep.adminOnly {
			t.Run("non-admin -> 403: "+ep.name, func(t *testing.T) {
				rec := doReconRequest(t, h, issued.Secret, ep.method, ep.path, ep.body, contribCookie)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("contributor %s: got %d, want 403: %s", ep.name, rec.Code, rec.Body.String())
				}
			})
			t.Run("scoped key without session -> 403: "+ep.name, func(t *testing.T) {
				rec := doReconRequest(t, h, issued.Secret, ep.method, ep.path, ep.body, nil)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("key-only %s: got %d, want 403: %s", ep.name, rec.Code, rec.Body.String())
				}
			})
			t.Run("admin -> handler reached: "+ep.name, func(t *testing.T) {
				rec := doReconRequest(t, h, issued.Secret, ep.method, ep.path, ep.body, adminCookie)
				// The boundary is: authz passes and the HANDLER runs. A 5xx here
				// means the boundary let an unhandled error through.
				if rec.Code >= 500 {
					t.Fatalf("admin %s: got %d, want 2xx/4xx from the handler: %s", ep.name, rec.Code, rec.Body.String())
				}
				// Split uses a fresh work row with expected_revision=1, so its
				// optimistic-concurrency check passes deterministically and the
				// handler must return 2xx.
				if ep.name == "split (pre-existing)" && (rec.Code < 200 || rec.Code >= 300) {
					t.Fatalf("admin %s: got %d, want 2xx: %s", ep.name, rec.Code, rec.Body.String())
				}
			})
		} else {
			// Reads: the documented floor is API-key auth; a valid scoped key
			// is enough even without a session.
			t.Run("scoped key without session -> 2xx: "+ep.name, func(t *testing.T) {
				rec := doReconRequest(t, h, issued.Secret, ep.method, ep.path, ep.body, nil)
				if rec.Code < 200 || rec.Code >= 300 {
					t.Fatalf("read %s: got %d, want 2xx: %s", ep.name, rec.Code, rec.Body.String())
				}
			})
		}
	}

	// Resolve read proves the handler actually ran (not a masked 4xx): the
	// response must carry the requested work ID. After the merge subtest,
	// workID's canonical target is the LOWER of workID and mergeTargetID —
	// a real, catalog-inserted work row (never a bare UUID) — so the
	// canonical must resolve to a real work (or the requested ID itself).
	// A dangling redirect target would be a data-integrity error, not a
	// handler success.
	rec := doReconRequest(t, h, issued.Secret, http.MethodGet, "/api/v1/reconciliation/resolve/work/"+workID.String(), "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve read: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resolved struct {
		EntityType  string `json:"entity_type"`
		RequestedID string `json:"requested_id"`
		CanonicalID string `json:"canonical_id"`
		IsRedirect  bool   `json:"is_redirect"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("unmarshal resolve: %v", err)
	}
	if resolved.RequestedID != workID.String() {
		t.Fatalf("resolve requested_id = %q, want %s", resolved.RequestedID, workID.String())
	}
	if resolved.CanonicalID != workID.String() {
		var canonicalInDB string
		if err := db.QueryRowContext(context.Background(),
			`SELECT work_id::text FROM bookdb.works WHERE work_id = $1`, resolved.CanonicalID).Scan(&canonicalInDB); err != nil {
			t.Fatalf("resolve canonical_id %q is not a work in the catalog: %v", resolved.CanonicalID, err)
		}
	}
}

// TestReconciliation_ReadsAllowKeyWithoutSession pins that the read surface
// (resolve, changes, duplicates) is open to a valid reconciliation-scoped key
// without a session — the documented floor — while an invalid key is still
// rejected with 401.
func TestReconciliation_ReadsAllowKeyWithoutSession(t *testing.T) {
	db := openTestDB(t)
	macKey := []byte("recon-read-mac-key-0000000000000000")
	srv := NewReconciliationServer(db, macKey)
	h := srv.Handler()
	store := apikey.NewStore(db, macKey)

	issued, err := store.Create(context.Background(), "recon-read-key", []string{ScopeReconciliation}, nil, nil)
	if err != nil {
		t.Fatalf("Create key: %v", err)
	}
	workID := insertReconWork(t, db, "Recon Read Work")

	for _, ep := range []struct {
		name string
		path string
	}{
		{"resolve", "/api/v1/reconciliation/resolve/work/" + workID.String()},
		{"changes", "/api/v1/reconciliation/changes"},
		{"duplicates", "/api/v1/reconciliation/duplicates/work"},
	} {
		rec := doReconRequest(t, h, issued.Secret, http.MethodGet, ep.path, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("read %s with key only: got %d, want 200: %s", ep.name, rec.Code, rec.Body.String())
		}
		// The same read without any key stays closed.
		rec = doReconRequest(t, h, "", http.MethodGet, ep.path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("read %s without key: got %d, want 401", ep.name, rec.Code)
		}
	}
}

// TestReconciliation_KeyWithoutScopeForbidden pins that a VALID key lacking
// the reconciliation scope is 403 on the namespace — including for an
// administrator session — so an ordinary catalog key cannot ride into the
// admin mutation surface.
func TestReconciliation_KeyWithoutScopeForbidden(t *testing.T) {
	db := openTestDB(t)
	macKey := []byte("recon-scope-mac-key-00000000000000")
	srv := NewReconciliationServer(db, macKey)
	h := srv.Handler()
	store := apikey.NewStore(db, macKey)

	catalogKey, err := store.Create(context.Background(), "recon-scope-catalog", []string{ScopeRead}, nil, nil)
	if err != nil {
		t.Fatalf("Create catalog key: %v", err)
	}

	password := "recon-scope-pass-123"
	stamp := itoa(time.Now().UnixNano())
	admin := registerAuthUser(t, db, "rbs_admin_"+stamp, string(auth.RoleAdministrator), password)
	adminCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), admin.Username, password)

	// Even a full administrator session does not help: the client API key
	// must carry the reconciliation scope.
	rec := doReconRequest(t, h, catalogKey.Secret, http.MethodPost, "/api/v1/reconciliation/merge",
		`{"entity_type":"work","id_a":"`+uuid.New().String()+`","id_b":"`+uuid.New().String()+`","reason":"scope-test"}`, adminCookie)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("catalog-scope key + admin session merge: got %d, want 403: %s", rec.Code, rec.Body.String())
	}
}
