package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TestSessionCookie_Attributes guards the bookdb_session cookie against the
// CodeQL findings (go/cookie-secure-not-set, go/cookie-httponly-not-set):
// the login Set-Cookie must carry Secure, and the logout clear-cookie must
// carry HttpOnly + Secure + SameSite=Strict so it matches the original
// cookie's attributes and is actually deleted by the browser.
func TestSessionCookie_Attributes(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Disposable user for this test (unique per run to avoid fixture residue).
	username := fmt.Sprintf("cookie59_%d", time.Now().UnixNano())
	password := "cookie59-test-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO bookdb.users (username, email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, 'Cookie Test', 'reader', 'active')`,
		username, username+"@test.invalid", string(hash))
	if err != nil {
		t.Fatalf("create disposable user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.users WHERE username = $1`, username)
	})

	// (The logout handler only revokes a session when its id parses as a
	// UUID; a garbage value exercises the no-session path while the
	// clear-cookie response is still produced.)

	h := NewAuthServer(db).Handler()

	doAuth := func(method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		var reader *strings.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			reader = strings.NewReader(string(b))
		} else {
			reader = strings.NewReader("")
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	sessionCookie := func(rec *httptest.ResponseRecorder) *http.Cookie {
		t.Helper()
		var found *http.Cookie
		for _, c := range rec.Result().Cookies() {
			if c.Name == "bookdb_session" {
				found = c
			}
		}
		if found == nil {
			t.Fatalf("no bookdb_session Set-Cookie in response: headers=%v", rec.Header())
		}
		return found
	}

	tests := []struct {
		name    string
		method  string
		path    string
		body    any
		cookies []*http.Cookie
	}{
		{
			name:   "login",
			method: http.MethodPost,
			path:   "/api/v1/auth/login",
			body:   loginRequest{Username: username, Password: password},
		},
		{
			name:    "logout",
			method:  http.MethodPost,
			path:    "/api/v1/auth/logout",
			cookies: []*http.Cookie{{Name: "bookdb_session", Value: "not-a-uuid"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doAuth(tc.method, tc.path, tc.body, tc.cookies...)
			cookie := sessionCookie(rec)

			if !cookie.Secure {
				t.Errorf("%s: bookdb_session cookie missing Secure", tc.name)
			}
			if !cookie.HttpOnly {
				t.Errorf("%s: bookdb_session cookie missing HttpOnly", tc.name)
			}
			if cookie.SameSite != http.SameSiteStrictMode {
				t.Errorf("%s: bookdb_session cookie SameSite = %v, want Strict", tc.name, cookie.SameSite)
			}
			if cookie.Path != "/" {
				t.Errorf("%s: bookdb_session cookie Path = %q, want /", tc.name, cookie.Path)
			}
			if tc.name == "login" && cookie.MaxAge <= 0 {
				t.Errorf("login: MaxAge = %d, want > 0", cookie.MaxAge)
			}
			if tc.name == "logout" && (cookie.MaxAge >= 0 || cookie.Value != "") {
				t.Errorf("logout: clear-cookie must be empty value with MaxAge<0, got value=%q MaxAge=%d", cookie.Value, cookie.MaxAge)
			}
		})
	}
}
