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
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// registerAuthUser inserts a user with a real bcrypt hash (so Login works)
// and an explicit role, and returns the user.
func registerAuthUser(t *testing.T, db *sql.DB, username, role, password string) *auth.User {
	t.Helper()
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var u auth.User
	err = db.QueryRowContext(ctx, `
		INSERT INTO bookdb.users (username, email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, 'S4 API Test', $4, 'active')
		RETURNING user_id, username, email, display_name, role, status`,
		username, username+"@test.invalid", string(hash), role).
		Scan(&u.UserID, &u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.correction_proposals WHERE user_id = $1`, u.UserID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.users WHERE user_id = $1`, u.UserID)
	})
	return &u
}

// loginAuthCookie logs in a user and returns the bookdb_session cookie.
func loginAuthCookie(t *testing.T, h http.Handler, username, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: got %d, want 200", username, rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "bookdb_session" {
			return c
		}
	}
	t.Fatalf("login %s: no bookdb_session cookie", username)
	return nil
}

// TestProposalList_Privacy pins the privacy side of BDB-011 at the API
// boundary: a contributor's proposal list returns only that contributor's
// own proposals, and a pending proposal's value is absent from any other
// user's list — the list endpoint must not become a proposal side channel.
func TestProposalList_Privacy(t *testing.T) {
	db := openTestDB(t)
	h := NewAuthServer(db).Handler()
	stamp := time.Now().UnixNano()
	password := "privacy-test-pass-123"
	contrib := registerAuthUser(t, db, "contrib_priv_"+itoa(stamp), string(auth.RoleContributor), password)
	stranger := registerAuthUser(t, db, "stranger_priv_"+itoa(stamp), string(auth.RoleContributor), password)

	contribCookie := loginAuthCookie(t, h, contrib.Username, password)
	strangerCookie := loginAuthCookie(t, h, stranger.Username, password)

	secret := "API Privacy Probe " + itoa(stamp)
	workID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	submit := map[string]any{
		"entity_type":    "work",
		"entity_id":      workID.String(),
		"field_name":     "title",
		"proposed_value": secret,
		"rationale":      "privacy probe",
	}
	submitBody, _ := json.Marshal(submit)

	doWith := func(cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		var reader *strings.Reader
		if body != "" {
			reader = strings.NewReader(body)
		} else {
			reader = strings.NewReader("")
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// Contributor submits a pending proposal.
	rec := doWith(contribCookie, http.MethodPost, "/api/v1/proposals", string(submitBody))
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit proposal: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var submitted map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("unmarshal proposal: %v", err)
	}

	// The owner sees exactly one proposal carrying the secret value.
	rec = doWith(contribCookie, http.MethodGet, "/api/v1/proposals?status=pending", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("owner list: got %d, want 200", rec.Code)
	}
	var ownerList struct {
		Proposals []map[string]any `json:"proposals"`
		Total     int              `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ownerList); err != nil {
		t.Fatalf("unmarshal owner list: %v", err)
	}
	if ownerList.Total != 1 {
		t.Fatalf("owner pending list total = %d, want 1: %v", ownerList.Total, ownerList.Proposals)
	}
	if got := ownerList.Proposals[0]["proposed_value"]; got != secret {
		t.Fatalf("owner proposal value = %v, want %q", got, secret)
	}

	// A stranger's list must not contain the pending proposal at all.
	rec = doWith(strangerCookie, http.MethodGet, "/api/v1/proposals?status=pending", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("stranger list: got %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, secret) {
		t.Fatalf("stranger's proposal list leaked the pending proposal value: %s", body)
	}
	var strangerList struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(body), &strangerList); err != nil {
		t.Fatalf("unmarshal stranger list: %v", err)
	}
	if strangerList.Total != 0 {
		t.Fatalf("stranger pending list total = %d, want 0", strangerList.Total)
	}
}

func itoa(n int64) string {
	return time.Unix(n/1e9, int64(n%1e9)).Format("20060102150405.000")
}

// TestReviewProposal_ErrorClasses pins the BDB-011 error taxonomy at the API
// boundary: reviewing an unknown proposal ID returns 404 (an unknown resource
// is not confirmed as existing), while reviewing an already-reviewed proposal
// returns 409 (stale reviewer / no partial publication). A non-admin review
// attempt is 403.
func TestReviewProposal_ErrorClasses(t *testing.T) {
	db := openTestDB(t)
	h := NewAuthServer(db).Handler()
	stamp := itoa(time.Now().UnixNano())
	password := "s4-errclass-pass-123"

	admin := registerAuthUser(t, db, "rc_admin_"+stamp, string(auth.RoleAdministrator), password)
	_ = admin
	contrib := registerAuthUser(t, db, "rc_contrib_"+stamp, string(auth.RoleContributor), password)

	adminCookie := loginAuthCookie(t, h, admin.Username, password)
	contribCookie := loginAuthCookie(t, h, contrib.Username, password)

	workID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	unknownID := uuid.MustParse("99999999-9999-4999-8999-999999999999")

	doReview := func(cookie *http.Cookie, proposalID string, approve bool) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"approve": approve, "note": "e2e"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals/"+proposalID+"/review",
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// A contributor cannot review (403) — even on a nonexistent ID, the role
	// gate must fire before the resource gate.
	rec := doReview(contribCookie, unknownID.String(), true)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("contrib review unknown proposal: got %d, want 403: %s", rec.Code, rec.Body.String())
	}

	// Unknown proposal ID from an admin: 404, not 409.
	rec = doReview(adminCookie, unknownID.String(), true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("admin review unknown proposal: got %d, want 404: %s", rec.Code, rec.Body.String())
	}

	// A real proposal: first review 200, second review 409.
	submit, _ := json.Marshal(map[string]any{
		"entity_type":    "work",
		"entity_id":      workID.String(),
		"field_name":     "title",
		"proposed_value": "ErrorClass Probe " + stamp,
		"rationale":      "errclass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proposals", strings.NewReader(string(submit)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(contribCookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit proposal: got %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var submitted struct {
		ProposalID string `json:"proposal_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("unmarshal proposal: %v", err)
	}

	rec = doReview(adminCookie, submitted.ProposalID, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("first review: got %d, want 200: %s", rec.Code, rec.Body.String())
	}
	rec = doReview(adminCookie, submitted.ProposalID, false)
	if rec.Code != http.StatusConflict {
		t.Fatalf("second review: got %d, want 409: %s", rec.Code, rec.Body.String())
	}
}
