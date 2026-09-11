// Package ingestion implements the S2 Open Library snapshot ingestion pipeline.
//
// It reads a pinned bibliographic dump subset, normalizes records into the
// canonical Work/Expression/Edition/Person model, stores source records with
// provenance, and publishes outbox events for the search projection.
//
// The pipeline is resumable: progress is checkpointed in the ingestion_jobs
// table, and reprocessing the same snapshot is idempotent.
package ingestion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SourceName is the canonical name for the Open Library source.
const SourceName = "openlibrary"

// Snapshot identifies a specific Open Library dump subset.
type Snapshot struct {
	ID          string    `json:"id"`
	Hash        string    `json:"hash"`
	RecordCount int       `json:"record_count"`
	RetrievedAt time.Time `json:"retrieved_at"`
}

// Record is a single normalized bibliographic record from the source.
type Record struct {
	SourceKey   string   `json:"source_key"`  // e.g. "OL1234567W"
	SourceType  string   `json:"source_type"` // "work", "edition", "author"
	Title       string   `json:"title"`
	Authors     []string `json:"authors,omitempty"`
	Language    string   `json:"language,omitempty"`
	ISBNs       []string `json:"isbns,omitempty"`
	Publisher   string   `json:"publisher,omitempty"`
	PublishDate string   `json:"publish_date,omitempty"`
	Format      string   `json:"format,omitempty"` // print, ebook, audio, etc.
	RawJSON     []byte   `json:"-"`                // original source record
}

// JobState tracks the progress of an ingestion run.
type JobState struct {
	JobID        uuid.UUID `json:"job_id"`
	SourceName   string    `json:"source_name"`
	SnapshotID   string    `json:"snapshot_id"`
	Status       string    `json:"status"`
	TotalRecords int64     `json:"total_records"`
	Processed    int64     `json:"processed"`
	Accepted     int64     `json:"accepted"`
	Unchanged    int64     `json:"unchanged"`
	Rejected     int64     `json:"rejected"`
	Quarantined  int64     `json:"quarantined"`
	Checkpoint   []byte    `json:"checkpoint,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Ingestor performs the Open Library ingestion pipeline.
type Ingestor struct {
	db *sql.DB
}

// NewIngestor creates an Ingestor backed by the given database.
func NewIngestor(db *sql.DB) *Ingestor {
	return &Ingestor{db: db}
}

// StartJob creates a new ingestion job and returns its state.
func (i *Ingestor) StartJob(ctx context.Context, snapshot Snapshot) (*JobState, error) {
	jobID := uuid.New()
	_, err := i.db.ExecContext(ctx, `
		INSERT INTO bookdb.ingestion_jobs (job_id, source_name, snapshot_id, status, total_records, started_at)
		VALUES ($1, $2, $3, 'running', $4, now())`,
		jobID, SourceName, snapshot.ID, snapshot.RecordCount)
	if err != nil {
		return nil, fmt.Errorf("ingestion: start job: %w", err)
	}

	// Record the source manifest.
	_, err = i.db.ExecContext(ctx, `
		INSERT INTO bookdb.source_manifests (source_name, snapshot_id, snapshot_hash, record_count, retrieved_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (source_name, snapshot_id) DO UPDATE SET
			snapshot_hash = EXCLUDED.snapshot_hash,
			record_count = EXCLUDED.record_count`,
		SourceName, snapshot.ID, snapshot.Hash, snapshot.RecordCount, snapshot.RetrievedAt)
	if err != nil {
		return nil, fmt.Errorf("ingestion: record manifest: %w", err)
	}

	return &JobState{
		JobID:        jobID,
		SourceName:   SourceName,
		SnapshotID:   snapshot.ID,
		Status:       "running",
		TotalRecords: int64(snapshot.RecordCount),
	}, nil
}

// ProcessRecord normalizes and stores a single record. Returns the outcome.
func (i *Ingestor) ProcessRecord(ctx context.Context, jobID uuid.UUID, rec Record) (string, error) {
	// Compute a content hash for idempotency.
	contentHash := hashRecord(rec)

	// Check if this exact record was already processed (idempotency).
	var existingStatus string
	err := i.db.QueryRowContext(ctx, `
		SELECT status FROM bookdb.source_records
		WHERE source_name = $1 AND source_key = $2 AND content_hash = $3`,
		SourceName, rec.SourceKey, contentHash).Scan(&existingStatus)
	if err == nil {
		// Already processed with same content: unchanged.
		i.incrementCounter(ctx, jobID, "unchanged")
		return "unchanged", nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("ingestion: check existing: %w", err)
	}

	// Validate the record.
	if err := validateRecord(rec); err != nil {
		i.incrementCounter(ctx, jobID, "rejected")
		return "rejected", nil
	}

	// Store the source record with provenance.
	rawJSON, _ := json.Marshal(rec)
	_, err = i.db.ExecContext(ctx, `
		INSERT INTO bookdb.source_records (source_name, source_key, content_hash, raw_json, status)
		VALUES ($1, $2, $3, $4, 'accepted')
		ON CONFLICT (source_name, source_key) DO UPDATE SET
			content_hash = EXCLUDED.content_hash,
			raw_json = EXCLUDED.raw_json,
			status = 'accepted'`,
		SourceName, rec.SourceKey, contentHash, rawJSON)
	if err != nil {
		return "", fmt.Errorf("ingestion: store source record: %w", err)
	}

	// Publish an outbox event for the search projection.
	eventType := "record.accepted"
	if rec.SourceType == "work" {
		eventType = "work.accepted"
	} else if rec.SourceType == "edition" {
		eventType = "edition.accepted"
	} else if rec.SourceType == "author" {
		eventType = "person.accepted"
	}
	payload, _ := json.Marshal(map[string]string{
		"source_key":  rec.SourceKey,
		"source_type": rec.SourceType,
		"title":       rec.Title,
	})
	_, err = i.db.ExecContext(ctx, `
		INSERT INTO bookdb.outbox (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)`,
		rec.SourceType, uuid.New(), eventType, payload)
	if err != nil {
		return "", fmt.Errorf("ingestion: publish outbox: %w", err)
	}

	i.incrementCounter(ctx, jobID, "accepted")
	return "accepted", nil
}

// UpdateCheckpoint saves the current processing position.
func (i *Ingestor) UpdateCheckpoint(ctx context.Context, jobID uuid.UUID, processed int64, checkpoint []byte) error {
	_, err := i.db.ExecContext(ctx, `
		UPDATE bookdb.ingestion_jobs
		SET processed = $2, checkpoint = $3, updated_at = now()
		WHERE job_id = $1`,
		jobID, processed, checkpoint)
	return err
}

// CompleteJob marks the job as completed.
func (i *Ingestor) CompleteJob(ctx context.Context, jobID uuid.UUID) error {
	_, err := i.db.ExecContext(ctx, `
		UPDATE bookdb.ingestion_jobs
		SET status = 'completed', finished_at = now(), updated_at = now()
		WHERE job_id = $1`,
		jobID)
	return err
}

// FailJob marks the job as failed with an error message.
func (i *Ingestor) FailJob(ctx context.Context, jobID uuid.UUID, errMsg string) error {
	_, err := i.db.ExecContext(ctx, `
		UPDATE bookdb.ingestion_jobs
		SET status = 'failed', error_message = $2, finished_at = now(), updated_at = now()
		WHERE job_id = $1`,
		jobID, errMsg)
	return err
}

// GetJob retrieves the current state of a job.
func (i *Ingestor) GetJob(ctx context.Context, jobID uuid.UUID) (*JobState, error) {
	var js JobState
	var checkpoint []byte
	err := i.db.QueryRowContext(ctx, `
		SELECT job_id, source_name, snapshot_id, status, total_records,
		       processed, accepted, unchanged, rejected, quarantined,
		       checkpoint, COALESCE(error_message, '')
		FROM bookdb.ingestion_jobs WHERE job_id = $1`,
		jobID).Scan(
		&js.JobID, &js.SourceName, &js.SnapshotID, &js.Status,
		&js.TotalRecords, &js.Processed, &js.Accepted, &js.Unchanged,
		&js.Rejected, &js.Quarantined, &checkpoint, &js.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	js.Checkpoint = checkpoint
	return &js, nil
}

// VerifyAccounting checks that accepted + unchanged + rejected + quarantined = processed.
func (i *Ingestor) VerifyAccounting(ctx context.Context, jobID uuid.UUID) error {
	var accepted, unchanged, rejected, quarantined, processed int64
	err := i.db.QueryRowContext(ctx, `
		SELECT accepted, unchanged, rejected, quarantined, processed
		FROM bookdb.ingestion_jobs WHERE job_id = $1`,
		jobID).Scan(&accepted, &unchanged, &rejected, &quarantined, &processed)
	if err != nil {
		return err
	}
	sum := accepted + unchanged + rejected + quarantined
	if sum != processed {
		return fmt.Errorf("ingestion: accounting mismatch: %d+ %d+ %d+ %d = %d, expected %d",
			accepted, unchanged, rejected, quarantined, sum, processed)
	}
	return nil
}

func (i *Ingestor) incrementCounter(ctx context.Context, jobID uuid.UUID, field string) {
	// Use a safe column name to prevent SQL injection.
	switch field {
	case "accepted", "unchanged", "rejected", "quarantined":
		_, _ = i.db.ExecContext(ctx, `
			UPDATE bookdb.ingestion_jobs
			SET `+field+` = `+field+` + 1, processed = processed + 1, updated_at = now()
			WHERE job_id = $1`, jobID)
	}
}

func hashRecord(rec Record) string {
	h := sha256.New()
	h.Write([]byte(rec.SourceKey))
	h.Write([]byte(rec.SourceType))
	h.Write([]byte(rec.Title))
	h.Write(rec.RawJSON)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func validateRecord(rec Record) error {
	if rec.SourceKey == "" {
		return fmt.Errorf("record has no source key")
	}
	if rec.SourceType == "" {
		return fmt.Errorf("record has no source type")
	}
	if rec.SourceType == "work" && rec.Title == "" {
		return fmt.Errorf("work record has no title")
	}
	return nil
}

// ParseOpenLibraryLine parses a single line from an Open Library dump file.
// The format is JSON lines (one JSON object per line).
func ParseOpenLibraryLine(line string) (*Record, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, fmt.Errorf("parse line: %w", err)
	}

	rec := &Record{RawJSON: []byte(line)}

	// Determine source type and key.
	if key, ok := raw["key"].(string); ok {
		rec.SourceKey = strings.TrimPrefix(key, "/works/")
		rec.SourceKey = strings.TrimPrefix(rec.SourceKey, "/editions/")
		rec.SourceKey = strings.TrimPrefix(rec.SourceKey, "/authors/")
	}

	// Determine type from the key prefix.
	if key, ok := raw["key"].(string); ok {
		switch {
		case strings.HasPrefix(key, "/works/"):
			rec.SourceType = "work"
			rec.SourceKey = strings.TrimPrefix(key, "/works/")
		case strings.HasPrefix(key, "/editions/"):
			rec.SourceType = "edition"
			rec.SourceKey = strings.TrimPrefix(key, "/editions/")
		case strings.HasPrefix(key, "/authors/"):
			rec.SourceType = "author"
			rec.SourceKey = strings.TrimPrefix(key, "/authors/")
		default:
			rec.SourceType = "unknown"
		}
	}

	// Extract title.
	if title, ok := raw["title"].(string); ok {
		rec.Title = title
	}

	// Extract language.
	if lang, ok := raw["language"].(string); ok {
		rec.Language = lang
	} else if langs, ok := raw["languages"].([]any); ok && len(langs) > 0 {
		if l, ok := langs[0].(map[string]any); ok {
			if code, ok := l["key"].(string); ok {
				rec.Language = strings.TrimPrefix(code, "/languages/")
			}
		}
	}

	// Extract ISBNs.
	if isbn13, ok := raw["isbn_13"].([]any); ok {
		for _, v := range isbn13 {
			if s, ok := v.(string); ok {
				rec.ISBNs = append(rec.ISBNs, s)
			}
		}
	}
	if isbn10, ok := raw["isbn_10"].([]any); ok {
		for _, v := range isbn10 {
			if s, ok := v.(string); ok {
				rec.ISBNs = append(rec.ISBNs, s)
			}
		}
	}

	// Extract publisher.
	if pub, ok := raw["publisher"].(string); ok {
		rec.Publisher = pub
	} else if pubs, ok := raw["publishers"].([]any); ok && len(pubs) > 0 {
		if p, ok := pubs[0].(map[string]any); ok {
			if name, ok := p["name"].(string); ok {
				rec.Publisher = name
			}
		}
	}

	// Extract publish date.
	if pd, ok := raw["publish_date"].(string); ok {
		rec.PublishDate = pd
	}

	// Extract format.
	if phys, ok := raw["physical_format"].(string); ok {
		rec.Format = phys
	}

	return rec, nil
}

// ReadSnapshot reads records from an io.Reader (JSON lines format).
func ReadSnapshot(r io.Reader) (<-chan *Record, <-chan error) {
	records := make(chan *Record, 100)
	errs := make(chan error, 1)

	go func() {
		defer close(records)
		defer close(errs)

		buf := make([]byte, 64*1024)
		var line strings.Builder

		for {
			n, err := r.Read(buf)
			if n > 0 {
				line.WriteString(string(buf[:n]))
				for {
					idx := strings.IndexByte(line.String(), '\n')
					if idx < 0 {
						break
					}
					l := line.String()[:idx]
					line.Reset()
					line.WriteString(line.String()[idx+1:])

					rec, parseErr := ParseOpenLibraryLine(l)
					if parseErr != nil {
						errs <- parseErr
						return
					}
					if rec != nil {
						records <- rec
					}
				}
			}
			if err != nil {
				if err != io.EOF {
					errs <- err
				}
				return
			}
		}
	}()

	return records, errs
}
