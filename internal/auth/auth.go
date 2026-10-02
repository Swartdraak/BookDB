// Package auth implements S4 local authentication, session management,
// RBAC, and the moderation workflow.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role is a user's access level.
type Role string

const (
	RoleReader        Role = "reader"
	RoleContributor   Role = "contributor"
	RoleAdministrator Role = "administrator"
)

// User is an authenticated account.
type User struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        Role      `json:"role"`
	Status      string    `json:"status"`
}

// Session is an active browser session.
type Session struct {
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      Role      `json:"-"` // role the session was issued with (privilege-change rotation)
	CSRFToken string    `json:"csrf_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ErrInvalidCredentials reports that login failed for any reason (unknown
// user, wrong password, or a non-active account). All three cases are
// deliberately indistinguishable to the caller: the API layer maps this to
// a single 401 so a disabled account cannot be probed through login.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrOIDCIdentityTaken reports that the (issuer, subject) pair is already
// bound to a different user (BDB-010): OIDC subject strings are only unique
// per issuer, and one user cannot adopt an identity another user already
// holds. The API layer maps this to 409.
var ErrOIDCIdentityTaken = errors.New("auth: oidc identity already linked to another user")

// ErrOIDCIdentityLinked reports that the user already has this exact
// (issuer, subject) pair linked; re-linking is a no-op conflict. The API
// layer maps this to 409.
var ErrOIDCIdentityLinked = errors.New("auth: oidc identity already linked to this user")

// ErrStaleRole reports that a session was issued with a role that no longer
// matches the account (BDB-007/009): the privilege change is real but the
// session predates it, so it must be rotated with a fresh login instead of
// silently carrying the new privileges. The API layer maps this to 401.
var ErrStaleRole = errors.New("auth: session role is stale")

// ErrWeakPassword reports that a registration password is below the
// minimum length (BDB-007). The API layer maps this to 409.
var ErrWeakPassword = errors.New("auth: password must be at least 8 characters")

// ErrProposalNotFound reports that a review targeted a proposal that does
// not exist. The API layer maps this to 404 (unknown resources are not
// revealed as 409, so a stale reviewer cannot probe for proposal IDs).
var ErrProposalNotFound = errors.New("auth: proposal not found")

// ErrProposalConflict reports that a review targeted a proposal whose
// status is no longer pending (already approved/rejected/withdrawn) or
// whose revision no longer matches. The API layer maps this to 409.
var ErrProposalConflict = errors.New("auth: proposal conflict")

// AuthService handles authentication and session management.
type AuthService struct {
	db         *sql.DB
	sessionTTL time.Duration
}

// NewAuthService creates an AuthService.
func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db, sessionTTL: 24 * time.Hour}
}

// Register creates a new user account.
func (a *AuthService) Register(ctx context.Context, username, email, password, displayName string) (*User, error) {
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("auth: hash password: %w", err)
	}

	var user User
	err = a.db.QueryRowContext(ctx, `
		INSERT INTO bookdb.users (username, email, password_hash, display_name, role, status)
		VALUES ($1, $2, $3, $4, 'reader', 'active')
		RETURNING user_id, username, email, display_name, role, status`,
		username, email, string(hash), displayName).
		Scan(&user.UserID, &user.Username, &user.Email, &user.DisplayName, &user.Role, &user.Status)
	if err != nil {
		return nil, fmt.Errorf("auth: register: %w", err)
	}
	return &user, nil
}

// Login verifies credentials and creates a session.
func (a *AuthService) Login(ctx context.Context, username, password string) (*User, *Session, error) {
	var (
		userID       uuid.UUID
		passwordHash string
		u            User
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT user_id, password_hash, username, email, display_name, role, status
		FROM bookdb.users WHERE username = $1`, username).
		Scan(&userID, &passwordHash, &u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status)
	if err == sql.ErrNoRows {
		return nil, nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, fmt.Errorf("auth: login: %w", err)
	}
	if u.Status != "active" {
		return nil, nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}
	u.UserID = userID

	// Create session.
	sessionID := uuid.New()
	csrfToken := generateToken(32)
	expiresAt := time.Now().Add(a.sessionTTL)

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO bookdb.sessions (session_id, user_id, csrf_token, expires_at)
		VALUES ($1, $2, $3, $4)`,
		sessionID, userID, csrfToken, expiresAt)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: create session: %w", err)
	}

	// Record the role the session was issued with (BDB-007/009: a privilege
	// change made outside the session must not silently extend the
	// session's privileges).
	if _, err := a.db.ExecContext(ctx,
		`UPDATE bookdb.sessions SET issued_role = $2 WHERE session_id = $1`,
		sessionID, string(u.Role)); err != nil {
		return nil, nil, fmt.Errorf("auth: record session role: %w", err)
	}

	// Update last login.
	_, _ = a.db.ExecContext(ctx, `
		UPDATE bookdb.users SET last_login_at = now() WHERE user_id = $1`, userID)

	session := &Session{
		SessionID: sessionID,
		UserID:    userID,
		Role:      u.Role,
		CSRFToken: csrfToken,
		ExpiresAt: expiresAt,
	}
	return &u, session, nil
}

// Logout revokes a session.
func (a *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE bookdb.sessions SET revoked_at = now() WHERE session_id = $1`, sessionID)
	return err
}

// ValidateSession checks if a session is valid and returns the user.
func (a *AuthService) ValidateSession(ctx context.Context, sessionID uuid.UUID) (*User, *Session, error) {
	var (
		s Session
		u User
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT s.session_id, s.user_id, s.csrf_token, s.expires_at,
		       u.username, u.email, u.display_name, u.role, u.status,
		       s.issued_role
		FROM bookdb.sessions s
		JOIN bookdb.users u ON u.user_id = s.user_id
		WHERE s.session_id = $1 AND s.revoked_at IS NULL`, sessionID).
		Scan(&s.SessionID, &s.UserID, &s.CSRFToken, &s.ExpiresAt,
			&u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status,
			&s.Role)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("auth: invalid session")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("auth: validate session: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, nil, fmt.Errorf("auth: session expired")
	}
	// A role change made outside the session must not silently extend the
	// session's privileges (BDB-007/009): a stale-issued session is
	// rejected so the user rotates with a fresh login. Sessions predating
	// the column (NULL issued_role) fall back to the account's current
	// role, preserving existing behavior.
	if s.Role == "" {
		s.Role = u.Role
	} else if s.Role != u.Role {
		return nil, nil, ErrStaleRole
	}
	u.UserID = s.UserID
	return &u, &s, nil
}

// HasRole checks if a user has at least the specified role.
func HasRole(userRole, required Role) bool {
	order := map[Role]int{
		RoleReader:        1,
		RoleContributor:   2,
		RoleAdministrator: 3,
	}
	return order[userRole] >= order[required]
}

// Proposal is a user-submitted correction.
type Proposal struct {
	ProposalID    uuid.UUID       `json:"proposal_id"`
	UserID        uuid.UUID       `json:"user_id"`
	EntityType    string          `json:"entity_type"`
	EntityID      uuid.UUID       `json:"entity_id"`
	FieldName     string          `json:"field_name"`
	ProposedValue json.RawMessage `json:"proposed_value"`
	Rationale     string          `json:"rationale"`
	Status        string          `json:"status"`
	Revision      int64           `json:"revision"`
	CreatedAt     time.Time       `json:"created_at"`
}

// SubmitProposal creates a new proposal.
func (a *AuthService) SubmitProposal(ctx context.Context, userID uuid.UUID, entityType string, entityID uuid.UUID, fieldName string, proposedValue json.RawMessage, rationale string) (*Proposal, error) {
	var p Proposal
	err := a.db.QueryRowContext(ctx, `
		INSERT INTO bookdb.correction_proposals (user_id, entity_type, entity_id, field_name, proposed_value, rationale)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING proposal_id, user_id, entity_type, entity_id, field_name, proposed_value, rationale, status, revision, created_at`,
		userID, entityType, entityID, fieldName, proposedValue, rationale).
		Scan(&p.ProposalID, &p.UserID, &p.EntityType, &p.EntityID, &p.FieldName, &p.ProposedValue, &p.Rationale, &p.Status, &p.Revision, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: submit proposal: %w", err)
	}
	return &p, nil
}

// ReviewProposal approves or rejects a proposal.
func (a *AuthService) ReviewProposal(ctx context.Context, proposalID uuid.UUID, reviewerID uuid.UUID, approve bool, note string) error {
	status := "rejected"
	if approve {
		status = "approved"
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: begin review: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE bookdb.correction_proposals
		SET status = $2, reviewed_by = $3, reviewed_at = now(), review_note = $4, updated_at = now()
		WHERE proposal_id = $1 AND status = 'pending'`,
		proposalID, status, reviewerID, note)
	if err != nil {
		return fmt.Errorf("auth: review proposal: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Distinguish "proposal does not exist" (404 class) from "proposal
		// already reviewed" (409 class) — BDB-011 requires a stale reviewer
		// to get a conflict, while an unknown ID must not be confirmed as
		// existing. No row was written in either case, so the transaction is
		// rolled back by the deferred Rollback (no partial publication).
		var exists int
		if err := tx.QueryRowContext(ctx,
			`SELECT count(*) FROM bookdb.correction_proposals WHERE proposal_id = $1`, proposalID).
			Scan(&exists); err != nil {
			return fmt.Errorf("auth: review proposal existence: %w", err)
		}
		if exists == 0 {
			return ErrProposalNotFound
		}
		return ErrProposalConflict
	}

	// Audit the action.
	action := "proposal.rejected"
	if approve {
		action = "proposal.approved"
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO bookdb.audit_events (user_id, action, detail)
		VALUES ($1, $2, $3)`,
		reviewerID, action, mustJSON(map[string]string{"proposal_id": proposalID.String()}))
	if err != nil {
		return fmt.Errorf("auth: audit: %w", err)
	}

	return tx.Commit()
}

// ListProposals returns proposals filtered by status.
func (a *AuthService) ListProposals(ctx context.Context, status string, limit int) ([]Proposal, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT proposal_id, user_id, entity_type, entity_id, field_name, proposed_value, rationale, status, revision, created_at
		FROM bookdb.correction_proposals`
	var args []any
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("auth: list proposals: %w", err)
	}
	defer rows.Close()

	var proposals []Proposal
	for rows.Next() {
		var p Proposal
		if err := rows.Scan(&p.ProposalID, &p.UserID, &p.EntityType, &p.EntityID, &p.FieldName, &p.ProposedValue, &p.Rationale, &p.Status, &p.Revision, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan proposal: %w", err)
		}
		proposals = append(proposals, p)
	}
	return proposals, rows.Err()
}

// SetUserRole changes a user's role (admin only).
func (a *AuthService) SetUserRole(ctx context.Context, userID uuid.UUID, newRole Role, adminID uuid.UUID) error {
	if !HasRole(RoleAdministrator, newRole) {
		return fmt.Errorf("auth: invalid role %q", newRole)
	}
	result, err := a.db.ExecContext(ctx, `
		UPDATE bookdb.users SET role = $2, updated_at = now() WHERE user_id = $1`,
		userID, string(newRole))
	if err != nil {
		return fmt.Errorf("auth: set role: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("auth: user not found")
	}
	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO bookdb.audit_events (user_id, action, entity_type, entity_id, detail)
		VALUES ($1, 'user.role_changed', 'user', $2, $3)`,
		adminID, userID, mustJSON(map[string]string{"new_role": string(newRole)}))
	return nil
}

// DisableUser disables a user account (admin only).
func (a *AuthService) DisableUser(ctx context.Context, userID uuid.UUID, adminID uuid.UUID) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE bookdb.users SET status = 'disabled', updated_at = now() WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("auth: disable user: %w", err)
	}
	// Revoke all sessions.
	_, _ = a.db.ExecContext(ctx, `
		UPDATE bookdb.sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO bookdb.audit_events (user_id, action, entity_type, entity_id)
		VALUES ($1, 'user.disabled', 'user', $2)`, adminID, userID)
	return nil
}

// LinkOIDCSubject binds an OIDC (issuer, subject) identity to an
// authenticated user (BDB-010). The identity is the PAIR: the same subject
// at a different issuer is a distinct identity and may be linked by another
// user. An identity already bound to a different user cannot be adopted;
// re-linking the same pair to the same user is a conflict.
func (a *AuthService) LinkOIDCSubject(ctx context.Context, userID uuid.UUID, issuer, subject string) error {
	if issuer == "" || subject == "" {
		return fmt.Errorf("auth: oidc issuer and subject are required")
	}
	var owner uuid.UUID
	err := a.db.QueryRowContext(ctx, `
		SELECT user_id FROM bookdb.users
		WHERE oidc_issuer = $1 AND oidc_subject = $2`, issuer, subject).
		Scan(&owner)
	switch {
	case err == sql.ErrNoRows:
		// Unclaimed identity: bind it to this user.
	case err != nil:
		return fmt.Errorf("auth: lookup oidc identity: %w", err)
	case owner == userID:
		return ErrOIDCIdentityLinked
	default:
		return ErrOIDCIdentityTaken
	}
	res, err := a.db.ExecContext(ctx, `
		UPDATE bookdb.users
		SET oidc_issuer = $2, oidc_subject = $3, updated_at = now()
		WHERE user_id = $1 AND oidc_subject IS NULL`,
		userID, issuer, subject)
	if err != nil {
		return fmt.Errorf("auth: link oidc subject: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// The user already carries some other (issuer, subject) binding:
		// a local account holds at most one external identity.
		return ErrOIDCIdentityLinked
	}
	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO bookdb.audit_events (user_id, action, entity_type, entity_id, detail)
		VALUES ($1, 'user.oidc_linked', 'user', $2, $3)`,
		userID, userID, mustJSON(map[string]string{"issuer": issuer}))
	return nil
}

// AuthMiddleware wraps a handler with session-based authentication.
func (a *AuthService) AuthMiddleware(requiredRole Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionIDStr := r.Header.Get("X-Session-ID")
		if sessionIDStr == "" {
			cookie, err := r.Cookie("bookdb_session")
			if err == nil {
				sessionIDStr = cookie.Value
			}
		}
		if sessionIDStr == "" {
			writeAuthError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
			return
		}
		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid_session", "Invalid session.")
			return
		}
		user, session, err := a.ValidateSession(r.Context(), sessionID)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid_session", "Session is invalid or expired.")
			return
		}
		if !HasRole(user.Role, requiredRole) {
			writeAuthError(w, http.StatusForbidden, "insufficient_role", "Insufficient permissions.")
			return
		}
		// Store user in context.
		ctx := context.WithValue(r.Context(), userContextKey, user)
		ctx = context.WithValue(ctx, sessionContextKey, session)
		next(w, r.WithContext(ctx))
	}
}

// userFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}

// SessionFromContext extracts the session from the request context.
func SessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(sessionContextKey).(*Session)
	return s
}

type contextKey string

const (
	userContextKey    contextKey = "bookdb_user"
	sessionContextKey contextKey = "bookdb_session"
)

func writeAuthError(w http.ResponseWriter, status int, typ, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   typ,
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	})
}

func generateToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// VerifyPassword checks a password against a bcrypt hash.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SHA256Hex returns the hex-encoded SHA-256 hash of a string.
func SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// NormalizeUsername normalizes a username for storage.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
