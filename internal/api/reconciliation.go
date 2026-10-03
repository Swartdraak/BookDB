package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bookdb/bookdb/internal/reconciliation"
	"github.com/google/uuid"
)

// ReconciliationServer provides S3 reconciliation API endpoints.
type ReconciliationServer struct {
	reconciler *reconciliation.Reconciler
	client     *reconciliation.EnrichmentClient
}

// NewReconciliationServer creates a ReconciliationServer.
func NewReconciliationServer(db *sql.DB) *ReconciliationServer {
	return &ReconciliationServer{
		reconciler: reconciliation.NewReconciler(db),
		client:     reconciliation.NewEnrichmentClient(reconciliation.EnrichmentConfig{}),
	}
}

// Handler returns the HTTP handler for the /api/v1/reconciliation namespace.
func (s *ReconciliationServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/reconciliation/merge", s.handleMerge)
	mux.HandleFunc("POST /api/v1/reconciliation/split", s.handleSplit)
	mux.HandleFunc("GET /api/v1/reconciliation/resolve/{type}/{id}", s.handleResolve)
	mux.HandleFunc("GET /api/v1/reconciliation/changes", s.handleChanges)
	mux.HandleFunc("GET /api/v1/reconciliation/duplicates/{type}", s.handleDuplicates)
	mux.HandleFunc("POST /api/v1/reconciliation/duplicates/generate", s.handleDuplicatesGenerate)
	mux.HandleFunc("POST /api/v1/reconciliation/reconcile", s.handleReconcile)
	return mux
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
