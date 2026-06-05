package sqlserver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/stretchr/testify/assert"
)

type mockSQLServerConn struct {
	queries map[string]*mockSQLServerRows
}

func (*mockSQLServerConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockSQLServerConn) Close() error {
	return nil
}

func (*mockSQLServerConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockSQLServerConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockSQLServerRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockSQLServerRows) Columns() []string {
	return r.cols
}

func (*mockSQLServerRows) Close() error {
	return nil
}

func (r *mockSQLServerRows) Next(dest []driver.Value) error {
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
	activeMockConn      *mockSQLServerConn
	activeMockConnMutex sync.Mutex
)

type sqlserverMockBridge struct{}

func (sqlserverMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("sqlserver_mock_bridge", sqlserverMockBridge{})
}

func TestSqlServerDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "sqlserver", d.Type())
	assert.Equal(t, dialect.SQLServer, d.Dialect())
}

func TestSqlServerDriver_Introspect(t *testing.T) {
	queries := map[string]*mockSQLServerRows{
		"sys.schemas where name": {
			cols: []string{"name"},
			rows: [][]driver.Value{
				{"sales"},
				{"hr"},
			},
		},
		"sys.tables t join": {
			cols: []string{"schema_name", "table_name", "table_type", "row_estimate"},
			rows: [][]driver.Value{
				{"sales", "customers", "BASE TABLE", nil},
				{"sales", "orders", "BASE TABLE", nil},
			},
		},
		"sys.columns c": {
			cols: []string{
				"schema_name", "table_name", "column_name", "data_type", "is_nullable",
				"column_id", "max_length", "precision", "scale", "column_default",
			},
			rows: [][]driver.Value{
				{"sales", "customers", "id", "int", int(0), int64(1), int64(4), int64(10), int64(0), nil},
				{"sales", "customers", "name", "nvarchar", int(1), int64(2), int64(200), int64(0), int64(0), "('')"},
				{"sales", "orders", "id", "int", int(0), int64(1), int64(4), int64(10), int64(0), nil},
				{"sales", "orders", "customer_id", "int", int(0), int64(2), int64(4), int64(10), int64(0), nil},
			},
		},
		"sys.foreign_keys fk": {
			cols: []string{
				"constraint_name", "from_schema", "from_table", "from_column",
				"to_schema", "to_table", "to_column",
			},
			rows: [][]driver.Value{
				{"fk_orders_customer_id", "sales", "orders", "customer_id", "sales", "customers", "id"},
			},
		},
	}

	db, err := sql.Open("sqlserver_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	activeMockConnMutex.Lock()
	activeMockConn = &mockSQLServerConn{queries: queries}
	activeMockConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Schemas
	assert.Len(t, result.Schemas, 2)
	assert.Equal(t, "sales", result.Schemas[0].Name)
	assert.Equal(t, "hr", result.Schemas[1].Name)

	// Tables
	assert.Len(t, result.Tables, 2)
	assert.Equal(t, "customers", result.Tables[0].TableName)
	assert.Equal(t, "orders", result.Tables[1].TableName)

	// Columns
	assert.Len(t, result.Columns, 4)
	assert.Equal(t, "id", result.Columns[0].ColumnName)
	assert.False(t, result.Columns[0].Nullable)
	assert.Equal(t, "name", result.Columns[1].ColumnName)
	assert.True(t, result.Columns[1].Nullable)
	assert.Equal(t, "('')", result.Columns[1].ColumnDefault)

	// Relations
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "fk_orders_customer_id", result.Relations[0].ConstraintName)
	assert.Equal(t, "customer_id", result.Relations[0].FromColumn)
	assert.Equal(t, "customers", result.Relations[0].ToTable)
}
