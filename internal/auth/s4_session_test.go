package auth

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// cleanTestSchema resets the shared disposable test database to a clean
// bookdb-schema state (drops + recreates schema and migration state) and
// re-applies migrations. Other packages share this database; the reset is
// serialized across packages by the database package's TestResetLockKey
// advisory lock (issue #68), so concurrent `go test` package execution
// cannot interleave a reset with another package's queries.
func cleanTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	// Clean schema + migrations in ONE advisory-locked critical section
	// (issue #68): the reset and the migration DDL cannot interleave with
	// another package's queries.
	if _, err := database.PrepareCleanAndMigrate(context.Background(), db, "bookdb"); err != nil {
		t.Fatalf("prepare clean test database: %v", err)
	}
	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM bookdb.users`).Scan(&count); err != nil {
		t.Fatalf("query users after reset: %v", err)
	}
	if count != 0 {
		t.Fatalf("users table after reset = %d rows, want 0", count)
	}
}

// openSharedTestDB connects to the shared disposable test database and
// guarantees a fully-migrated bookdb schema. It performs a clean reset +
// migration in ONE advisory-locked critical section
// (database.PrepareCleanAndMigrate, TestResetLockKey), matching the api
// package's openTestDB, so the schema state NEVER depends on what another
// package left behind in the -p 1 sequence (or on a subset run where no
// other package ran first).
func openSharedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("BOOKDB_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookdb@127.0.0.1:5432/bookdb?sslmode=disable"
	}
	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dsn, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := database.PrepareCleanAndMigrate(ctx, db, "bookdb"); err != nil {
		t.Fatalf("prepare clean test database: %v", err)
	}
	return db
}

// newS4User inserts a user with an explicit role and a real bcrypt hash so
// Login works end-to-end.
func newS4User(t *testing.T, db *sql.DB, username, role, password string) *User {
	t.Helper()
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var u User
	err = db.QueryRowContext(ctx, `
		INSERT INTO bookdb.users (username, email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, 'S4 Test', $4, 'active')
		RETURNING user_id, username, email, display_name, role, status`,
		username, username+"@test.invalid", string(hash), role).
		Scan(&u.UserID, &u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status)
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.correction_proposals WHERE user_id = $1`, u.UserID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM bookdb.users WHERE user_id = $1`, u.UserID)
	})
	return &u
}

// loginSession logs in via the real service path and returns the session.
func loginSession(t *testing.T, service *AuthService, username, password string) (*User, *Session) {
	t.Helper()
	u, s, err := service.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return u, s
}

// TestRoleChange_RevokesUnrotatedSessions pins BDB-007/009: a privilege
// change (role upgrade) must not silently extend the privileges of a
// session issued under the old role. The session row records the role it
// was issued with; ValidateSession rejects the session once the stored
// role no longer matches the account, and a fresh login rotates into a
// session carrying the new role.
func TestRoleChange_RevokesUnrotatedSessions(t *testing.T) {
	db := openSharedTestDB(t)
	cleanTestSchema(t, db)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-role-change-pass-1"

	user := newS4User(t, db, fmt.Sprintf("rotate_%d", time.Now().UnixNano()), string(RoleReader), password)
	_, session := loginSession(t, service, user.Username, password)

	// Precondition: the original role does not satisfy contributor.
	if HasRole(user.Role, RoleContributor) {
		t.Fatalf("precondition: role %q must not satisfy contributor", user.Role)
	}
	if _, _, err := service.ValidateSession(ctx, session.SessionID); err != nil {
		t.Fatalf("precondition: issued session should validate: %v", err)
	}

	// Promote the account outside the session.
	if err := service.SetUserRole(ctx, user.UserID, RoleContributor, user.UserID); err != nil {
		t.Fatalf("set role: %v", err)
	}

	// The pre-change session must now be rejected (stale role).
	if _, _, err := service.ValidateSession(ctx, session.SessionID); err == nil {
		t.Fatalf("expected the unrotated session to be rejected after the role change")
	}

	// A fresh login rotates into a session carrying the new role.
	user2, session2 := loginSession(t, service, user.Username, password)
	if user2.Role != RoleContributor {
		t.Fatalf("new login role = %q, want contributor", user2.Role)
	}
	if session2.SessionID == session.SessionID {
		t.Fatalf("login must issue a rotated session, got the same session id")
	}
	if _, _, err := service.ValidateSession(ctx, session2.SessionID); err != nil {
		t.Fatalf("rotated session must validate: %v", err)
	}
}

// TestDisabledAccount_LosesExistingSession pins BDB-009: disabling a user
// revokes all of their sessions immediately and blocks new logins.
func TestDisabledAccount_LosesExistingSession(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-disable-pass-1"

	user := newS4User(t, db, fmt.Sprintf("disable_%d", time.Now().UnixNano()), string(RoleReader), password)
	_, session := loginSession(t, service, user.Username, password)
	if _, _, err := service.ValidateSession(ctx, session.SessionID); err != nil {
		t.Fatalf("precondition: session should validate: %v", err)
	}

	if err := service.DisableUser(ctx, user.UserID, user.UserID); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, _, err := service.ValidateSession(ctx, session.SessionID); err == nil {
		t.Fatalf("disabled user's session must be rejected, but it validated")
	}
	if _, _, err := service.Login(ctx, user.Username, password); err == nil {
		t.Fatalf("disabled user must not be able to log in")
	}
}

// TestLinkOIDCSubject pins the BDB-010 account-linking rules:
//   - linking an unknown (issuer, subject) to an authenticated user stores
//     the binding on that user
//   - a user cannot link a subject that is already bound to ANOTHER user
//   - re-linking the same (issuer, subject) to the same user is a conflict
//   - a different issuer with the same subject is a distinct identity and
//     may be linked elsewhere
func TestLinkOIDCSubject(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-oidc-pass-1"

	owner := newS4User(t, db, fmt.Sprintf("oidc_owner_%d", time.Now().UnixNano()), string(RoleReader), password)
	stranger := newS4User(t, db, fmt.Sprintf("oidc_stranger_%d", time.Now().UnixNano()), string(RoleReader), password)

	issuer := "https://idp.test.local/realms/bookdb"
	subject := fmt.Sprintf("sub-%d", time.Now().UnixNano())

	// Owner adopts the identity.
	if err := service.LinkOIDCSubject(ctx, owner.UserID, issuer, subject); err != nil {
		t.Fatalf("link to owner: %v", err)
	}
	if got := userOIDCSubject(t, db, owner.UserID); got != subject {
		t.Fatalf("owner oidc_subject = %q, want %q", got, subject)
	}
	if got := userOIDCIssuer(t, db, owner.UserID); got != issuer {
		t.Fatalf("owner oidc_issuer = %q, want %q", got, issuer)
	}

	// A stranger must not be able to steal the identity.
	if err := service.LinkOIDCSubject(ctx, stranger.UserID, issuer, subject); err == nil {
		t.Fatalf("linking a taken (issuer, subject) to another user must fail")
	}

	// Re-linking the same identity to the same user is a conflict.
	if err := service.LinkOIDCSubject(ctx, owner.UserID, issuer, subject); err == nil {
		t.Fatalf("re-linking an already-linked identity must fail")
	}

	// A different issuer with the same subject is a distinct identity.
	if err := service.LinkOIDCSubject(ctx, stranger.UserID, "https://other.test.local", subject); err != nil {
		t.Fatalf("linking a distinct (issuer, subject) pair must succeed: %v", err)
	}
}

func userOIDCSubject(t *testing.T, db *sql.DB, userID uuid.UUID) string {
	t.Helper()
	var s string
	err := db.QueryRowContext(context.Background(),
		`SELECT oidc_subject FROM bookdb.users WHERE user_id = $1`, userID).Scan(&s)
	if err != nil {
		t.Fatalf("read oidc_subject: %v", err)
	}
	return s
}

func userOIDCIssuer(t *testing.T, db *sql.DB, userID uuid.UUID) string {
	t.Helper()
	var s string
	err := db.QueryRowContext(context.Background(),
		`SELECT oidc_issuer FROM bookdb.users WHERE user_id = $1`, userID).Scan(&s)
	if err != nil {
		t.Fatalf("read oidc_issuer: %v", err)
	}
	return s
}

// TestReviewProposal_UnknownReturnsNotFound pins the not-found vs stale
// conflict distinction for BDB-011: reviewing a proposal that does not
// exist is a 404-class error, while reviewing an already-reviewed
// proposal is a 409-class conflict. The API layer maps these to their
// documented statuses.
func TestReviewProposal_UnknownReturnsNotFound(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-review-pass-1"

	admin := newS4User(t, db, fmt.Sprintf("review_admin_%d", time.Now().UnixNano()), string(RoleAdministrator), password)

	// Unknown proposal id: not-found class, NOT a conflict.
	err := service.ReviewProposal(ctx, uuid.New(), admin.UserID, true, "nope")
	if err != ErrProposalNotFound {
		t.Fatalf("review of unknown proposal = %v, want ErrProposalNotFound", err)
	}

	// A real, already-reviewed proposal: conflict class.
	other := newS4User(t, db, fmt.Sprintf("reviewer_%d", time.Now().UnixNano()), string(RoleContributor), password)

	// A fresh work (inserted + cleaned up by this test) so the publication
	// target never depends on fixture state or cross-package side effects.
	workID := uuid.New()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO bookdb.works (work_id, canonical_title, normalized_title) VALUES ($1, 'Review Test Work', 'review test work')`,
		workID); err != nil {
		t.Fatalf("insert test work: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.works WHERE work_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.change_feed WHERE entity_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.outbox WHERE aggregate_id = $1`, workID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM bookdb.canonical_revisions WHERE entity_id = $1`, workID)
	})

	proposal, err := service.SubmitProposal(ctx, other.UserID, "work",
		workID,
		"title", []byte(`"Stale Title"`), "typo")
	if err != nil {
		t.Fatalf("submit proposal: %v", err)
	}

	if err := service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, true, "first"); err != nil {
		t.Fatalf("first review: %v", err)
	}
	err = service.ReviewProposal(ctx, proposal.ProposalID, admin.UserID, false, "second")
	if err != ErrProposalConflict {
		t.Fatalf("second review = %v, want ErrProposalConflict", err)
	}
}

// TestLinkOIDCSplitIdentity pins the (issuer, subject) split that makes
// OIDC account linking safe: the same subject string at two different
// issuers is a DISTINCT identity. Linking the other issuer's subject must
// succeed and must not be blocked by the first binding.
func TestLinkOIDCSplitIdentity(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-split-pass-1"

	a := newS4User(t, db, fmt.Sprintf("split_a_%d", time.Now().UnixNano()), string(RoleReader), password)
	b := newS4User(t, db, fmt.Sprintf("split_b_%d", time.Now().UnixNano()), string(RoleReader), password)

	issuerA := "https://issuer-a.test.local/realms/bookdb"
	issuerB := "https://issuer-b.test.local/realms/bookdb"
	subject := fmt.Sprintf("shared-sub-%d", time.Now().UnixNano())

	if err := service.LinkOIDCSubject(ctx, a.UserID, issuerA, subject); err != nil {
		t.Fatalf("link a: %v", err)
	}
	// Same subject, different issuer: a distinct identity, must succeed.
	if err := service.LinkOIDCSubject(ctx, b.UserID, issuerB, subject); err != nil {
		t.Fatalf("linking the same subject at a different issuer must succeed: %v", err)
	}
	if got := userOIDCIssuer(t, db, b.UserID); got != issuerB {
		t.Fatalf("b oidc_issuer = %q, want %q", got, issuerB)
	}
	if got := userOIDCSubject(t, db, b.UserID); got != subject {
		t.Fatalf("b oidc_subject = %q, want %q", got, subject)
	}
}

// TestLoginDisabledAccount_Indistinguishable pins the login error
// taxonomy: a disabled account, an unknown user, and a wrong password must
// all produce the SAME error (ErrInvalidCredentials) so the API can return
// one 401 without revealing account state.
func TestLoginDisabledAccount_Indistinguishable(t *testing.T) {
	db := openSharedTestDB(t)
	service := NewAuthService(db)
	ctx := context.Background()
	password := "s4-disabled-pass-1"

	user := newS4User(t, db, fmt.Sprintf("indist_%d", time.Now().UnixNano()), string(RoleReader), password)
	if err := service.DisableUser(ctx, user.UserID, user.UserID); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if _, _, err := service.Login(ctx, user.Username, password); err != ErrInvalidCredentials {
		t.Fatalf("disabled login = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := service.Login(ctx, fmt.Sprintf("no_such_user_%d", time.Now().UnixNano()), password); err != ErrInvalidCredentials {
		t.Fatalf("unknown-user login = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := service.Login(ctx, user.Username, "wrong-password-1"); err != ErrInvalidCredentials {
		t.Fatalf("wrong-password login = %v, want ErrInvalidCredentials", err)
	}
}
