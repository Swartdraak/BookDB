package runtime

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/bookdb/bookdb/internal/search/opensearch"
)

// runWorkerIndex consumes the transactional outbox and projects the events
// into the OpenSearch search indices (S2 pipeline: publish). It runs to
// completion (exit 0) or on context cancellation; use --once to process a
// single batch, which is how the dev smoke and cron-style invocations drive it.
//
// Usage: bookdb worker index [--once] [--batch-size <n>] [--timeout <duration>]
func runWorkerIndex(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worker index", flag.ExitOnError)
	once := fs.Bool("once", false, "Process batches until the outbox is drained, then exit")
	batchSize := fs.Int("batch-size", 500, "Outbox events per batch")
	timeout := fs.Duration("timeout", 0, "Optional run timeout (for example 30m); 0 disables")
	_ = fs.Parse(args)

	cfg, _, err := loadRuntimeConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if strings.TrimSpace(cfg.OpenSearch.URL) == "" {
		fmt.Fprintln(stderr, "bookdb worker index: opensearch url is empty (BOOKDB_OPENSEARCH_URL)")
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
		fmt.Fprintf(stderr, "bookdb worker index: database: %v\n", err)
		return 1
	}
	defer db.Close()

	client, err := opensearch.New(cfg.OpenSearch.URL, cfg.OpenSearch.Timeout)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb worker index: opensearch client: %v\n", err)
		return 1
	}
	if err := client.Check(ctx); err != nil {
		fmt.Fprintf(stderr, "bookdb worker index: opensearch unreachable: %v\n", err)
		return 1
	}

	indexer := opensearch.NewIndexer(client, db)

	// Ensure the projection indices/aliases exist for the canonical entities
	// so the first batch can index immediately.
	entities := []string{"work", "edition", "person"}
	for _, entity := range entities {
		if err := indexer.EnsureIndex(ctx, entity, 1); err != nil {
			fmt.Fprintf(stderr, "bookdb worker index: ensure %s index: %v\n", entity, err)
			return 1
		}
	}

	total := 0
	batch := 0
	for {
		processed, err := indexer.ProcessOutbox(ctx, *batchSize)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(stderr, "bookdb worker index: process outbox: %v\n", err)
			return 1
		}
		batch++
		total += processed
		fmt.Fprintf(stderr, "bookdb worker index: batch %d processed %d (total %d)\n", batch, processed, total)
		if *once || processed < *batchSize {
			break
		}
	}

	fmt.Fprintf(stdout, "Batches:     %d\n", batch)
	fmt.Fprintf(stdout, "Processed:   %d\n", total)
	return 0
}
