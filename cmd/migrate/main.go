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

func backfillExpressions(ctx context.Context, db *sql.DB) error {
	dimUpdates, err := collectDimensionExpressionUpdates(ctx, db)
	if err != nil {
		return err
	}
	metUpdates, err := collectMetricExpressionUpdates(ctx, db)
	if err != nil {
		return err
	}
	return applyExpressionBackfill(ctx, db, dimUpdates, metUpdates)
}

func collectDimensionExpressionUpdates(ctx context.Context, db *sql.DB) ([]expressionUpdate, error) {
	dimRows, err := db.QueryContext(ctx, `SELECT id, calculated_expression FROM semantic_dimensions WHERE calculated_expression IS NOT NULL AND calculated_expression != ''`)
	if err != nil {
		return nil, fmt.Errorf("query dimensions: %w", err)
	}
	defer func() { _ = dimRows.Close() }()

	var updates []expressionUpdate
	for dimRows.Next() {
		var id, exprStr string
		if err := dimRows.Scan(&id, &exprStr); err != nil {
			return nil, fmt.Errorf("scan dimension: %w", err)
		}
		update, ok, err := expressionUpdateFromString(id, exprStr)
		if err != nil {
			slog.Warn("failed to parse dimension calculated expression", "id", id, "expr", strings.TrimSpace(exprStr), "error", err)
			continue
		}
		if !ok {
			continue
		}
		updates = append(updates, update)
	}
	if err := dimRows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
}

func collectMetricExpressionUpdates(ctx context.Context, db *sql.DB) ([]expressionUpdate, error) {
	metRows, err := db.QueryContext(ctx, `SELECT id, expression FROM semantic_metrics WHERE expression IS NOT NULL AND expression != '' AND expression != '*'`)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer func() { _ = metRows.Close() }()

	var updates []expressionUpdate
	for metRows.Next() {
		var id, exprStr string
		if err := metRows.Scan(&id, &exprStr); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		exprStr = strings.TrimSpace(exprStr)
		if exprStr == "" || exprStr == "*" {
			continue
		}
		update, ok, err := expressionUpdateFromString(id, exprStr)
		if err != nil {
			return nil, err
		}
		if !ok {
			slog.Warn("failed to parse metric expression", "id", id, "expr", exprStr, "error", err)
			continue
		}
		updates = append(updates, update)
	}
	if err := metRows.Err(); err != nil {
		return nil, err
	}
	return updates, nil
}

func expressionUpdateFromString(id, exprStr string) (expressionUpdate, bool, error) {
	exprStr = strings.TrimSpace(exprStr)
	if exprStr == "" {
		return expressionUpdate{}, false, nil
	}
	ast, err := query.ParseExpression(exprStr)
	if err != nil {
		return expressionUpdate{}, false, err
	}
	jsonBytes, err := sonic.Marshal(ast)
	if err != nil {
		return expressionUpdate{}, false, err
	}
	return expressionUpdate{id: id, json: jsonBytes}, true, nil
}

func applyExpressionBackfill(ctx context.Context, db *sql.DB, dimUpdates, metUpdates []expressionUpdate) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rbErr := tx.Rollback(); rbErr != nil {
			slog.Warn("rollback expression backfill transaction", "error", rbErr)
		}
	}()

	if err := applyExpressionUpdates(ctx, tx, `UPDATE semantic_dimensions SET calculated_expr_json = $1::jsonb WHERE id = $2::uuid`, dimUpdates, "dimension"); err != nil {
		return err
	}
	if err := applyExpressionUpdates(ctx, tx, `UPDATE semantic_metrics SET expr_json = $1::jsonb WHERE id = $2::uuid`, metUpdates, "metric"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	slog.Info("backfill completed", "dimensions_updated", len(dimUpdates), "metrics_updated", len(metUpdates))
	return nil
}

func applyExpressionUpdates(ctx context.Context, tx *sql.Tx, updateSQL string, updates []expressionUpdate, label string) (err error) {
	stmt, err := tx.PrepareContext(ctx, updateSQL)
	if err != nil {
		return fmt.Errorf("prepare %s update: %w", label, err)
	}
	defer func() {
		if closeErr := stmt.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s update statement: %w", label, closeErr)
		}
	}()
	return executeExpressionUpdates(ctx, stmt, updates, label)
}

type expressionUpdate struct {
	id   string
	json []byte
}

func executeExpressionUpdates(ctx context.Context, stmt *sql.Stmt, updates []expressionUpdate, label string) error {
	for _, u := range updates {
		if _, err := stmt.ExecContext(ctx, u.json, u.id); err != nil {
			return fmt.Errorf("update %s %s: %w", label, u.id, err)
		}
	}
	return nil
}
