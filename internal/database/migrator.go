package database

// Package database provides the BookDB PostgreSQL foundation: health checks,
// migration loading, migration execution, and test-database bootstrap helpers.

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bookdb/bookdb/internal/health"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

const (
	// DefaultTrackingTable stores applied migration state in the public schema.
	DefaultTrackingTable = "public.bookdb_migrations"
	// DefaultApplicationSchema is the empty application schema created by the
	// M0 bootstrap migration.
	DefaultApplicationSchema = "bookdb"
	// defaultMigrationDir is the embedded migration directory root.
	defaultMigrationDir = "."
)

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]+)_([a-zA-Z0-9_-]+)\.sql$`)
var safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Row is the subset of sql.Row used by the migrator.
type Row interface {
	Scan(dest ...any) error
}

// Rows is the subset of sql.Rows used by the migrator.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

// Executor is the query/exec surface needed for status reporting.
type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (Rows, error)
	QueryRowContext(context.Context, string, ...any) Row
}

// Transaction is the transactional subset used to apply a single migration.
type Transaction interface {
	Executor
	Commit() error
	Rollback() error
}

// Transactor can create a transaction.
type Transactor interface {
	BeginTx(context.Context, *sql.TxOptions) (Transaction, error)
}

// DB is the complete surface required to run migrations.
type DB interface {
	Executor
	Transactor
}

// Migration describes a single SQL migration file.
type Migration struct {
	Version  string
	Name     string
	Filename string
	SQL      string
}

// AppliedMigration is a row from the migration tracking table.
type AppliedMigration struct {
	Version   string
	Name      string
	AppliedAt time.Time
}

// StatusReport summarizes the current database migration state.
type StatusReport struct {
	TrackingTablePresent bool
	CurrentVersion       string
	Applied              []AppliedMigration
	Pending              []Migration
}

// UpToDate reports whether all known migrations are applied.
func (r StatusReport) UpToDate() bool { return len(r.Pending) == 0 }

// Migrator loads and applies embedded or file-based SQL migrations.
type Migrator struct {
	FS                   fs.FS
	Directory            string
	TrackingTable        string
	ApplicationSchema    string
	MigrationFilePattern *regexp.Regexp
}

// DefaultMigrator uses the embedded M0 migration set.
func DefaultMigrator() Migrator {
	return Migrator{
		FS:                defaultEmbeddedMigrationFS(),
		Directory:         defaultMigrationDir,
		TrackingTable:     DefaultTrackingTable,
		ApplicationSchema: DefaultApplicationSchema,
	}
}

func defaultEmbeddedMigrationFS() fs.FS {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(fmt.Errorf("database: embedded migrations unavailable: %w", err))
	}
	return sub
}

func (m Migrator) normalize() Migrator {
	if m.FS == nil {
		m.FS = defaultEmbeddedMigrationFS()
	}
	if m.Directory == "" {
		m.Directory = defaultMigrationDir
	}
	if m.TrackingTable == "" {
		m.TrackingTable = DefaultTrackingTable
	}
	if m.ApplicationSchema == "" {
		m.ApplicationSchema = DefaultApplicationSchema
	}
	if m.MigrationFilePattern == nil {
		m.MigrationFilePattern = migrationFilenamePattern
	}
	return m
}

// Run executes the requested migrate action. The empty action and "up" both
// apply pending migrations; "status" only reports state.
func (m Migrator) Run(ctx context.Context, db DB, action string) (StatusReport, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "up", "apply":
		return m.Up(ctx, db)
	case "status":
		return m.Status(ctx, db)
	default:
		return StatusReport{}, fmt.Errorf("database: unknown migrate action %q", action)
	}
}

// Load returns all SQL migrations in filename order.
func (m Migrator) Load() ([]Migration, error) {
	m = m.normalize()

	entries, err := fs.ReadDir(m.FS, m.Directory)
	if err != nil {
		return nil, fmt.Errorf("database: read migrations: %w", err)
	}

	loaded := make([]Migration, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		matches := m.MigrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("database: migration file %q must match %s", entry.Name(), m.MigrationFilePattern.String())
		}
		version := matches[1]
		if _, exists := seen[version]; exists {
			return nil, fmt.Errorf("database: duplicate migration version %q", version)
		}
		seen[version] = struct{}{}

		content, err := fs.ReadFile(m.FS, pathJoin(m.Directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("database: read migration %q: %w", entry.Name(), err)
		}
		loaded = append(loaded, Migration{
			Version:  version,
			Name:     matches[2],
			Filename: entry.Name(),
			SQL:      strings.TrimSpace(string(content)),
		})
	}

	sort.SliceStable(loaded, func(i, j int) bool {
		left, _ := strconv.Atoi(loaded[i].Version)
		right, _ := strconv.Atoi(loaded[j].Version)
		if left == right {
			return loaded[i].Filename < loaded[j].Filename
		}
		return left < right
	})

	return loaded, nil
}

// Status reports which migrations exist and which have been applied.
func (m Migrator) Status(ctx context.Context, db Executor) (StatusReport, error) {
	m = m.normalize()

	migrations, err := m.Load()
	if err != nil {
		return StatusReport{}, err
	}

	present, err := trackingTablePresent(ctx, db, m.TrackingTable)
	if err != nil {
		return StatusReport{}, err
	}
	if !present {
		return StatusReport{TrackingTablePresent: false, Pending: migrations}, nil
	}

	applied, err := loadApplied(ctx, db, m.TrackingTable)
	if err != nil {
		return StatusReport{}, err
	}

	appliedSet := make(map[string]struct{}, len(applied))
	for _, row := range applied {
		appliedSet[row.Version] = struct{}{}
	}

	pending := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if _, ok := appliedSet[migration.Version]; ok {
			continue
		}
		pending = append(pending, migration)
	}

	currentVersion := ""
	if len(applied) > 0 {
		currentVersion = applied[len(applied)-1].Version
	}

	return StatusReport{
		TrackingTablePresent: true,
		CurrentVersion:       currentVersion,
		Applied:              applied,
		Pending:              pending,
	}, nil
}

// Up applies all pending migrations and then returns the resulting status.
func (m Migrator) Up(ctx context.Context, db DB) (StatusReport, error) {
	m = m.normalize()

	if err := ensureTrackingTable(ctx, db, m.TrackingTable); err != nil {
		return StatusReport{}, err
	}

	migrations, err := m.Load()
	if err != nil {
		return StatusReport{}, err
	}

	status, err := m.Status(ctx, db)
	if err != nil {
		return StatusReport{}, err
	}
	applied := make(map[string]struct{}, len(status.Applied))
	for _, row := range status.Applied {
		applied[row.Version] = struct{}{}
	}

	for _, migration := range migrations {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if err := applyMigration(ctx, db, m.TrackingTable, migration); err != nil {
			return StatusReport{}, err
		}
	}

	return m.Status(ctx, db)
}

func trackingTablePresent(ctx context.Context, db Executor, table string) (bool, error) {
	row := db.QueryRowContext(ctx, `SELECT to_regclass($1)`, table)
	var regclass sql.NullString
	if err := row.Scan(&regclass); err != nil {
		return false, fmt.Errorf("database: check migration table: %w", err)
	}
	return regclass.Valid, nil
}

func loadApplied(ctx context.Context, db Executor, table string) ([]AppliedMigration, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT version, name, applied_at FROM %s ORDER BY applied_at, version`, table))
	if err != nil {
		return nil, fmt.Errorf("database: load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make([]AppliedMigration, 0)
	for rows.Next() {
		var row AppliedMigration
		if err := rows.Scan(&row.Version, &row.Name, &row.AppliedAt); err != nil {
			return nil, fmt.Errorf("database: scan applied migration: %w", err)
		}
		applied = append(applied, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("database: applied migration rows: %w", err)
	}
	return applied, nil
}

func ensureTrackingTable(ctx context.Context, db Executor, table string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version text PRIMARY KEY,
		name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`, table))
	if err != nil {
		return fmt.Errorf("database: ensure migration table: %w", err)
	}
	return nil
}

func applyMigration(ctx context.Context, db DB, table string, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin migration %s: %w", migration.Version, err)
	}

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("database: apply migration %s (%s): %w", migration.Version, migration.Filename, err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (version, name, applied_at) VALUES ($1, $2, now()) ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name`, table), migration.Version, migration.Name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("database: record migration %s: %w", migration.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: commit migration %s: %w", migration.Version, err)
	}
	return nil
}

// quoteIdent validates a SQL identifier and returns a quoted version suitable
// for use in SQL statements. It rejects anything that is not a simple Postgres
// identifier to keep the helper predictable and injection-resistant.
func quoteIdent(name string) (string, error) {
	if !safeIdentifierPattern.MatchString(name) {
		return "", fmt.Errorf("database: invalid identifier %q", name)
	}
	return `"` + name + `"`, nil
}

func pathJoin(base, name string) string {
	if base == "." || base == "" {
		return name
	}
	return base + "/" + name
}

// Pinger is the minimal dependency health surface.
type Pinger interface {
	PingContext(context.Context) error
}

// HealthChecker adapts a Pinger to the BookDB health package.
type HealthChecker struct{ Pinger Pinger }

// NewHealthChecker returns a health.Checker for a PostgreSQL connection.
func NewHealthChecker(p Pinger) health.Checker { return HealthChecker{Pinger: p} }

// Check reports OK when PingContext succeeds and Down otherwise.
func (c HealthChecker) Check(ctx context.Context) health.Status {
	if c.Pinger == nil {
		return health.StatusDown
	}
	if err := c.Pinger.PingContext(ctx); err != nil {
		return health.StatusDown
	}
	return health.StatusOK
}
