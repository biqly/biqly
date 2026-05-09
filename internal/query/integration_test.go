package query

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

// testDSN returns the DSN for the test database.
func testDSN() string {
	dsn := os.Getenv("BI_TEST_DB_DSN")
	if dsn == "" {
		//nolint:gosec // test-only default DSN for local development
		dsn = "postgres://test_user:test_password@localhost:5433/test_data?sslmode=disable"
	}
	return dsn
}

// skipIfNoDB skips the test if no database is available.
func skipIfNoDB(t *testing.T) {
	dsn := testDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("no test database available")
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(context.Background()); err != nil {
		t.Skip("test database not reachable")
	}
}

// TestIntegration_PostgresConnection tests connecting to a real PostgreSQL database.
func TestIntegration_PostgresConnection(t *testing.T) {
	skipIfNoDB(t)

	dsn := testDSN()
	driver := postgres.NewDriver()

	ctx := context.Background()
	if err := driver.Ping(ctx, dsn); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	db, err := driver.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	t.Log("connected successfully")
}

// TestIntegration_Introspection tests schema introspection against a real database.
func TestIntegration_Introspection(t *testing.T) {
	skipIfNoDB(t)

	dsn := testDSN()
	driver := postgres.NewDriver()
	ctx := context.Background()

	db, err := driver.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	result, err := driver.Introspect(ctx, db)
	if err != nil {
		t.Fatalf("introspection failed: %v", err)
	}

	if len(result.Schemas) == 0 {
		t.Error("expected at least one schema")
	}
	if len(result.Tables) == 0 {
		t.Error("expected at least one table")
	}
	if len(result.Columns) == 0 {
		t.Error("expected at least one column")
	}

	t.Logf("found %d schemas, %d tables, %d columns, %d relations",
		len(result.Schemas), len(result.Tables), len(result.Columns), len(result.Relations))
}

// TestIntegration_CompileAndExecute tests the full compile → execute flow.
func TestIntegration_CompileAndExecute(t *testing.T) {
	skipIfNoDB(t)

	dsn := testDSN()
	driver := postgres.NewDriver()
	ctx := context.Background()

	db, err := driver.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to open connection: %v", err)
	}
	defer func() { _ = db.Close() }()

	model := &semantic.SemanticModel{
		Name:       "orders",
		BaseSchema: "public",
		BaseTable:  "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "total", Expression: "orders.amount", Aggregation: "sum"},
			{Name: "count", Expression: "orders.id", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name:         "orders_customers",
				FromTable:    "orders",
				FromColumn:   "customer_id",
				ToTable:      "customers",
				ToColumn:     "id",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}

	lq := LogicalQuery{
		ModelID: "orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "total"},
			{Type: "metric", Name: "count"},
		},
		GroupBy: []GroupBy{{Field: "country"}},
		OrderBy: []OrderBy{{Field: "total", Direction: "desc"}},
		Limit:   100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(ctx, lq, model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	t.Logf("compiled SQL: %s", cq.SQL)

	executor := NewExecutor(1000, 0)
	result, err := executor.Execute(ctx, db, cq)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	if result.Stats.RowCount == 0 {
		t.Error("expected at least one row")
	}

	if len(result.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(result.Columns))
	}

	t.Logf("got %d rows in %d ms", result.Stats.RowCount, result.Stats.DurationMs)
}

// TestIntegration_CompileMySQL tests MySQL dialect compilation.
func TestIntegration_CompileMySQL(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "products",
		BaseSchema: "store",
		BaseTable:  "products",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "products.name", Type: "text"},
			{Name: "price", ColumnRef: "products.price", Type: "number"},
		},
		Metrics: []semantic.Metric{
			{Name: "avg_price", Expression: "products.price", Aggregation: "avg"},
		},
	}

	lq := LogicalQuery{
		ModelID: "products",
		Select: []SelectItem{
			{Type: "dimension", Name: "name"},
			{Type: "metric", Name: "avg_price"},
		},
		Filters: []Filter{{Field: "price", Operator: OpGte, Value: 10.00}},
		GroupBy: []GroupBy{{Field: "name"}},
		Limit:   50,
	}

	compiler := NewCompiler(dialect.MySQLDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("MySQL compilation failed: %v", err)
	}

	// Verify MySQL-specific syntax
	if !containsStr(cq.SQL, "`") {
		t.Errorf("expected backtick quoting: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "?") {
		t.Errorf("expected ? placeholder: %s", cq.SQL)
	}
}

// TestIntegration_CompileClickHouse tests ClickHouse dialect compilation.
func TestIntegration_CompileClickHouse(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "events",
		BaseSchema: "default",
		BaseTable:  "events",
		Dimensions: []semantic.Dimension{
			{Name: "event_type", ColumnRef: "events.event_type", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "events.id", Aggregation: "count_distinct"},
		},
	}

	lq := LogicalQuery{
		ModelID: "events",
		Select: []SelectItem{
			{Type: "dimension", Name: "event_type"},
			{Type: "metric", Name: "count"},
		},
		GroupBy: []GroupBy{{Field: "event_type"}},
		Limit:   1000,
	}

	compiler := NewCompiler(dialect.ClickHouseDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("ClickHouse compilation failed: %v", err)
	}

	// ClickHouse uses uniq() for count_distinct
	if !containsStr(cq.SQL, "uniq(") {
		t.Errorf("expected uniq() for count_distinct: %s", cq.SQL)
	}
}

// TestIntegration_CompileSQLServer tests SQL Server dialect compilation.
func TestIntegration_CompileSQLServer(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "users",
		BaseSchema: "dbo",
		BaseTable:  "users",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "users.name", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "total", Expression: "users.id", Aggregation: "count"},
		},
	}

	lq := LogicalQuery{
		ModelID: "users",
		Select: []SelectItem{
			{Type: "dimension", Name: "name"},
			{Type: "metric", Name: "total"},
		},
		Filters: []Filter{{Field: "name", Operator: OpContains, Value: "admin"}},
		GroupBy: []GroupBy{{Field: "name"}},
		Limit:   10,
		Offset:  5,
	}

	compiler := NewCompiler(dialect.SQLServerDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("SQL Server compilation failed: %v", err)
	}

	// SQL Server uses @pN placeholders
	if !containsStr(cq.SQL, "@p") {
		t.Errorf("expected @p placeholder: %s", cq.SQL)
	}
	// SQL Server uses OFFSET/FETCH
	if !containsStr(cq.SQL, "OFFSET") || !containsStr(cq.SQL, "FETCH") {
		t.Errorf("expected OFFSET/FETCH: %s", cq.SQL)
	}
}

// TestIntegration_ReadOnlyProtection verifies that the executor rejects non-SELECT queries.
func TestIntegration_ReadOnlyProtection(t *testing.T) {
	cq := &CompiledQuery{
		SQL:  "DROP TABLE users",
		Args: nil,
	}

	executor := NewExecutor(1000, 0)
	_, err := executor.Execute(context.Background(), nil, cq)
	if err == nil {
		t.Fatal("expected error for DROP TABLE query")
	}

	cq2 := &CompiledQuery{
		SQL:  "INSERT INTO users VALUES (1, 'test')",
		Args: nil,
	}
	_, err = executor.Execute(context.Background(), nil, cq2)
	if err == nil {
		t.Fatal("expected error for INSERT query")
	}
}
