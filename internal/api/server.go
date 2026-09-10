// Package api implements the S1 key-protected canonical catalog HTTP API.
//
// All /api/v1 catalog operations require a valid API key (X-API-Key header).
// Missing/invalid/revoked/expired keys return 401; a valid identity lacking
// the required scope returns 403; unknown resources return 404. Exceeding
// the shared rate limit returns 429 with Retry-After; an unavailable Valkey
// backend returns 503.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bookdb/bookdb/internal/apikey"
	"github.com/bookdb/bookdb/internal/catalog"
	"github.com/bookdb/bookdb/internal/ratelimit"
)

// ScopeRead is the scope required for catalog read operations.
const ScopeRead = "catalog:read"

// Server is the S1 catalog API server.
type Server struct {
	repo    *catalog.SQLRepository
	keys    *apikey.Store
	limiter *ratelimit.Limiter
}

// NewServer constructs a catalog API server. limiter may be nil to disable
// rate limiting (useful for tests that do not have a Valkey backend).
func NewServer(db *sql.DB, macKey []byte, limiter *ratelimit.Limiter) *Server {
	return &Server{
		repo:    catalog.NewSQLRepository(db),
		keys:    apikey.NewStore(db, macKey),
		limiter: limiter,
	}
}

// Handler returns the HTTP handler for the /api/v1 catalog namespace.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/works/{id}", s.requireKey(ScopeRead, s.handleGetWork))
	mux.HandleFunc("GET /api/v1/works/{id}/editions", s.requireKey(ScopeRead, s.handleListEditions))
	mux.HandleFunc("GET /api/v1/expressions/{id}", s.requireKey(ScopeRead, s.handleGetExpression))
	mux.HandleFunc("GET /api/v1/editions/{id}", s.requireKey(ScopeRead, s.handleGetEdition))
	mux.HandleFunc("GET /api/v1/people/{id}", s.requireKey(ScopeRead, s.handleGetPerson))
	mux.HandleFunc("GET /api/v1/people/{id}/works", s.requireKey(ScopeRead, s.handleListWorksForPerson))
	mux.HandleFunc("GET /api/v1/organizations/{id}", s.requireKey(ScopeRead, s.handleGetOrganization))
	mux.HandleFunc("GET /api/v1/resolve", s.requireKey(ScopeRead, s.handleResolve))
	return mux
}

// requireKey wraps a handler with API key authentication, scope checks, and
// shared rate limiting.
func (s *Server) requireKey(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-API-Key")
		if secret == "" {
			writeError(w, http.StatusUnauthorized, "missing_api_key", "A valid API key is required.")
			return
		}
		key, err := s.keys.Verify(r.Context(), secret)
		if err != nil {
			// Missing, invalid, revoked, or expired keys all map to 401.
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "The API key is missing, invalid, revoked, or expired.")
			return
		}
		if !hasScope(key.Scopes, scope) {
			writeError(w, http.StatusForbidden, "insufficient_scope", "The API key lacks the required scope.")
			return
		}

		// Shared rate limiting via Valkey.
		if s.limiter != nil {
			allowed, err := s.limiter.Allow(r.Context(), key.KeyID)
			if err != nil {
				// Valkey unavailable: documented 503.
				writeError(w, http.StatusServiceUnavailable, "rate_limit_unavailable", "Rate limiting backend is unavailable.")
				return
			}
			if !allowed {
				retryAfter := s.limiter.RetryAfter()
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Rate limit exceeded. Retry after the specified interval.")
				return
			}
		}

		s.keys.TouchLastUsed(r.Context(), key.KeyID)
		next(w, r)
	}
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

func (s *Server) handleGetWork(w http.ResponseWriter, r *http.Request) {
	work, err := s.repo.GetWork(r.Context(), r.PathValue("id"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Work not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to load work.")
		return
	}
	writeJSON(w, http.StatusOK, work)
}

func (s *Server) handleListEditions(w http.ResponseWriter, r *http.Request) {
	editions, err := s.repo.ListEditionsForWork(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to list editions.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"editions": editions})
}

func (s *Server) handleGetExpression(w http.ResponseWriter, r *http.Request) {
	expr, err := s.repo.GetExpression(r.Context(), r.PathValue("id"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Expression not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to load expression.")
		return
	}
	writeJSON(w, http.StatusOK, expr)
}

func (s *Server) handleGetEdition(w http.ResponseWriter, r *http.Request) {
	edition, err := s.repo.GetEdition(r.Context(), r.PathValue("id"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Edition not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to load edition.")
		return
	}
	writeJSON(w, http.StatusOK, edition)
}

func (s *Server) handleGetPerson(w http.ResponseWriter, r *http.Request) {
	person, err := s.repo.GetPerson(r.Context(), r.PathValue("id"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Person not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to load person.")
		return
	}
	writeJSON(w, http.StatusOK, person)
}

func (s *Server) handleListWorksForPerson(w http.ResponseWriter, r *http.Request) {
	works, err := s.repo.ListWorksForPerson(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to list works.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"works": works})
}

func (s *Server) handleGetOrganization(w http.ResponseWriter, r *http.Request) {
	org, err := s.repo.GetOrganization(r.Context(), r.PathValue("id"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Organization not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to load organization.")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	value := r.URL.Query().Get("value")
	if namespace == "" || value == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Both namespace and value are required.")
		return
	}
	res, err := s.repo.Resolve(r.Context(), namespace, value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to resolve identifier.")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// errorBody is the consistent error object.
type errorBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func writeError(w http.ResponseWriter, status int, typ, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Type: typ, Title: titleFor(status), Status: status, Detail: detail})
}

func titleFor(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusTooManyRequests:
		return "Too Many Requests"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	default:
		return "Error"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
