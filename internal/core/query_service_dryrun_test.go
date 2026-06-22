package core_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// openDryRunDB opens a real Postgres for DryRun integration tests.
// Skips the test when no database is available.
func openDryRunDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		dsn = "postgres://bi_user:***@localhost:5432/bi_metadata?sslmode=disable" //nolint:gosec // default DSN for local dev, skips when unavailable
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("DryRun integration test: cannot open DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("DryRun integration test: DB not reachable: %v", err)
	}
	return db
}

// TestDryRun_Integration_ReadOnlyCheckPassesThenExplainRuns verifies the full
// DryRun pipeline against a real Postgres:
//  1. CompileWithContext succeeds (fake model, fake driver + real dialect)
//  2. Read-only guard passes (no false-positive on legitimate SELECT/WITH)
//  3. EXPLAIN runs against the real DB (may fail if tables don't exist, but
//     the error is from EXPLAIN, not from the read-only check)
func TestDryRun_Integration_ReadOnlyCheckPassesBeforeExplain(t *testing.T) {
	ctx := context.Background()
	db := openDryRunDB(t)

	model := coreTestModel()
	model.DatasourceID = "ds1"
	lq := coreTestLogicalQuery()

	driver := fakeDriver{dialect: dialect.PostgresDialect{}}
	service := core.NewQueryService(&core.QueryServiceDeps{
		Validator: query.NewValidator(1000),
	})

	se := service.DryRun(ctx, db, &lq, model, driver)

	if se == nil {
		// Tables happen to exist — full pipeline succeeded.
		t.Log("DryRun succeeded (EXPLAIN passed on real DB)")
		return
	}

	// The read-only guard must NOT have rejected the query.
	if strings.Contains(se.Error(), "read-only check:") {
		t.Fatalf("read-only guard falsely rejected a valid SELECT/WITH query: %v", se)
	}

	// If EXPLAIN fails because tables don't exist in the test DB, that's expected.
	// This still proves the read-only guard passed before reaching the DB.
	t.Logf("DryRun EXPLAIN failed (expected without real tables): %v", se)
}

// TestDryRun_Integration_CompileErrorBeforeGuard proves that a compile
// error (e.g. missing model fields) is returned BEFORE the read-only guard runs,
// confirming the guard isn't masking real compilation errors.
func TestDryRun_Integration_CompileErrorBeforeGuard(t *testing.T) {
	ctx := context.Background()
	db := openDryRunDB(t)

	// Empty model — CompileWithContext should fail with a validation error,
	// never reaching the read-only check or EXPLAIN.
	model := &semantic.SemanticModel{
		ID:           "empty",
		Name:         "empty",
		DatasourceID: "ds1",
		BaseSchema:   "public",
		BaseTable:    "t",
		Status:       semantic.ModelStatusPublished,
		Version:      1,
	}
	lq := query.LogicalQuery{
		DatasourceID: "ds1",
		ModelID:      "empty",
		Select:       []query.SelectItem{{Type: query.SelectTypeDimension, Name: "missing"}},
	}

	driver := fakeDriver{dialect: dialect.PostgresDialect{}}
	service := core.NewQueryService(&core.QueryServiceDeps{
		Validator: query.NewValidator(1000),
	})

	se := service.DryRun(ctx, db, &lq, model, driver)

	if se == nil {
		t.Fatal("expected DryRun to fail with compile error, got nil")
	}

	// The error must NOT mention "read-only check" — it should be a compile
	// validation error (missing dimension, invalid model, etc.).
	if strings.Contains(se.Error(), "read-only check:") {
		t.Fatalf("guard ran before compile validation: %v", se)
	}
	t.Logf("DryRun correctly returned compile error before guard: %v", se)
}
