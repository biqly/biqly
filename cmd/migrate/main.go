// Package main runs database migrations.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/biqly/biqly/internal/dbmigrate"
	"github.com/biqly/biqly/internal/query"
	"github.com/bytedance/sonic"
)

func main() {
	commands := dbmigrate.DefaultCommands()
	commands["backfill"] = func(ctx context.Context, db *sql.DB, _ string) error {
		return backfillExpressions(ctx, db)
	}
	os.Exit(dbmigrate.RunCLI(dbmigrate.CLIConfig{
		DSNEnv:     "BI_METADATA_DB_DSN",
		DSNUsage:   "Database DSN",
		DefaultDir: "migrations",
		Usage:      "Usage: migrate [up|down|backfill]",
		Commands:   commands,
	}))
}

func backfillExpressions(ctx context.Context, db *sql.DB) (err error) { //nolint:funlen,gocognit,gocyclo // one-off migration backfill over dimensions and metrics
	// Process dimensions
	dimRows, err := db.QueryContext(ctx, `SELECT id, calculated_expression FROM semantic_dimensions WHERE calculated_expression IS NOT NULL AND calculated_expression != ''`)
	if err != nil {
		return fmt.Errorf("query dimensions: %w", err)
	}
	defer func() {
		if closeErr := dimRows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close dimension rows: %w", closeErr)
		}
	}()

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
	defer func() {
		if closeErr := metRows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close metric rows: %w", closeErr)
		}
	}()

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
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := tx.Rollback(); err != nil {
			slog.Warn("rollback expression backfill transaction", "error", err)
		}
	}()

	dimStmt, err := tx.PrepareContext(ctx, `UPDATE semantic_dimensions SET calculated_expr_json = $1::jsonb WHERE id = $2::uuid`)
	if err != nil {
		return fmt.Errorf("prepare dimension update: %w", err)
	}
	defer func() {
		if closeErr := dimStmt.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close dimension update statement: %w", closeErr)
		}
	}()

	for _, u := range dimUpdates {
		if _, err := dimStmt.ExecContext(ctx, u.json, u.id); err != nil {
			return fmt.Errorf("update dimension %s: %w", u.id, err)
		}
	}

	metStmt, err := tx.PrepareContext(ctx, `UPDATE semantic_metrics SET expr_json = $1::jsonb WHERE id = $2::uuid`)
	if err != nil {
		return fmt.Errorf("prepare metric update: %w", err)
	}
	defer func() {
		if closeErr := metStmt.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close metric update statement: %w", closeErr)
		}
	}()

	for _, u := range metUpdates {
		if _, err := metStmt.ExecContext(ctx, u.json, u.id); err != nil {
			return fmt.Errorf("update metric %s: %w", u.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true

	slog.Info("backfill completed", "dimensions_updated", len(dimUpdates), "metrics_updated", len(metUpdates))
	return nil
}
