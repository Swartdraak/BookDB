package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bookdb/bookdb/internal/auth"
)

// AuthServer provides S4 authentication and moderation API endpoints.
type AuthServer struct {
	authService *auth.AuthService
}

// NewAuthServer creates an AuthServer.
func NewAuthServer(db *sql.DB) *AuthServer {
	return &AuthServer{authService: auth.NewAuthService(db)}
}

// Handler returns the HTTP handler for the /api/v1/auth namespace.
func (s *AuthServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", s.handleMe)
	mux.HandleFunc("POST /api/v1/proposals", s.handleSubmitProposal)
	mux.HandleFunc("GET /api/v1/proposals", s.handleListProposals)
	mux.HandleFunc("POST /api/v1/proposals/{id}/review", s.handleReviewProposal)
	mux.HandleFunc("PUT /api/v1/users/{id}/role", s.handleSetRole)
	mux.HandleFunc("POST /api/v1/users/{id}/disable", s.handleDisableUser)
	return mux
}

type registerRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (s *AuthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid registration request.")
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Username, email, and password are required.")
		return
	}

	user, err := s.authService.Register(r.Context(), req.Username, req.Email, req.Password, req.DisplayName)
	if err != nil {
		writeError(w, http.StatusConflict, "registration_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *AuthServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid login request.")
		return
	}

	user, session, err := s.authService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password.")
		return
	}

	// Set session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "bookdb_session",
		Value:    session.SessionID.String(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user":    user,
		"session": session,
	})
}

func (s *AuthServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.Header.Get("X-Session-ID")
	if sessionIDStr == "" {
		cookie, err := r.Cookie("bookdb_session")
		if err == nil {
			sessionIDStr = cookie.Value
		}
	}
	if sessionIDStr != "" {
		if sessionID, err := parseUUID(sessionIDStr); err == nil {
			_ = s.authService.Logout(r.Context(), sessionID)
		}
	}

	// Clear cookie.
	http.SetCookie(w, &http.Cookie{
		Name:   "bookdb_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *AuthServer) handleMe(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.Header.Get("X-Session-ID")
	if sessionIDStr == "" {
		cookie, err := r.Cookie("bookdb_session")
		if err == nil {
			sessionIDStr = cookie.Value
		}
	}
	if sessionIDStr == "" {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}
	sessionID, err := parseUUID(sessionIDStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "Invalid session.")
		return
	}
	user, _, err := s.authService.ValidateSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "Session is invalid or expired.")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

type proposalRequest struct {
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	FieldName     string `json:"field_name"`
	ProposedValue any    `json:"proposed_value"`
	Rationale     string `json:"rationale"`
}

func (s *AuthServer) handleSubmitProposal(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}
	if !auth.HasRole(user.Role, auth.RoleContributor) {
		writeError(w, http.StatusForbidden, "insufficient_role", "Contributor role required.")
		return
	}

	var req proposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid proposal request.")
		return
	}
	entityID, err := parseUUID(req.EntityID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id must be a valid UUID.")
		return
	}
	valueJSON, _ := json.Marshal(req.ProposedValue)

	proposal, err := s.authService.SubmitProposal(r.Context(), user.UserID, req.EntityType, entityID, req.FieldName, valueJSON, req.Rationale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to submit proposal.")
		return
	}

	writeJSON(w, http.StatusCreated, proposal)
}

func (s *AuthServer) handleListProposals(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}

	status := r.URL.Query().Get("status")
	// Non-admins can only see their own proposals.
	if !auth.HasRole(user.Role, auth.RoleAdministrator) {
		// Filter by user in the query.
		proposals, err := s.authService.ListProposals(r.Context(), status, 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Failed to list proposals.")
			return
		}
		// Filter to user's own proposals.
		var filtered []any
		for _, p := range proposals {
			if p.UserID == user.UserID || auth.HasRole(user.Role, auth.RoleAdministrator) {
				filtered = append(filtered, p)
			}
		}
		if filtered == nil {
			filtered = []any{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"proposals": filtered, "total": len(filtered)})
		return
	}

	proposals, err := s.authService.ListProposals(r.Context(), status, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to list proposals.")
		return
	}
	if proposals == nil {
		proposals = []auth.Proposal{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals, "total": len(proposals)})
}

type reviewRequest struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
}

func (s *AuthServer) handleReviewProposal(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}
	if !auth.HasRole(user.Role, auth.RoleAdministrator) {
		writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
		return
	}

	proposalIDStr := r.PathValue("id")
	proposalID, err := parseUUID(proposalIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid proposal ID.")
		return
	}

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid review request.")
		return
	}

	if err := s.authService.ReviewProposal(r.Context(), proposalID, user.UserID, req.Approve, req.Note); err != nil {
		writeError(w, http.StatusConflict, "review_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

type roleRequest struct {
	Role string `json:"role"`
}

func (s *AuthServer) handleSetRole(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}
	if !auth.HasRole(user.Role, auth.RoleAdministrator) {
		writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
		return
	}

	targetIDStr := r.PathValue("id")
	targetID, err := parseUUID(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID.")
		return
	}

	var req roleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid role request.")
		return
	}

	if err := s.authService.SetUserRole(r.Context(), targetID, auth.Role(req.Role), user.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "role_updated"})
}

func (s *AuthServer) handleDisableUser(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "missing_session", "Authentication required.")
		return
	}
	if !auth.HasRole(user.Role, auth.RoleAdministrator) {
		writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
		return
	}

	targetIDStr := r.PathValue("id")
	targetID, err := parseUUID(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid user ID.")
		return
	}

	if err := s.authService.DisableUser(r.Context(), targetID, user.UserID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "user_disabled"})
}

