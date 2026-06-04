package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestBackfillExpressions(t *testing.T) {
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("Database not available:", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("Database not available (ping failed):", err)
	}

	// Verify required tables exist
	var tablesExist bool
	err = db.QueryRowContext(ctx, `
		SELECT to_regclass('public.semantic_dimensions') IS NOT NULL AND 
		       to_regclass('public.semantic_metrics') IS NOT NULL
	`).Scan(&tablesExist)
	if err != nil || !tablesExist {
		t.Skip("Required semantic tables do not exist or are not migrated.")
	}

	// Seed test data
	datasourceID := uuid.NewString()
	modelID := uuid.NewString()
	dimID := uuid.NewString()
	metID := uuid.NewString()

	_, err = db.ExecContext(ctx,
		`INSERT INTO datasources (id, name, type, dsn_encrypted) VALUES ($1, $2, 'postgres', 'enc')`,
		datasourceID, "backfill-test-ds")
	if err != nil {
		t.Fatalf("failed to seed datasource: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM datasources WHERE id = $1`, datasourceID)
	}()

	_, err = db.ExecContext(ctx,
		`INSERT INTO semantic_models (id, datasource_id, name, base_schema, base_table) VALUES ($1, $2, $3, 'public', $4)`,
		modelID, datasourceID, "backfill_test_model", "orders")
	if err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}

	// Insert dimension with text calculated expression and null json
	_, err = db.ExecContext(ctx, `
		INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, is_active, calculated_expression, calculated_expr_json)
		VALUES ($1, $2, $3, $4, $5, $6, true, $7, NULL)
	`, dimID, modelID, "revenue_minus_cost", "Revenue Minus Cost", "revenue", "number", "orders.revenue - orders.cost")
	if err != nil {
		t.Fatalf("failed to seed dimension: %v", err)
	}

	// Insert metric with text expression and null json
	_, err = db.ExecContext(ctx, `
		INSERT INTO semantic_metrics (id, model_id, name, label, expression, aggregation, is_active, expr_json)
		VALUES ($1, $2, $3, $4, $5, $6, true, NULL)
	`, metID, modelID, "profit_margin", "Profit Margin", "orders.revenue / orders.cost", "sum",)
	if err != nil {
		t.Fatalf("failed to seed metric: %v", err)
	}

	// Run backfill
	err = backfillExpressions(ctx, db)
	if err != nil {
		t.Fatalf("backfillExpressions failed: %v", err)
	}

	// Verify dimension JSON AST
	var dimJSON sql.NullString
	err = db.QueryRowContext(ctx, `SELECT calculated_expr_json FROM semantic_dimensions WHERE id = $1`, dimID).Scan(&dimJSON)
	if err != nil {
		t.Fatalf("failed to query backfilled dimension: %v", err)
	}
	if !dimJSON.Valid || dimJSON.String == "" {
		t.Fatal("dimension calculated_expr_json was not backfilled")
	}

	var dimAST map[string]any
	if err := json.Unmarshal([]byte(dimJSON.String), &dimAST); err != nil {
		t.Fatalf("failed to unmarshal dimension AST JSON: %v", err)
	}
	if dimAST["type"] != "binary" || dimAST["op"] != "subtract" {
		t.Fatalf("unexpected dimension AST structure: %v", dimAST)
	}

	// Verify metric JSON AST
	var metJSON sql.NullString
	err = db.QueryRowContext(ctx, `SELECT expr_json FROM semantic_metrics WHERE id = $1`, metID).Scan(&metJSON)
	if err != nil {
		t.Fatalf("failed to query backfilled metric: %v", err)
	}
	if !metJSON.Valid || metJSON.String == "" {
		t.Fatal("metric expr_json was not backfilled")
	}

	var metAST map[string]any
	if err := json.Unmarshal([]byte(metJSON.String), &metAST); err != nil {
		t.Fatalf("failed to unmarshal metric AST JSON: %v", err)
	}
	if metAST["type"] != "binary" || metAST["op"] != "divide" {
		t.Fatalf("unexpected metric AST structure: %v", metAST)
	}
}
