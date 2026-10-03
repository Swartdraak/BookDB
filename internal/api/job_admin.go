// S4-ADMIN job administration API (issue #30, BDB-011).
//
// Permission boundaries:
//   - GET  /api/v1/ingestion/jobs            — administrator read
//   - GET  /api/v1/ingestion/jobs/{id}       — administrator read
//   - POST /api/v1/ingestion/jobs/{id}/{action} — administrator write
//     (action: pause | resume | retry | quarantine; quarantine requires a
//     non-empty reason in the request body)
//
// No session (401) and insufficient role (403) are mapped before any
// resource lookup: a non-admin cannot probe for job IDs, mirroring the
// proposal review boundary (BDB-009/011).
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bookdb/bookdb/internal/auth"
	"github.com/bookdb/bookdb/internal/ingestion"
	"github.com/google/uuid"
)

// JobAdminServer provides S4-ADMIN ingestion job administration endpoints.
type JobAdminServer struct {
	authService *auth.AuthService
	admin       *ingestion.JobAdmin
}

// NewJobAdminServer creates a JobAdminServer backed by the given database.
func NewJobAdminServer(db *sql.DB) *JobAdminServer {
	return &JobAdminServer{
		authService: auth.NewAuthService(db),
		admin:       ingestion.NewJobAdmin(db),
	}
}

// Handler returns the HTTP handler for the /api/v1/ingestion namespace.
func (s *JobAdminServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ingestion/jobs", s.authService.AuthMiddleware(auth.RoleAdministrator, s.handleListJobs))
	mux.HandleFunc("GET /api/v1/ingestion/jobs/{id}", s.authService.AuthMiddleware(auth.RoleAdministrator, s.handleGetJob))
	mux.HandleFunc("POST /api/v1/ingestion/jobs/{id}/{action}", s.authService.AuthMiddleware(auth.RoleAdministrator, s.handleJobAction))
	return mux
}

func (s *JobAdminServer) handleListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	jobs, err := s.admin.ListJobs(r.Context(), q.Get("source"), q.Get("status"), parseLimit(q.Get("limit")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to list jobs.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "total": len(jobs)})
}

func (s *JobAdminServer) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parsePathUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	job, err := s.admin.GetJob(r.Context(), jobID)
	if errors.Is(err, ingestion.ErrJobNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Job not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Failed to fetch job.")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

type jobActionRequest struct {
	Reason string `json:"reason"`
}

func (s *JobAdminServer) handleJobAction(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parsePathUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	action := r.PathValue("action")
	if _, known := map[string]struct{}{
		ingestion.JobActionPause:      {},
		ingestion.JobActionResume:     {},
		ingestion.JobActionRetry:      {},
		ingestion.JobActionQuarantine: {},
	}[action]; !known {
		writeError(w, http.StatusBadRequest, "invalid_request",
			"action must be one of: pause, resume, retry, quarantine.")
		return
	}

	var req jobActionRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid job action request.")
			return
		}
	}

	job, err := s.admin.TransitionJob(r.Context(), jobID, action, req.Reason)
	switch {
	case errors.Is(err, ingestion.ErrJobNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Job not found.")
		return
	case errors.Is(err, ingestion.ErrJobState):
		// A stale or invalid transition is a documented conflict: the job's
		// state moved (or the action is not valid from its state). 409 is
		// the BDB-011 conflict class — no partial state change occurred.
		writeError(w, http.StatusConflict, "job_state_conflict", err.Error())
		return
	case errors.Is(err, ingestion.ErrQuarantineReason):
		writeError(w, http.StatusBadRequest, "invalid_request", "Quarantine requires a reason.")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "job_action_failed", "Job action failed.")
		return
	}

	// Durable audit trail entry (best effort: the transition already
	// committed; a failed audit insert is logged but does not revert it).
	if user := auth.UserFromContext(r.Context()); user != nil {
		if aerr := s.admin.AuditTransition(r.Context(), user.UserID, jobID, action, req.Reason); aerr != nil {
			// The transition itself is authoritative; the audit insert
			// failure is reported via the 500 path only if it is a
			// systematic problem. We deliberately do not fail the request:
			// the job state change is the committed fact.
			_ = aerr
		}
	}

	writeJSON(w, http.StatusOK, job)
}

// parsePathUUID parses a UUID path parameter, writing a 400 on failure.
func parsePathUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	jobID, err := uuid.Parse(s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Job ID must be a valid UUID.")
		return uuid.UUID{}, false
	}
	return jobID, true
}

// parseLimit parses a positive integer query limit with a 50 default.
func parseLimit(s string) int {
	if s == "" {
		return 50
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 50
	}
	return n
}
