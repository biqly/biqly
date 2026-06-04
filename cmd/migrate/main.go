// Package main runs database migrations.
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

	"github.com/biqly/biqly/internal/query"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("BI_METADATA_DB_DSN"), "Database DSN")
	dir := flag.String("dir", "migrations", "Migrations directory")
	flag.Parse()

	if *dsn == "" {
		slog.Error("BI_METADATA_DB_DSN is required")
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) == 0 {
		slog.Error("Usage: migrate [up|down|backfill]")
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
	case "backfill":
		err = backfillExpressions(ctx, db)
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
		body, readErr := os.ReadFile(path) //nolint:gosec // path is constrained to migration files discovered from the configured migrations directory
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
	body, err := os.ReadFile(path) //nolint:gosec // path is derived from the latest applied migration name and configured migrations directory
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
	// 001a_foo.up.sql -> 001b_foo.down.sql
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

func backfillExpressions(ctx context.Context, db *sql.DB) error {
	// Process dimensions
	dimRows, err := db.QueryContext(ctx, `SELECT id, calculated_expression FROM semantic_dimensions WHERE calculated_expression IS NOT NULL AND calculated_expression != ''`)
	if err != nil {
		return fmt.Errorf("query dimensions: %w", err)
	}
	defer func() { _ = dimRows.Close() }()

	type dimUpdate struct {
		id   string
		json []byte
	}
	var dimUpdates []dimUpdate

	for dimRows.Next() {
		var id, exprStr string
		if err := dimRows.Scan(&id, &exprStr); err != nil {
			return fmt.Errorf("scan dimension: %w", err)
		}
		exprStr = strings.TrimSpace(exprStr)
		if exprStr == "" {
			continue
		}
		ast, err := query.ParseExpression(exprStr)
		if err != nil {
			slog.Warn("failed to parse dimension calculated expression", "id", id, "expr", exprStr, "error", err)
			continue
		}
		jsonBytes, err := sonic.Marshal(ast)
		if err != nil {
			return fmt.Errorf("marshal dimension ast: %w", err)
		}
		dimUpdates = append(dimUpdates, dimUpdate{id: id, json: jsonBytes})
	}
	if err := dimRows.Err(); err != nil {
		return err
	}

	// Process metrics
	metRows, err := db.QueryContext(ctx, `SELECT id, expression FROM semantic_metrics WHERE expression IS NOT NULL AND expression != '' AND expression != '*'`)
	if err != nil {
		return fmt.Errorf("query metrics: %w", err)
	}
	defer func() { _ = metRows.Close() }()

	type metUpdate struct {
		id   string
		json []byte
	}
	var metUpdates []metUpdate

	for metRows.Next() {
		var id, exprStr string
		if err := metRows.Scan(&id, &exprStr); err != nil {
			return fmt.Errorf("scan metric: %w", err)
		}
		exprStr = strings.TrimSpace(exprStr)
		if exprStr == "" || exprStr == "*" {
			continue
		}
		ast, err := query.ParseExpression(exprStr)
		if err != nil {
			slog.Warn("failed to parse metric expression", "id", id, "expr", exprStr, "error", err)
			continue
		}
		jsonBytes, err := sonic.Marshal(ast)
		if err != nil {
			return fmt.Errorf("marshal metric ast: %w", err)
		}
		metUpdates = append(metUpdates, metUpdate{id: id, json: jsonBytes})
	}
	if err := metRows.Err(); err != nil {
		return err
	}

	// Write updates in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	dimStmt, err := tx.PrepareContext(ctx, `UPDATE semantic_dimensions SET calculated_expr_json = $1::jsonb WHERE id = $2::uuid`)
	if err != nil {
		return fmt.Errorf("prepare dimension update: %w", err)
	}
	defer func() { _ = dimStmt.Close() }()

	for _, u := range dimUpdates {
		if _, err := dimStmt.ExecContext(ctx, u.json, u.id); err != nil {
			return fmt.Errorf("update dimension %s: %w", u.id, err)
		}
	}

	metStmt, err := tx.PrepareContext(ctx, `UPDATE semantic_metrics SET expr_json = $1::jsonb WHERE id = $2::uuid`)
	if err != nil {
		return fmt.Errorf("prepare metric update: %w", err)
	}
	defer func() { _ = metStmt.Close() }()

	for _, u := range metUpdates {
		if _, err := metStmt.ExecContext(ctx, u.json, u.id); err != nil {
			return fmt.Errorf("update metric %s: %w", u.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	slog.Info("backfill completed", "dimensions_updated", len(dimUpdates), "metrics_updated", len(metUpdates))
	return nil
}
