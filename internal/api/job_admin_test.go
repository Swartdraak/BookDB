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

	"github.com/bookdb/bookdb/internal/auth"
	"github.com/bookdb/bookdb/internal/ingestion"
)

// insertJobRow inserts an ingestion job for the given status and returns
// its UUID as a string. Cleaned up on test end.
func insertJobRow(t *testing.T, db *sql.DB, status, tag string) string {
	t.Helper()
	ctx := context.Background()
	var jobID string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO bookdb.ingestion_jobs (source_name, snapshot_id, status)
		 VALUES ('s4admin-test', $1, $2) RETURNING job_id::text`,
		"job-"+tag, status).Scan(&jobID); err != nil {
		t.Fatalf("seed job: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.ingestion_jobs WHERE job_id = $1`, jobID)
	})
	return jobID
}

// doJobRequest is a small HTTP helper for the job-admin tests.
func doJobRequest(t *testing.T, h http.Handler, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *strings.Reader
	if body != "" {
		r = strings.NewReader(body)
	} else {
		r = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustJobJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal job: %v", err)
	}
	return m
}

// TestJobAdmin_PermissionBoundaries pins the S4-ADMIN permission boundary
// at the HTTP level (BDB-011 "clear permission boundaries"): no session is
// 401, reader/contributor are 403 (before any resource lookup, so job IDs
// cannot be probed), administrator reads and writes.
func TestJobAdmin_PermissionBoundaries(t *testing.T) {
	db := openTestDB(t)
	h := NewJobAdminServer(db).Handler()
	stamp := itoa(time.Now().UnixNano())
	password := "jobadmin-pass-123"

	admin := registerAuthUser(t, db, "ja_admin_"+stamp, string(auth.RoleAdministrator), password)
	contrib := registerAuthUser(t, db, "ja_contrib_"+stamp, string(auth.RoleContributor), password)
	reader := registerAuthUser(t, db, "ja_reader_"+stamp, string(auth.RoleReader), password)

	adminCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), admin.Username, password)
	contribCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), contrib.Username, password)
	readerCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), reader.Username, password)

	jobID := insertJobRow(t, db, "running", stamp)

	// 1. No session: 401 on read and write alike.
	rec := doJobRequest(t, h, nil, http.MethodGet, "/api/v1/ingestion/jobs", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-session list: got %d, want 401: %s", rec.Code, rec.Body.String())
	}
	rec = doJobRequest(t, h, nil, http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/pause", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-session pause: got %d, want 401: %s", rec.Code, rec.Body.String())
	}

	// 2. Reader: 403 on read.
	rec = doJobRequest(t, h, readerCookie, http.MethodGet, "/api/v1/ingestion/jobs", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reader list: got %d, want 403: %s", rec.Code, rec.Body.String())
	}

	// 3. Contributor: 403 on write — even on a valid job ID, so a
	// non-admin cannot probe job IDs through state errors.
	rec = doJobRequest(t, h, contribCookie, http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/pause", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("contributor pause: got %d, want 403: %s", rec.Code, rec.Body.String())
	}

	// 4. Admin read: 200 and the job is visible with its state.
	rec = doJobRequest(t, h, adminCookie, http.MethodGet, "/api/v1/ingestion/jobs/"+jobID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin get: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	job := mustJobJSON(t, rec.Body.Bytes())
	if job["status"] != "running" {
		t.Fatalf("job status = %v, want running", job["status"])
	}

	// 5. Admin list: 200, well-formed array including our job.
	rec = doJobRequest(t, h, adminCookie, http.MethodGet, "/api/v1/ingestion/jobs?source=s4admin-test", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Jobs  []map[string]any `json:"jobs"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listResp.Total != 1 {
		t.Fatalf("list total = %d, want 1", listResp.Total)
	}
	if listResp.Jobs[0]["job_id"] != jobID {
		t.Fatalf("list job_id = %v, want %s", listResp.Jobs[0]["job_id"], jobID)
	}

	// 6. Unknown job ID from an admin: 404, not 409 (no ID probing).
	rec = doJobRequest(t, h, adminCookie, http.MethodGet, "/api/v1/ingestion/jobs/99999999-9999-4999-8999-999999999999", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin get unknown: got %d, want 404: %s", rec.Code, rec.Body.String())
	}
	rec = doJobRequest(t, h, adminCookie, http.MethodPost, "/api/v1/ingestion/jobs/99999999-9999-4999-8999-999999999999/pause", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin pause unknown: got %d, want 404: %s", rec.Code, rec.Body.String())
	}

	// 7. Invalid action name: 400.
	rec = doJobRequest(t, h, adminCookie, http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/delete", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("admin delete: got %d, want 400: %s", rec.Code, rec.Body.String())
	}

	// 8. Malformed job ID: 400.
	rec = doJobRequest(t, h, adminCookie, http.MethodGet, "/api/v1/ingestion/jobs/not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("admin get bad id: got %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestJobAdmin_TransitionLifecycle pins the full BDB-011 state machine at
// the HTTP boundary: pause, resume, retry and quarantine (with required
// reason), the 400 on a reason-less quarantine, and the 409 on a transition
// out of a terminal state.
func TestJobAdmin_TransitionLifecycle(t *testing.T) {
	db := openTestDB(t)
	h := NewJobAdminServer(db).Handler()
	stamp := itoa(time.Now().UnixNano())
	password := "joblc-pass-123"
	admin := registerAuthUser(t, db, "jlc_admin_"+stamp, string(auth.RoleAdministrator), password)
	adminCookie := loginAuthCookie(t, NewAuthServer(db).Handler(), admin.Username, password)

	do := func(method, path, body string) *httptest.ResponseRecorder {
		return doJobRequest(t, h, adminCookie, method, path, body)
	}

	jobID := insertJobRow(t, db, "running", stamp)

	// running -> paused
	rec := do(http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/pause", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pause running: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	job := mustJobJSON(t, rec.Body.Bytes())
	if job["status"] != "paused" {
		t.Fatalf("status after pause = %v, want paused", job["status"])
	}

	// paused -> running (resume)
	rec = do(http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/resume", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("resume paused: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	job = mustJobJSON(t, rec.Body.Bytes())
	if job["status"] != "running" {
		t.Fatalf("status after resume = %v, want running", job["status"])
	}

	// quarantine WITHOUT reason: 400
	rec = do(http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/quarantine", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("quarantine no reason: got %d, want 400: %s", rec.Code, rec.Body.String())
	}

	// quarantine WITH reason: 200, reason persisted on the job
	rec = do(http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/quarantine",
		`{"reason":"repeated poison records"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("quarantine: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	job = mustJobJSON(t, rec.Body.Bytes())
	if job["status"] != "quarantined" {
		t.Fatalf("status after quarantine = %v, want quarantined", job["status"])
	}
	if job["error_message"] != "repeated poison records" {
		t.Fatalf("quarantine reason = %v, want the supplied reason", job["error_message"])
	}

	// terminal state: pause on quarantined -> 409 (documented conflict,
	// no partial state change)
	rec = do(http.MethodPost, "/api/v1/ingestion/jobs/"+jobID+"/pause", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("pause quarantined: got %d, want 409: %s", rec.Code, rec.Body.String())
	}

	// a failed job can be retried: failed -> running
	failedID := insertJobRow(t, db, "failed", stamp+"f")
	rec = do(http.MethodPost, "/api/v1/ingestion/jobs/"+failedID+"/retry", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("retry failed: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	job = mustJobJSON(t, rec.Body.Bytes())
	if job["status"] != "running" {
		t.Fatalf("status after retry = %v, want running", job["status"])
	}
}

// Pin the exact action names the handlers accept (BDB-011: trigger/
// pause/resume/quarantine boundaries are explicit, not open-ended).
var jobAdminActions = []string{
	ingestion.JobActionPause,
	ingestion.JobActionResume,
	ingestion.JobActionRetry,
	ingestion.JobActionQuarantine,
}
