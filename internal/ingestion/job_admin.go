// S4-ADMIN job administration (issue #30, BDB-011): expose schedules,
// trigger/pause/resume/quarantine and clear permission boundaries for
// ingestion jobs.
//
// State machine (single source of truth, enforced by Ingestor.TransitionJob):
//
//	running   -> completed | failed | paused | quarantined
//	paused    -> running (resume) | failed | quarantined
//	failed    -> running (retry) | quarantined
//	completed -> (terminal)
//	quarantined -> (terminal until operator re-queue; out of scope)
//
// The quarantine transition carries a required reason (BDB-011: quarantine
// with health/reason context) which is recorded on the job and in the audit
// trail.
package ingestion

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Job transition actions.
const (
	JobActionPause      = "pause"
	JobActionResume     = "resume"
	JobActionRetry      = "retry"
	JobActionQuarantine = "quarantine"
)

// Job transitions and their allowed source states.
var jobTransitions = map[string]map[string]string{
	JobActionPause:      {"running": "paused"},
	JobActionResume:     {"paused": "running"},
	JobActionRetry:      {"failed": "running"},
	JobActionQuarantine: {"running": "quarantined", "paused": "quarantined", "failed": "quarantined"},
}

// ErrJobNotFound reports that a job ID does not exist.
var ErrJobNotFound = errors.New("ingestion: job not found")

// ErrJobState reports that a transition is not valid from the job's current
// state.
var ErrJobState = errors.New("ingestion: invalid job state transition")

// ErrQuarantineReason reports that a quarantine action carried no reason.
var ErrQuarantineReason = errors.New("ingestion: quarantine requires a reason")

// JobQueryer is the narrow database interface the JobAdmin operates on, so
// unit tests can exercise the state machine without a live PostgreSQL.
type JobQueryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// JobAdmin performs validated state transitions on ingestion jobs.
type JobAdmin struct {
	db JobQueryer
}

// NewJobAdmin creates a JobAdmin backed by the given database.
func NewJobAdmin(db JobQueryer) *JobAdmin {
	return &JobAdmin{db: db}
}

// TransitionJob validates and applies a state transition to a job. The
// transition is guarded by the current state (UPDATE ... WHERE status =
// <from>), so concurrent transitions resolve to a single winner; the loser
// gets ErrJobState. Quarantine requires a non-empty reason, which is
// persisted on the job (error_message doubles as the quarantine reason for
// a quarantined job) and returned in the updated JobState.
func (a *JobAdmin) TransitionJob(ctx context.Context, jobID uuid.UUID, action, reason string) (*JobState, error) {
	fromTo, ok := jobTransitions[action]
	if !ok {
		return nil, fmt.Errorf("ingestion: unknown job action %q: %w", action, ErrJobState)
	}
	if action == JobActionQuarantine && reason == "" {
		return nil, ErrQuarantineReason
	}

	// Read the current state first so the error message can name it.
	var current string
	if err := a.db.QueryRowContext(ctx,
		`SELECT status FROM bookdb.ingestion_jobs WHERE job_id = $1`, jobID).
		Scan(&current); err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	} else if err != nil {
		return nil, fmt.Errorf("ingestion: read job state: %w", err)
	}
	to, allowed := fromTo[current]
	if !allowed {
		return nil, fmt.Errorf("ingestion: cannot %s a job in state %q: %w", action, current, ErrJobState)
	}

	var setExpr string
	var args []any
	switch action {
	case JobActionQuarantine:
		setExpr = "status = $2, error_message = $3, updated_at = now()"
		args = []any{jobID, to, reason}
	case JobActionResume, JobActionRetry:
		// Re-entry into running records when the job (re)started.
		setExpr = "status = $2, started_at = COALESCE(started_at, now()), updated_at = now()"
		args = []any{jobID, to}
	default:
		setExpr = "status = $2, updated_at = now()"
		args = []any{jobID, to}
	}
	// The guarded UPDATE appends the from-state as the LAST placeholder
	// ($len(args)+1) so the argument list stays valid for every branch.
	result, err := a.db.ExecContext(ctx,
		`UPDATE bookdb.ingestion_jobs SET `+setExpr+
			` WHERE job_id = $1 AND status = $`+itoaArg(len(args)+1),
		append(args, current)...)
	if err != nil {
		return nil, fmt.Errorf("ingestion: transition job: %w", err)
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return nil, fmt.Errorf("ingestion: concurrent transition: job left state %q: %w", current, ErrJobState)
	}
	return a.GetJob(ctx, jobID)
}

// GetJobWithAdmin fetches a job with the quarantine reason surfaced as
// ErrorMessage (the column doubles as the reason for quarantined jobs).
func (a *JobAdmin) GetJob(ctx context.Context, jobID uuid.UUID) (*JobState, error) {
	// Reuse the Ingestor query shape via the narrow interface.
	var js JobState
	var checkpoint []byte
	err := a.db.QueryRowContext(ctx, `
		SELECT job_id, source_name, snapshot_id, status, total_records,
		       processed, accepted, unchanged, rejected, quarantined,
		       checkpoint, COALESCE(error_message, '')
		FROM bookdb.ingestion_jobs WHERE job_id = $1`, jobID).Scan(
		&js.JobID, &js.SourceName, &js.SnapshotID, &js.Status,
		&js.TotalRecords, &js.Processed, &js.Accepted, &js.Unchanged,
		&js.Rejected, &js.Quarantined, &checkpoint, &js.ErrorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ingestion: get job: %w", err)
	}
	js.Checkpoint = checkpoint
	return &js, nil
}

// ListJobs returns the most recent jobs, optionally filtered by source and
// status. For administrators: read access only — no mutations.
func (a *JobAdmin) ListJobs(ctx context.Context, sourceName, status string, limit int) ([]JobState, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `
		SELECT job_id, source_name, snapshot_id, status, total_records,
		       processed, accepted, unchanged, rejected, quarantined,
		       checkpoint, COALESCE(error_message, '')
		FROM bookdb.ingestion_jobs`
	var conds []string
	var args []any
	if sourceName != "" {
		conds = append(conds, "source_name = $1")
		args = append(args, sourceName)
	}
	if status != "" {
		conds = append(conds, "status = $"+itoaArg(len(args)+1))
		args = append(args, status)
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT $" + itoaArg(len(args)+1)
	args = append(args, limit)

	type queryer interface {
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	}
	rows, err := a.db.(queryer).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ingestion: list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []JobState
	for rows.Next() {
		var js JobState
		var checkpoint []byte
		if err := rows.Scan(&js.JobID, &js.SourceName, &js.SnapshotID, &js.Status,
			&js.TotalRecords, &js.Processed, &js.Accepted, &js.Unchanged,
			&js.Rejected, &js.Quarantined, &checkpoint, &js.ErrorMessage); err != nil {
			return nil, fmt.Errorf("ingestion: scan job: %w", err)
		}
		js.Checkpoint = checkpoint
		jobs = append(jobs, js)
	}
	if jobs == nil {
		jobs = []JobState{}
	}
	return jobs, rows.Err()
}

// JobAdmin audit: record a transition in the durable audit trail. Called by
// the API layer with the actor's user ID after a successful transition.
func (a *JobAdmin) AuditTransition(ctx context.Context, actorID uuid.UUID, jobID uuid.UUID, action, reason string) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO bookdb.audit_events (user_id, action, detail)
		VALUES ($1, $2, $3)`,
		actorID, "job."+action, mustJobAuditDetail(jobID, action, reason))
	if err != nil {
		return fmt.Errorf("ingestion: audit transition: %w", err)
	}
	return nil
}

func mustJobAuditDetail(jobID uuid.UUID, action, reason string) string {
	detail := map[string]string{"job_id": jobID.String(), "action": action, "at": time.Now().UTC().Format(time.RFC3339)}
	if reason != "" {
		detail["reason"] = reason
	}
	b, _ := json.Marshal(detail)
	return string(b)
}

// itoaArg formats a 1-based argument index for a SQL placeholder.
func itoaArg(n int) string { return strconv.Itoa(n) }
