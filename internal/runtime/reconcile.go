package runtime

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/bookdb/bookdb/internal/database"
	"github.com/bookdb/bookdb/internal/ingestion"
	"github.com/bookdb/bookdb/internal/reconciliation"
)

// runReconcile handles the bookdb reconcile subcommand (S3 post-pass,
// issue #55): OL API enrichment of promoted works/people with field-level
// provenance selection, plus cross-source duplicate candidate generation.
//
// Usage: bookdb reconcile [--source openlibrary] [--max-per-type N]
//
// The pass performs bounded external HTTP (one request per entity key,
// default per-host concurrency 1 — sequential execution) and is idempotent:
// re-running it re-selects the same provenance values and re-upserts the
// same candidate rows.
func runReconcile(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile", flag.ExitOnError)
	source := fs.String("source", ingestion.SourceName, "Source name whose promoted entities are enriched")
	maxPerType := fs.Int("max-per-type", 0, "Cap on enrichment requests per entity type (0 = all)")
	timeout := fs.Duration("timeout", 0, "Optional reconcile timeout (for example 30m); 0 disables")
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

	dsn := cfg.Database.DSNDirect
	if dsn == "" {
		dsn = cfg.Database.DSN
	}
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	db, err := database.Open(ctx, dsn, 10, 2, 0)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb reconcile: %v\n", err)
		return 1
	}
	defer db.Close()

	client := reconciliation.NewEnrichmentClient(reconciliation.EnrichmentConfig{
		Timeout: 15 * time.Second,
	})
	rec := reconciliation.NewReconciler(db)

	out, err := rec.RunReconcile(ctx, client, *source, *maxPerType)
	if err != nil {
		fmt.Fprintf(stderr, "bookdb reconcile: %v\n", err)
		return 1
	}

	raw, _ := json.Marshal(out)
	fmt.Fprintf(stdout, "%s\n", raw)
	fmt.Fprintf(stderr, "bookdb reconcile: %d fields enriched, %d duplicate candidates, %d errors\n",
		out.Enriched, len(out.Candidates), len(out.Errors))
	return 0
}
