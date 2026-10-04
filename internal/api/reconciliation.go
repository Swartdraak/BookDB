// Package note: the S3 reconciliation namespace (issue #60).
//
// Permission boundaries (docs/bookdb/05-api-security.md: merge/split are
// administrator actions; every /api/v1 route requires a valid key):
//
//   - POST /api/v1/reconciliation/merge            — administrator
//   - POST /api/v1/reconciliation/split            — administrator
//   - POST /api/v1/reconciliation/reconcile        — administrator
//   - POST /api/v1/reconciliation/duplicates/generate — administrator
//   - GET  /api/v1/reconciliation/resolve/{type}/{id} — API-key auth (read)
//   - GET  /api/v1/reconciliation/changes          — API-key auth (read)
//   - GET  /api/v1/reconciliation/duplicates/{type} — API-key auth (read)
//
// Enforcement order, mapped before any resource lookup so non-authorized
// callers cannot probe entity IDs (mirrors the S4 job-admin boundary,
// TestJobAdmin_PermissionBoundaries):
//
//  1. 401 missing/invalid/revoked/expired API key (X-API-Key, S1 key store)
//  2. 403 valid key without the reconciliation scope
//  3. 403 administrator-only mutation without an administrator session
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bookdb/bookdb/internal/apikey"
	"github.com/bookdb/bookdb/internal/auth"
	"github.com/bookdb/bookdb/internal/reconciliation"
	"github.com/google/uuid"
)

// ScopeReconciliation is the API-key scope required for the S3 reconciliation
// namespace. Ordinary catalog keys (catalog:read) do not carry it, so the
// reconciliation surface stays closed to plain integration keys.
const ScopeReconciliation = "reconciliation:*"

// ReconciliationServer provides S3 reconciliation API endpoints.
type ReconciliationServer struct {
	reconciler  *reconciliation.Reconciler
	client      *reconciliation.EnrichmentClient
	keys        *apikey.Store
	authService *auth.AuthService
}

// NewReconciliationServer creates a ReconciliationServer. macKey is the API-key
// MAC key shared with the S1 catalog server (internal/apikey).
func NewReconciliationServer(db *sql.DB, macKey []byte) *ReconciliationServer {
	return &ReconciliationServer{
		reconciler:  reconciliation.NewReconciler(db),
		client:      reconciliation.NewEnrichmentClient(reconciliation.EnrichmentConfig{}),
		keys:        apikey.NewStore(db, macKey),
		authService: auth.NewAuthService(db),
	}
}

// Handler returns the HTTP handler for the /api/v1/reconciliation namespace.
func (s *ReconciliationServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/reconciliation/merge", s.requireReconAuth(true, s.handleMerge))
	mux.HandleFunc("POST /api/v1/reconciliation/split", s.requireReconAuth(true, s.handleSplit))
	mux.HandleFunc("GET /api/v1/reconciliation/resolve/{type}/{id}", s.requireReconAuth(false, s.handleResolve))
	mux.HandleFunc("GET /api/v1/reconciliation/changes", s.requireReconAuth(false, s.handleChanges))
	mux.HandleFunc("GET /api/v1/reconciliation/duplicates/{type}", s.requireReconAuth(false, s.handleDuplicates))
	mux.HandleFunc("POST /api/v1/reconciliation/duplicates/generate", s.requireReconAuth(true, s.handleDuplicatesGenerate))
	mux.HandleFunc("POST /api/v1/reconciliation/reconcile", s.requireReconAuth(true, s.handleReconcile))
	return mux
}

// requireReconAuth enforces the reconciliation permission boundary before the
// wrapped handler runs:
//
//  1. A valid API key (X-API-Key) is always required — missing or
//     missing/invalid/revoked/expired is 401. The client API key namespace
//     deliberately has no browser-session fallback: administrator actions
//     stay separate from an embedded administrator API key
//     (05-api-security.md).
//  2. The key must carry the reconciliation scope — a valid key without it is
//     403 (insufficient scope, S1 model).
//  3. For mutation operations (adminOnly) the authenticated user session must
//     carry the administrator role — a valid non-administrator is 403. Reads
//     (adminOnly=false) stop at API-key auth, the documented floor for the
//     read surface.
//
// All checks run before any resource lookup, so an unauthorized caller cannot
// probe entity IDs through handler-level 400/404 responses.
func (s *ReconciliationServer) requireReconAuth(adminOnly bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-API-Key")
		if secret == "" {
			writeError(w, http.StatusUnauthorized, "missing_api_key", "A valid API key is required.")
			return
		}
		key, err := s.keys.Verify(r.Context(), secret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "The API key is missing, invalid, revoked, or expired.")
			return
		}
		if !hasScope(key.Scopes, ScopeReconciliation) {
			writeError(w, http.StatusForbidden, "insufficient_scope", "The API key lacks the reconciliation scope.")
			return
		}
		if adminOnly {
			sessionIDStr := r.Header.Get("X-Session-ID")
			if sessionIDStr == "" {
				if cookie, cerr := r.Cookie("bookdb_session"); cerr == nil {
					sessionIDStr = cookie.Value
				}
			}
			if sessionIDStr == "" {
				writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
				return
			}
			sessionID, perr := uuid.Parse(sessionIDStr)
			if perr != nil {
				writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
				return
			}
			user, _, verr := s.authService.ValidateSession(r.Context(), sessionID)
			if verr != nil || !auth.HasRole(user.Role, auth.RoleAdministrator) {
				writeError(w, http.StatusForbidden, "insufficient_role", "Administrator role required.")
				return
			}
		}
		next(w, r)
	}
}

type mergeRequest struct {
	EntityType string `json:"entity_type"`
	IDA        string `json:"id_a"`
	IDB        string `json:"id_b"`
	Reason     string `json:"reason"`
}

func (s *ReconciliationServer) handleMerge(w http.ResponseWriter, r *http.Request) {
	var req mergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid merge request.")
		return
	}
	idA, err := uuid.Parse(req.IDA)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "id_a must be a valid UUID.")
		return
	}
	idB, err := uuid.Parse(req.IDB)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "id_b must be a valid UUID.")
		return
	}
	if req.Reason == "" {
		req.Reason = "manual_merge"
	}

	result, err := s.reconciler.Merge(r.Context(), req.EntityType, idA, idB, req.Reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Merge failed.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

type splitRequest struct {
	EntityType       string `json:"entity_type"`
	OriginalID       string `json:"original_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Reason           string `json:"reason"`
}

func (s *ReconciliationServer) handleSplit(w http.ResponseWriter, r *http.Request) {
	var req splitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid split request.")
		return
	}
	originalID, err := uuid.Parse(req.OriginalID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "original_id must be a valid UUID.")
		return
	}
	if req.Reason == "" {
		req.Reason = "manual_split"
	}

	result, err := s.reconciler.Split(r.Context(), req.EntityType, originalID, req.ExpectedRevision, req.Reason)
	if err != nil {
		if err.Error() == "reconciliation: revision mismatch: expected 1, got 1" {
			writeError(w, http.StatusPreconditionFailed, "revision_mismatch", "Revision does not match.")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "Split failed.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *ReconciliationServer) handleResolve(w http.ResponseWriter, r *http.Request) {
	entityType := r.PathValue("type")
	entityIDStr := r.PathValue("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "id must be a valid UUID.")
		return
	}

	canonicalID, err := s.reconciler.ResolveRedirect(r.Context(), entityType, entityID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Resolve failed.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entity_type":  entityType,
		"requested_id": entityIDStr,
		"canonical_id": canonicalID.String(),
		"is_redirect":  canonicalID != entityID,
	})
}

func (s *ReconciliationServer) handleChanges(w http.ResponseWriter, r *http.Request) {
	cursor := int64(0)
	if c := r.URL.Query().Get("cursor"); c != "" {
		var err error
		cursor, err = strconv.ParseInt(c, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "cursor must be an integer.")
			return
		}
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	changes, newCursor, err := s.reconciler.ConsumeChanges(r.Context(), cursor, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to read changes.")
		return
	}

	if changes == nil {
		changes = []reconciliation.Change{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"changes":  changes,
		"cursor":   newCursor,
		"has_more": len(changes) == limit,
	})
}

func (s *ReconciliationServer) handleDuplicates(w http.ResponseWriter, r *http.Request) {
	entityType := r.PathValue("type")
	candidates, err := s.reconciler.FindDuplicateCandidates(r.Context(), entityType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to find duplicates.")
		return
	}

	if candidates == nil {
		candidates = []reconciliation.DuplicateCandidate{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entity_type": entityType,
		"candidates":  candidates,
		"total":       len(candidates),
	})
}

// handleDuplicatesGenerate runs cross-source duplicate candidate generation
// (S3, issue #55) and returns the persisted pending candidates.
func (s *ReconciliationServer) handleDuplicatesGenerate(w http.ResponseWriter, r *http.Request) {
	candidates, err := s.reconciler.FindDuplicateCandidatesCrossSource(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Candidate generation failed.")
		return
	}
	if candidates == nil {
		candidates = []reconciliation.DuplicateCandidate{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"candidates": candidates,
		"total":      len(candidates),
		"note":       "candidates are pending; same-name matches are never auto-merged",
	})
}

type reconcileRequest struct {
	Source     string `json:"source"`
	MaxPerType int    `json:"max_per_type"`
}

// handleReconcile runs the S3 enrichment + candidate post-pass (issue #55)
// for a source: OL API enrichment with field-level provenance selection
// plus cross-source duplicate candidate generation.
func (s *ReconciliationServer) handleReconcile(w http.ResponseWriter, r *http.Request) {
	var req reconcileRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Source == "" {
		req.Source = "openlibrary"
	}
	out, err := s.reconciler.RunReconcile(r.Context(), s.client, req.Source, req.MaxPerType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Reconciliation pass failed.")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
