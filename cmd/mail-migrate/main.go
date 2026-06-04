// Package main runs database migrations for the mail database.
package main

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
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("BI_MAIL_DB_DSN"), "Database DSN for mail DB")
	dir := flag.String("dir", "migrations/mail", "Migrations directory")
	flag.Parse()

	if *dsn == "" {
		slog.Error("BI_MAIL_DB_DSN is required")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		slog.Error("Usage: mail-migrate [up|down]")
		os.Exit(1)
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		slog.Error("resolve migrations directory", "dir", *dir, "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("ping database", "error", err)
		os.Exit(1)
	}

	if err := ensureAppliedTable(ctx, db); err != nil {
		slog.Error("ensure tracking table", "error", err)
		os.Exit(1)
	}

	switch args[0] {
	case "up":
		err = migrateUp(ctx, db, absDir)
	case "down":
		err = migrateDown(ctx, db, absDir)
	default:
		slog.Error("unknown command", "command", args[0])
		os.Exit(1)
	}

	if err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migration completed", "command", args[0])
}

func ensureAppliedTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS biqly_applied_migrations (
    filename TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

func migrateUp(ctx context.Context, db *sql.DB, dir string) error {
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
		body, readErr := os.ReadFile(path) //nolint:gosec
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

func migrateDown(ctx context.Context, db *sql.DB, dir string) error {
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
	body, err := os.ReadFile(path) //nolint:gosec
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
	if !strings.Contains(base, "a_") {
		return ""
	}
	return strings.Replace(base, "a_", "b_", 1) + ".down.sql"
}

func loadAppliedSet(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT filename FROM biqly_applied_migrations`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func execSQL(ctx context.Context, db *sql.DB, sqlText string) error {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, sqlText)
	return err
}

func isAlreadyAppliedError(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return false
	}
	switch pgErr.Code {
	case "42P07", "42701", "42P06", "42710", "23505":
		return true
	default:
		return false
	}
}
