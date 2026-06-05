package mysql

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

type mockMySQLConn struct {
	queries map[string]*mockMySQLRows
}

func (*mockMySQLConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockMySQLConn) Close() error {
	return nil
}

func (*mockMySQLConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockMySQLConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockMySQLRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockMySQLRows) Columns() []string {
	return r.cols
}

func (*mockMySQLRows) Close() error {
	return nil
}

func (r *mockMySQLRows) Next(dest []driver.Value) error {
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
	activeMockConn      *mockMySQLConn
	activeMockConnMutex sync.Mutex
)

type mysqlMockBridge struct{}

func (mysqlMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("mysql_mock_bridge", mysqlMockBridge{})
}

func TestMySQLDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "mysql", d.Type())
	assert.Equal(t, dialect.MySQL, d.Dialect())
}

func TestMySQLDriver_Introspect(t *testing.T) {
	queries := map[string]*mockMySQLRows{
		"distinct table_schema": {
			cols: []string{"TABLE_SCHEMA"},
			rows: [][]driver.Value{
				{"sales"},
				{"inventory"},
			},
		},
		"table_schema, table_name, table_type": {
			cols: []string{"TABLE_SCHEMA", "TABLE_NAME", "TABLE_TYPE", "ROW_ESTIMATE"},
			rows: [][]driver.Value{
				{"sales", "customers", "BASE TABLE", nil},
				{"sales", "orders", "BASE TABLE", nil},
			},
		},
		"is_nullable": {
			cols: []string{
				"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE",
				"ORDINAL_POSITION", "CHARACTER_MAXIMUM_LENGTH", "NUMERIC_PRECISION", "NUMERIC_SCALE", "COLUMN_DEFAULT",
			},
			rows: [][]driver.Value{
				{"sales", "customers", "id", "int", int(0), int64(1), nil, nil, nil, nil},
				{"sales", "customers", "name", "varchar", int(1), int64(2), int64(100), nil, nil, nil},
				{"sales", "orders", "id", "int", int(0), int64(1), nil, nil, nil, nil},
				{"sales", "orders", "customer_id", "int", int(0), int64(2), nil, nil, nil, nil},
			},
		},
		"key_column_usage": {
			cols: []string{
				"CONSTRAINT_NAME", "TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME",
				"REFERENCED_TABLE_SCHEMA", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME",
			},
			rows: [][]driver.Value{
				{"fk_orders_customer_id", "sales", "orders", "customer_id", "sales", "customers", "id"},
			},
		},
	}

	db, err := sql.Open("mysql_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	activeMockConnMutex.Lock()
	activeMockConn = &mockMySQLConn{queries: queries}
	activeMockConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Schemas
	assert.Len(t, result.Schemas, 2)
	assert.Equal(t, "sales", result.Schemas[0].Name)
	assert.Equal(t, "inventory", result.Schemas[1].Name)

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

	// Relations
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "fk_orders_customer_id", result.Relations[0].ConstraintName)
	assert.Equal(t, "customer_id", result.Relations[0].FromColumn)
	assert.Equal(t, "customers", result.Relations[0].ToTable)
}
