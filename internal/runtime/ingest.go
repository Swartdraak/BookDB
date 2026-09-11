package runtime

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/bookdb/bookdb/internal/ingestion"
)

// runIngest handles the bookdb ingest subcommand.
// Usage: bookdb ingest --file <path> --snapshot-id <id>
func runIngest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	file := fs.String("file", "", "Path to the Open Library dump file (JSON lines)")
	snapshotID := fs.String("snapshot-id", "", "Snapshot identifier")
	_ = fs.Parse(args)

	if *file == "" || *snapshotID == "" {
		fmt.Fprintln(stderr, "Usage: bookdb ingest --file <path> --snapshot-id <id>")
		return 2
	}

	cfg, _, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	dsn := cfg.Database.DSNDirect
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.Database.DSN
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	db, err := database.Open(ctx, dsn, cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns, cfg.Database.ConnMaxLifetime)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: %v\n", err)
		return 1
	}
	defer db.Close()

	f, err := os.Open(*file)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: open file: %v\n", err)
		return 1
	}
	defer f.Close()

	// Compute file hash for snapshot identity.
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: hash file: %v\n", err)
		return 1
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: seek: %v\n", err)
		return 1
	}
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Count lines for total.
	lineCount := 0
	{
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			fmt.Fprintf(stderr, "bookdb ingest: seek: %v\n", err)
			return 1
		}
		buf := make([]byte, 64*1024)
		for {
			n, err := f.Read(buf)
			for i := 0; i < n; i++ {
				if buf[i] == '\n' {
					lineCount++
				}
			}
			if err != nil {
				break
			}
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			fmt.Fprintf(stderr, "bookdb ingest: seek: %v\n", err)
			return 1
		}
	}

	snapshot := ingestion.Snapshot{
		ID:          *snapshotID,
		Hash:        fileHash,
		RecordCount: lineCount,
		RetrievedAt: time.Now().UTC(),
	}

	ing := ingestion.NewIngestor(db)
	job, err := ing.StartJob(ctx, snapshot)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: start job: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "bookdb ingest: job %s started (%d records)\n", job.JobID, job.TotalRecords)

	// Process records.
	records, errs := ingestion.ReadSnapshot(f)
	processed := int64(0)
	for rec := range records {
		_, err := ing.ProcessRecord(ctx, job.JobID, *rec)
		if err != nil {
			ing.FailJob(ctx, job.JobID, err.Error())
			fmt.Fprintf(stderr, "bookdb ingest: process record: %v\n", err)
			return 1
		}
		processed++
		if processed%1000 == 0 {
			_ = ing.UpdateCheckpoint(ctx, job.JobID, processed, nil)
			fmt.Fprintf(stderr, "bookdb ingest: %d/%d processed\n", processed, job.TotalRecords)
		}
	}
	if err := <-errs; err != nil {
		ing.FailJob(ctx, job.JobID, err.Error())
		fmt.Fprintf(stderr, "bookdb ingest: read error: %v\n", err)
		return 1
	}

	if err := ing.CompleteJob(ctx, job.JobID); err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: complete job: %v\n", err)
		return 1
	}

	// Verify accounting.
	if err := ing.VerifyAccounting(ctx, job.JobID); err != nil {
		fmt.Fprintf(stderr, "bookdb ingest: accounting: %v\n", err)
		return 1
	}

	finalJob, _ := ing.GetJob(ctx, job.JobID)
	fmt.Fprintf(stdout, "Job:        %s\n", job.JobID)
	fmt.Fprintf(stdout, "Status:     %s\n", finalJob.Status)
	fmt.Fprintf(stdout, "Processed:  %d\n", finalJob.Processed)
	fmt.Fprintf(stdout, "Accepted:   %d\n", finalJob.Accepted)
	fmt.Fprintf(stdout, "Unchanged:  %d\n", finalJob.Unchanged)
	fmt.Fprintf(stdout, "Rejected:   %d\n", finalJob.Rejected)
	fmt.Fprintf(stdout, "Quarantined:%d\n", finalJob.Quarantined)
	fmt.Fprintln(stderr, "bookdb ingest: complete")
	return 0
}
