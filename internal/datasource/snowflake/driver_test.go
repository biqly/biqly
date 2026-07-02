package snowflake

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

	"github.com/stretchr/testify/assert"
)

type mockSnowflakeConn struct {
	queries map[string]*mockSnowflakeRows
}

func (*mockSnowflakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockSnowflakeConn) Close() error {
	return nil
}

func (*mockSnowflakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockSnowflakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockSnowflakeRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockSnowflakeRows) Columns() []string {
	return r.cols
}

func (*mockSnowflakeRows) Close() error {
	return nil
}

func (r *mockSnowflakeRows) Next(dest []driver.Value) error {
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
	activeMockConn      *mockSnowflakeConn
	activeMockConnMutex sync.Mutex
)

type snowflakeMockBridge struct{}

func (snowflakeMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("snowflake_mock_bridge", snowflakeMockBridge{})
}

func TestSnowflakeDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "snowflake", d.Type())
	assert.Equal(t, "snowflake", d.Dialect().Name())
}

func TestSnowflakeDriver_Introspect(t *testing.T) {
	queries := map[string]*mockSnowflakeRows{
		"information_schema.schemata": {
			cols: []string{"schema_name"},
			rows: [][]driver.Value{
				{"ANALYTICS"},
			},
		},
		"information_schema.tables": {
			cols: []string{"table_schema", "table_name", "table_type", "row_count", "comment"},
			rows: [][]driver.Value{
				{"ANALYTICS", "ORDERS", "BASE TABLE", int64(42), "Orders table"},
			},
		},
		"information_schema.columns": {
			cols: []string{
				"table_schema", "table_name", "column_name", "data_type", "is_nullable",
				"ordinal_position", "character_maximum_length", "numeric_precision", "numeric_scale", "column_default",
			},
			rows: [][]driver.Value{
				{"ANALYTICS", "ORDERS", "ID", "NUMBER", int64(0), int64(1), nil, nil, nil, ""},
			},
		},
	}

	db, err := sql.Open("snowflake_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	activeMockConnMutex.Lock()
	activeMockConn = &mockSnowflakeConn{queries: queries}
	activeMockConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Len(t, result.Schemas, 1)
	assert.Len(t, result.Tables, 1)
	assert.Len(t, result.Columns, 1)
	assert.Empty(t, result.Relations)
}
