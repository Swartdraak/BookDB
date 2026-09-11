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
	SessionID  uuid.UUID `json:"session_id"`
	UserID     uuid.UUID `json:"user_id"`
	CSRFToken  string    `json:"csrf_token"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// AuthService handles authentication and session management.
type AuthService struct {
	db          *sql.DB
	sessionTTL  time.Duration
}

// NewAuthService creates an AuthService.
func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db, sessionTTL: 24 * time.Hour}
}

// Register creates a new user account.
func (a *AuthService) Register(ctx context.Context, username, email, password, displayName string) (*User, error) {
	if len(password) < 8 {
		return nil, fmt.Errorf("auth: password must be at least 8 characters")
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
		userID      uuid.UUID
		passwordHash string
		u           User
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT user_id, password_hash, username, email, display_name, role, status
		FROM bookdb.users WHERE username = $1`, username).
		Scan(&userID, &passwordHash, &u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("auth: invalid credentials")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("auth: login: %w", err)
	}
	if u.Status != "active" {
		return nil, nil, fmt.Errorf("auth: account is %s", u.Status)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("auth: invalid credentials")
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

	// Update last login.
	_, _ = a.db.ExecContext(ctx, `
		UPDATE bookdb.users SET last_login_at = now() WHERE user_id = $1`, userID)

	session := &Session{
		SessionID: sessionID,
		UserID:    userID,
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
		s       Session
		u       User
	)
	err := a.db.QueryRowContext(ctx, `
		SELECT s.session_id, s.user_id, s.csrf_token, s.expires_at,
		       u.username, u.email, u.display_name, u.role, u.status
		FROM bookdb.sessions s
		JOIN bookdb.users u ON u.user_id = s.user_id
		WHERE s.session_id = $1 AND s.revoked_at IS NULL`, sessionID).
		Scan(&s.SessionID, &s.UserID, &s.CSRFToken, &s.ExpiresAt,
			&u.Username, &u.Email, &u.DisplayName, &u.Role, &u.Status)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("auth: invalid session")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("auth: validate session: %w", err)
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, nil, fmt.Errorf("auth: session expired")
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
		return fmt.Errorf("auth: proposal not found or already reviewed")
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

