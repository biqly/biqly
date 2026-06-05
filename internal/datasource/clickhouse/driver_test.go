package clickhouse

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

type mockClickHouseConn struct {
	queries map[string]*mockClickHouseRows
}

func (*mockClickHouseConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockClickHouseConn) Close() error {
	return nil
}

func (*mockClickHouseConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockClickHouseConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockClickHouseRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockClickHouseRows) Columns() []string {
	return r.cols
}

func (*mockClickHouseRows) Close() error {
	return nil
}

func (r *mockClickHouseRows) Next(dest []driver.Value) error {
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
	activeMockConn      *mockClickHouseConn
	activeMockConnMutex sync.Mutex
)

type clickhouseMockBridge struct{}

func (clickhouseMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("clickhouse_mock_bridge", clickhouseMockBridge{})
}

func TestClickHouseDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "clickhouse", d.Type())
	assert.Equal(t, dialect.ClickHouse, d.Dialect())
}

func TestClickHouseDriver_Introspect(t *testing.T) {
	queries := map[string]*mockClickHouseRows{
		"distinct database": {
			cols: []string{"database"},
			rows: [][]driver.Value{
				{"analytics"},
				{"default"},
			},
		},
		"database, name, engine": {
			cols: []string{"database", "name", "engine", "row_estimate"},
			rows: [][]driver.Value{
				{"analytics", "hits", "MergeTree", int64(0)},
				{"analytics", "users", "MergeTree", int64(0)},
			},
		},
		"database, table, name, type": {
			cols: []string{
				"database", "table", "name", "type", "nullable",
				"position", "char_max_len", "numeric_precision", "numeric_scale", "column_default",
			},
			rows: [][]driver.Value{
				{"analytics", "hits", "id", "UInt64", int64(0), int64(1), int64(0), int64(0), int64(0), ""},
				{"analytics", "hits", "title", "String", int64(0), int64(2), int64(0), int64(0), int64(0), ""},
			},
		},
	}

	db, err := sql.Open("clickhouse_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	activeMockConnMutex.Lock()
	activeMockConn = &mockClickHouseConn{queries: queries}
	activeMockConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Schemas
	assert.Len(t, result.Schemas, 2)
	assert.Equal(t, "analytics", result.Schemas[0].Name)
	assert.Equal(t, "default", result.Schemas[1].Name)

	// Tables
	assert.Len(t, result.Tables, 2)
	assert.Equal(t, "hits", result.Tables[0].TableName)
	assert.Equal(t, "users", result.Tables[1].TableName)

	// Columns
	assert.Len(t, result.Columns, 2)
	assert.Equal(t, "id", result.Columns[0].ColumnName)
	assert.Equal(t, "UInt64", result.Columns[0].DataType)
}
