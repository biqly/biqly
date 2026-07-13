// Package dbmigrate applies simple SQL migration files tracked in the target database.
package dbmigrate

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver for database/sql
)

const DefaultCommandTimeout = 5 * time.Minute

// CommandFunc runs a migration command against an opened database.
type CommandFunc func(context.Context, *sql.DB, string) error

// CLIConfig configures a migration command-line entrypoint.
type CLIConfig struct {
	DSNEnv     string
	DSNUsage   string
	DefaultDir string
	Usage      string
	Commands   map[string]CommandFunc
}

// DefaultCommands returns the standard up/down migration commands.
func DefaultCommands() map[string]CommandFunc {
	return map[string]CommandFunc{
		"up":   Up,
		"down": Down,
	}
}

// RunCLI parses standard migration flags, runs the requested command, and returns an exit code.
func RunCLI(config CLIConfig) int {
	dsn := flag.String("dsn", os.Getenv(config.DSNEnv), config.DSNUsage)
	dir := flag.String("dir", config.DefaultDir, "Migrations directory")
	flag.Parse()

	if *dsn == "" {
		slog.Error(config.DSNEnv + " is required")
		return 1
	}

	args := flag.Args()
	if len(args) == 0 {
		slog.Error(config.Usage)
		return 1
	}

	commands := config.Commands
	if commands == nil {
		commands = DefaultCommands()
	}
	command := args[0]
	run, ok := commands[command]
	if !ok {
		slog.Error("unknown command", "command", command)
		return 1
	}

	absDir, err := ResolveMigrationsDir(*dir)
	if err != nil {
		slog.Error("resolve migrations directory", "dir", *dir, "error", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultCommandTimeout)
	defer cancel()

	db, err := Connect(ctx, *dsn)
	if err != nil {
		slog.Error("connect database", "error", err)
		return 1
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("close database", "error", err)
		}
	}()

	if err := EnsureAppliedTable(ctx, db); err != nil {
		slog.Error("ensure tracking table", "error", err)
		return 1
	}

	if err := run(ctx, db, absDir); err != nil {
		slog.Error("migration failed", "error", err)
		return 1
	}
	slog.Info("migration completed", "command", command)
	return 0
}

// ResolveMigrationsDir returns the absolute path for a migrations directory.
func ResolveMigrationsDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve migrations directory: %w", err)
	}
	return abs, nil
}

// Connect opens a pgx database and verifies connectivity.
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// EnsureAppliedTable creates the migration tracking table when it does not exist.
func EnsureAppliedTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS biqly_applied_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

// Up applies all unapplied *.up.sql migrations in lexical order.
func Up(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		slog.Info("no up migrations found", "dir", dir)
		return nil
	}

	applied, err := loadAppliedSet(ctx, db)
	if err != nil {
		return err
	}

	var appliedAny bool
	for _, path := range files {
		name := filepath.Base(path)
		if applied[name] {
			continue
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // migration paths come from the migrations directory listing
		if readErr != nil {
			return fmt.Errorf("read %s: %w", name, readErr)
		}
		if execErr := execSQL(ctx, db, string(body)); execErr != nil && !isAlreadyAppliedError(execErr) {
			return fmt.Errorf("apply %s: %w", name, execErr)
		}
		if _, insErr := db.ExecContext(ctx,
			`INSERT INTO biqly_applied_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`,
			name,
		); insErr != nil {
			return fmt.Errorf("record %s: %w", name, insErr)
		}
		slog.Info("applied migration", "file", name)
		appliedAny = true
	}
	if !appliedAny {
		slog.Info("no changes to apply")
	}
	return nil
}

// Down reverts the latest applied migration using its paired *.down.sql file.
func Down(ctx context.Context, db *sql.DB, dir string) error {
	var latest string
	err := db.QueryRowContext(ctx,
		`SELECT filename FROM biqly_applied_migrations ORDER BY filename DESC LIMIT 1`,
	).Scan(&latest)
	if errors.Is(err, sql.ErrNoRows) {
		slog.Info("no applied migrations")
		return nil
	}
	if err != nil {
		return err
	}

	downName := upToDownFilename(latest)
	if downName == "" {
		return fmt.Errorf("no down migration for %s", latest)
	}
	path := filepath.Join(dir, downName)
	body, err := os.ReadFile(path) //nolint:gosec // down migration path is derived from applied migration record
	if err != nil {
		return fmt.Errorf("read %s: %w", downName, err)
	}
	if err := execSQL(ctx, db, string(body)); err != nil {
		return fmt.Errorf("apply %s: %w", downName, err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM biqly_applied_migrations WHERE filename = $1`, latest); err != nil {
		return err
	}
	slog.Info("reverted migration", "up", latest, "down", downName)
	return nil
}

func upToDownFilename(up string) string {
	if !strings.HasSuffix(up, ".up.sql") {
		return ""
	}
	base := strings.TrimSuffix(up, ".up.sql")
	return base + ".down.sql"
}

func loadAppliedSet(ctx context.Context, db *sql.DB) (out map[string]bool, err error) {
	rows, err := db.QueryContext(ctx, `SELECT filename FROM biqly_applied_migrations`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	out = make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return out, nil
}

func execSQL(ctx context.Context, db *sql.DB, sqlText string) error {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return nil
	}
	// Run the whole file body in one transaction so a migration that fails
	// partway rolls back atomically instead of leaving the schema half-applied
	// (and then getting recorded as applied). Postgres DDL is transactional; no
	// migration in this repo uses a statement that cannot run inside a tx (e.g.
	// CREATE INDEX CONCURRENTLY) — add a non-transactional path here if one is
	// ever introduced.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, execErr := tx.ExecContext(ctx, sqlText); execErr != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback failed: %w)", execErr, rbErr)
		}
		return execErr
	}
	return tx.Commit()
}

func isAlreadyAppliedError(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return false
	}
	// Only tolerate idempotent-DDL "already exists" codes (duplicate
	// table/column/schema/object) so re-running a create is a no-op. 23505
	// (unique_violation) is intentionally NOT here: a unique violation is a real
	// failure that must not be silently recorded as an applied migration.
	switch pgErr.Code {
	case "42P07", "42701", "42P06", "42710":
		return true
	default:
		return false
	}
}
