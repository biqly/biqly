package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/stretchr/testify/assert"
)

type mockPostgresConn struct {
	queries map[string]*mockPostgresRows
}

func (c *mockPostgresConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepare is not implemented")
}

func (c *mockPostgresConn) Close() error {
	return nil
}

func (c *mockPostgresConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions are not implemented")
}

func (c *mockPostgresConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockPostgresRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockPostgresRows) Columns() []string {
	return r.cols
}

func (r *mockPostgresRows) Close() error {
	return nil
}

func (r *mockPostgresRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.pos]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.pos++
	return nil
}

var (
	activeMockConn      *mockPostgresConn
	activeMockConnMutex sync.Mutex
)

// Define another driver wrapping the mock for our SQL Open
type postgresMockBridge struct{}

func (postgresMockBridge) Open(name string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("postgres_mock_bridge", postgresMockBridge{})
}

func TestPostgresDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "postgres", d.Type())
	assert.Equal(t, dialect.Postgres, d.Dialect())
}

func TestPostgresDriver_Introspect(t *testing.T) {
	queries := map[string]*mockPostgresRows{
		"schemata": {
			cols: []string{"schema_name"},
			rows: [][]driver.Value{
				{"public"},
				{"auth"},
			},
		},
		"pg_class": {
			cols: []string{"schema_name", "table_name", "table_type", "row_estimate", "comment"},
			rows: [][]driver.Value{
				{"public", "users", "BASE TABLE", int64(100), "User Table"},
				{"public", "orders", "BASE TABLE", int64(500), ""},
			},
		},
		"columns": {
			cols: []string{
				"table_schema", "table_name", "column_name", "data_type",
				"nullable", "ordinal_position", "character_maximum_length",
				"numeric_precision", "numeric_scale", "column_default", "comment",
			},
			rows: [][]driver.Value{
				{"public", "users", "id", "integer", true, int64(1), nil, nil, nil, "", "Primary Key"},
				{"public", "users", "name", "text", true, int64(2), int64(255), nil, nil, "''", ""},
				{"public", "orders", "id", "integer", true, int64(1), nil, nil, nil, "", "Order ID"},
				{"public", "orders", "user_id", "integer", true, int64(2), nil, nil, nil, "", "User ID Link"},
			},
		},
		"primary key": {
			cols: []string{"table_schema", "table_name", "column_name"},
			rows: [][]driver.Value{
				{"public", "users", "id"},
				{"public", "orders", "id"},
			},
		},
		"foreign key": {
			cols: []string{
				"constraint_name", "from_schema", "from_table", "from_column",
				"to_schema", "to_table", "to_column",
			},
			rows: [][]driver.Value{
				{"fk_orders_user_id", "public", "orders", "user_id", "public", "users", "id"},
			},
		},
	}

	db, err := sql.Open("postgres_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	activeMockConnMutex.Lock()
	activeMockConn = &mockPostgresConn{queries: queries}
	activeMockConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Schemas
	assert.Len(t, result.Schemas, 2)
	assert.Equal(t, "public", result.Schemas[0].Name)
	assert.Equal(t, "auth", result.Schemas[1].Name)

	// Tables
	assert.Len(t, result.Tables, 2)
	assert.Equal(t, "users", result.Tables[0].TableName)
	assert.Equal(t, "BASE TABLE", result.Tables[0].TableType)
	assert.Equal(t, int64(100), *result.Tables[0].RowEstimate)
	assert.Equal(t, "User Table", result.Tables[0].Comment)

	// Columns
	assert.Len(t, result.Columns, 4)
	assert.Equal(t, "id", result.Columns[0].ColumnName)
	assert.True(t, result.Columns[0].IsPrimaryKey)
	assert.True(t, result.Columns[0].Nullable)
	assert.Equal(t, "integer", result.Columns[0].DataType)

	assert.Equal(t, "name", result.Columns[1].ColumnName)
	assert.False(t, result.Columns[1].IsPrimaryKey)

	// Relations
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "fk_orders_user_id", result.Relations[0].ConstraintName)
	assert.Equal(t, "orders", result.Relations[0].FromTable)
	assert.Equal(t, "user_id", result.Relations[0].FromColumn)
	assert.Equal(t, "users", result.Relations[0].ToTable)
	assert.Equal(t, "id", result.Relations[0].ToColumn)
}
