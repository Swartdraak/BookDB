package runtime

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/bookdb/bookdb/internal/ingestion"
	"github.com/google/uuid"
)

// runIngest handles the bookdb ingest subcommand.
// Usage: bookdb ingest --file <path> --snapshot-id <id> [--timeout <duration>] [--count-lines]
func runIngest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	file := fs.String("file", "", "Path to the Open Library dump file (.jsonl or .txt.gz)")
	snapshotID := fs.String("snapshot-id", "", "Snapshot identifier")
	timeout := fs.Duration("timeout", 0, "Optional ingest timeout (for example 30m or 2h); 0 disables timeout")
	countLines := fs.Bool("count-lines", false, "Count records before ingest (expensive on large .gz dumps)")
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

	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

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

	// Compute file hash for snapshot identity (hashes compressed bytes for .gz).
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

	// Optionally pre-count records; skipped by default on large .gz dumps.
	lineCount := 0
	if *countLines {
		lineCount, err = countInputLines(*file)
		if err != nil {
			fmt.Fprintf(stderr, "bookdb ingest: count lines: %v\n", err)
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
	defer func() {
		if rec := recover(); rec != nil {
			failJobBestEffort(ing, job.JobID, fmt.Sprintf("panic: %v", rec))
			panic(rec)
		}
	}()
	fmt.Fprintf(stderr, "bookdb ingest: job %s started (%d records)\n", job.JobID, job.TotalRecords)

	// Open a decompressed reader for the actual ingest pass.
	input, err := openIngestInput(*file)
	if err != nil {
		failJobBestEffort(ing, job.JobID, err.Error())
		fmt.Fprintf(stderr, "bookdb ingest: open input: %v\n", err)
		return 1
	}
	defer input.Close()

	records, errs := ingestion.ReadSnapshot(input)
	processed := int64(0)
	for rec := range records {
		_, err := ing.ProcessRecord(ctx, job.JobID, *rec)
		if err != nil {
			failJobBestEffort(ing, job.JobID, err.Error())
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
		failJobBestEffort(ing, job.JobID, err.Error())
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
	fmt.Fprintf(stdout, "Job:         %s\n", job.JobID)
	fmt.Fprintf(stdout, "Status:      %s\n", finalJob.Status)
	fmt.Fprintf(stdout, "Processed:   %d\n", finalJob.Processed)
	fmt.Fprintf(stdout, "Accepted:    %d\n", finalJob.Accepted)
	fmt.Fprintf(stdout, "Unchanged:   %d\n", finalJob.Unchanged)
	fmt.Fprintf(stdout, "Rejected:    %d\n", finalJob.Rejected)
	fmt.Fprintf(stdout, "Quarantined: %d\n", finalJob.Quarantined)
	fmt.Fprintln(stderr, "bookdb ingest: complete")
	return 0
}

// openIngestInput opens the named file for reading, transparently decompressing
// .gz files so that both JSONL subsets and Open Library complete dumps (.txt.gz)
// are handled without pre-conversion.
func openIngestInput(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		return &compositeReadCloser{Reader: gz, closers: []io.Closer{gz, f}}, nil
	}
	return f, nil
}

// countInputLines counts newline-delimited records in the named file.
// For .gz files this requires a full decompression pass; use sparingly.
func countInputLines(path string) (int, error) {
	r, err := openIngestInput(path)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 32*1024*1024)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

type compositeReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *compositeReadCloser) Close() error {
	var firstErr error
	for _, cl := range c.closers {
		if err := cl.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// failJobBestEffort records job failure using a short-lived background context
// so the status persists even when the ingest context has already been canceled.
func failJobBestEffort(ing *ingestion.Ingestor, jobID uuid.UUID, errMsg string) {
	failCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ing.FailJob(failCtx, jobID, errMsg)
}
