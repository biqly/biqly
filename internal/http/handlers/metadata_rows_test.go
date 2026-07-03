package handlers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/go-chi/chi/v5"
)

func TestBuildTableRowsWhereRejectsUnknownColumn(t *testing.T) {
	cols := map[string]bool{"name": true}
	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name; DROP TABLE x", Operator: "eq", Value: "a"},
	}, cols)
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestBuildTableRowsWhereRejectsUnknownOperator(t *testing.T) {
	cols := map[string]bool{"name": true}
	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "regex", Value: "a"},
	}, cols)
	if err == nil {
		t.Fatal("expected error for unsupported operator")
	}
}

func TestBuildTableRowsWherePredicatesAndArgs(t *testing.T) {
	cols := map[string]bool{"name": true, "likes": true}
	where, args, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "contains", Value: "ali"},
		{Column: "likes", Operator: "gte", Value: "10"},
	}, cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(where, `"name"`) || !strings.Contains(where, `"likes" >= $2`) {
		t.Errorf("unexpected where clause: %q", where)
	}
	if len(args) != 2 || args[0] != "%ali%" || args[1] != "10" {
		t.Errorf("unexpected args: %#v", args)
	}
}

func TestBuildTableRowsWhereMultiChipBecomesOrGroup(t *testing.T) {
	cols := map[string]bool{"name": true}
	where, args, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "eq", Value: `["a","b"]`},
	}, cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(where, "OR") || !strings.HasPrefix(strings.TrimSpace(where), "WHERE (") {
		t.Errorf("expected OR group, got %q", where)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %#v", args)
	}
}

func TestBuildTableRowsProjectionAppliesPIIMasking(t *testing.T) {
	emailType := pii.TypeEmail
	tcknType := pii.TypeTCKimlikNo
	columns := []metadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "id"},
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &emailType},
		{SchemaName: "public", TableName: "customers", ColumnName: "tckn", PIIType: &tcknType},
	}
	access, types := pii.BuildColumnAccessMaps(pii.RoleViewer, columns, nil)
	cfg := &query.PIIMaskingConfig{ColumnAccess: access, ColumnTypes: types}

	projection := buildTableRowsProjection(dialect.Postgres, columns, cfg)

	if strings.Contains(strings.Join(projection, ", "), `"tckn"`) {
		t.Fatalf("hidden column must not be projected: %#v", projection)
	}
	if !containsProjection(projection, `"id"`) {
		t.Fatalf("raw column missing from projection: %#v", projection)
	}
	if !containsProjection(projection, `AS "email"`) || !containsProjection(projection, "'***'") {
		t.Fatalf("masked column must be projected as masked expression with stable alias: %#v", projection)
	}
}

func TestBuildTableRowsWhereRejectsProtectedPIIColumns(t *testing.T) {
	cols := map[string]bool{"email": true, "tckn": true}
	protected := map[string]bool{"email": true, "tckn": true}

	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "email", Operator: "eq", Value: "alice@example.com"},
	}, cols, protected)
	if err == nil {
		t.Fatal("expected masked column filter to be rejected")
	}

	_, _, err = buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "tckn", Operator: "eq", Value: "10000000146"},
	}, cols, protected)
	if err == nil {
		t.Fatal("expected hidden column filter to be rejected")
	}
}

func TestBuildTableRowsOrderRejectsProtectedPIIColumns(t *testing.T) {
	err := validateTableRowsOrderBy("email", map[string]bool{"email": true}, map[string]bool{"email": true})
	if err == nil {
		t.Fatal("expected masked order_by column to be rejected")
	}
}

func containsProjection(projection []string, want string) bool {
	for _, expr := range projection {
		if strings.Contains(expr, want) {
			return true
		}
	}
	return false
}

type failingRowsDriver struct{}

func (failingRowsDriver) Type() string { return "failing_rows" }
func (failingRowsDriver) Ping(context.Context, string) error {
	return nil
}
func (failingRowsDriver) Open(context.Context, string) (*sql.DB, error) {
	return sql.Open("failing_rows_sql", "")
}
func (failingRowsDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return &datasource.IntrospectionResult{}, nil
}
func (failingRowsDriver) Dialect() dialect.Dialect { return dialect.Postgres }
func (failingRowsDriver) SupportsReadOnlyTx() bool { return false }

type failingRowsSQLDriver struct{}

func (failingRowsSQLDriver) Open(string) (driver.Conn, error) { return failingRowsConn{}, nil }

type failingRowsConn struct{}

func (failingRowsConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (failingRowsConn) Close() error                        { return nil }
func (failingRowsConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (failingRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("customer-db-secret: permission denied on payroll.salaries")
}

type failingCountDriver struct{}

func (failingCountDriver) Type() string { return "failing_count" }
func (failingCountDriver) Ping(context.Context, string) error {
	return nil
}
func (failingCountDriver) Open(context.Context, string) (*sql.DB, error) {
	return sql.Open("failing_count_sql", "")
}
func (failingCountDriver) Introspect(context.Context, *sql.DB) (*datasource.IntrospectionResult, error) {
	return &datasource.IntrospectionResult{}, nil
}
func (failingCountDriver) Dialect() dialect.Dialect { return dialect.Postgres }
func (failingCountDriver) SupportsReadOnlyTx() bool { return false }

type failingCountSQLDriver struct{}

func (failingCountSQLDriver) Open(string) (driver.Conn, error) { return failingCountConn{}, nil }

type failingCountConn struct{}

func (failingCountConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (failingCountConn) Close() error                        { return nil }
func (failingCountConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (failingCountConn) QueryContext(_ context.Context, sqlQuery string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(sqlQuery, "COUNT") {
		return nil, errors.New("customer-db-secret: count denied on payroll.salaries")
	}
	return &fakeSampleRows{
		cols: []string{"email"},
		rows: [][]driver.Value{{"alice@example.com"}},
	}, nil
}

var registerFailingRowsSQLDriver sync.Once
var registerFailingCountSQLDriver sync.Once

func TestBrowseTableRowsDoesNotLeakDriverError(t *testing.T) {
	registerFailingRowsSQLDriver.Do(func() {
		sql.Register("failing_rows_sql", failingRowsSQLDriver{})
	})

	db, state := setupMockDB(t)
	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM tables WHERE datasource_id",
			Cols:    metadataTableCols(),
			Rows: [][]driver.Value{
				{"table-1", "ds-1", "schema-1", "public", "orders", "BASE TABLE", nil, nil, nil, nil, now, now},
			},
		},
		{
			Pattern: "FROM columns",
			Cols:    metadataColumnCols(),
			Rows: [][]driver.Value{
				{"col-1", "ds-1", "table-1", "public", "orders", "email", "text", true, nil, nil, nil, nil, nil, nil, false, false, nil, nil, nil, now, nil, nil, nil, nil, nil},
			},
		},
		{
			Pattern: "FROM datasources WHERE id",
			Cols:    metadataDatasourceCols(),
			Rows: [][]driver.Value{
				{"ds-1", "Failing DS", "failing_rows", "postgres://secret@db", "{}", true, nil, now, now, nil, nil, nil, nil, nil, nil, []byte("{}"), "raw"},
			},
		},
	}
	reg := datasource.NewRegistry()
	reg.Register(failingRowsDriver{})
	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db), DriverReg: reg})
	router := chi.NewRouter()
	router.Post("/datasources/{id}/tables/{schema}/{table}/rows", handler.BrowseTableRows)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/datasources/ds-1/tables/public/orders/rows", strings.NewReader(`{"limit":1}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("BrowseTableRows failing driver status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Fatalf("BrowseTableRows failing driver body = %q, want public message", body)
	}
	if strings.Contains(body, "customer-db-secret") || strings.Contains(body, "payroll.salaries") {
		t.Fatalf("BrowseTableRows response leaked driver error detail: %q", body)
	}
}

func TestBrowseTableRowsCountDoesNotLeakDriverError(t *testing.T) {
	registerFailingCountSQLDriver.Do(func() {
		sql.Register("failing_count_sql", failingCountSQLDriver{})
	})

	db, state := setupMockDB(t)
	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM tables WHERE datasource_id",
			Cols:    metadataTableCols(),
			Rows: [][]driver.Value{
				{"table-1", "ds-1", "schema-1", "public", "orders", "BASE TABLE", nil, nil, nil, nil, now, now},
			},
		},
		{
			Pattern: "FROM columns",
			Cols:    metadataColumnCols(),
			Rows: [][]driver.Value{
				{"col-1", "ds-1", "table-1", "public", "orders", "email", "text", true, nil, nil, nil, nil, nil, nil, false, false, nil, nil, nil, now, nil, nil, nil, nil, nil},
			},
		},
		{
			Pattern: "FROM datasources WHERE id",
			Cols:    metadataDatasourceCols(),
			Rows: [][]driver.Value{
				{"ds-1", "Failing DS", "failing_count", "postgres://secret@db", "{}", true, nil, now, now, nil, nil, nil, nil, nil, nil, []byte("{}"), "raw"},
			},
		},
	}
	reg := datasource.NewRegistry()
	reg.Register(failingCountDriver{})
	handler := NewMetadataHandler(&app.CatalogDeps{MetaRepo: metadata.NewRepository(db), DriverReg: reg})
	router := chi.NewRouter()
	router.Post("/datasources/{id}/tables/{schema}/{table}/rows", handler.BrowseTableRows)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/datasources/ds-1/tables/public/orders/rows", strings.NewReader(`{"limit":1,"include_total":true}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("BrowseTableRows count failing driver status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") {
		t.Fatalf("BrowseTableRows count failing driver body = %q, want public message", body)
	}
	if strings.Contains(body, "customer-db-secret") || strings.Contains(body, "payroll.salaries") {
		t.Fatalf("BrowseTableRows count response leaked driver error detail: %q", body)
	}
}

func metadataDatasourceCols() []string {
	return []string{"id", "name", "type", "dsn_encrypted", "config", "is_active", "last_sync_at", "created_at", "updated_at", "host", "port", "username", "password_encrypted", "database_name", "ssl_mode", "connection_params", "dsn_mode"}
}

var _ datasource.Driver = failingRowsDriver{}
var _ datasource.Driver = failingCountDriver{}
var _ driver.Driver = failingRowsSQLDriver{}
var _ driver.Driver = failingCountSQLDriver{}
var _ driver.QueryerContext = failingRowsConn{}
var _ driver.QueryerContext = failingCountConn{}
