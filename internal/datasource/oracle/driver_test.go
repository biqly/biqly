package oracle

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

type mockOracleConn struct {
	queries map[string]*mockOracleRows
}

func (*mockOracleConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockOracleConn) Close() error {
	return nil
}

func (*mockOracleConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockOracleConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	var bestKey string
	var bestRows *mockOracleRows
	for k, v := range c.queries {
		key := strings.ToLower(k)
		if strings.Contains(normalized, key) && len(key) > len(bestKey) {
			bestKey = key
			bestRows = v
		}
	}
	if bestRows != nil {
		bestRows.pos = 0
		return bestRows, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockOracleRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockOracleRows) Columns() []string {
	return r.cols
}

func (*mockOracleRows) Close() error {
	return nil
}

func (r *mockOracleRows) Next(dest []driver.Value) error {
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
	activeMockOracleConn      *mockOracleConn
	activeMockOracleConnMutex sync.Mutex
)

type oracleMockBridge struct{}

func (oracleMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockOracleConnMutex.Lock()
	defer activeMockOracleConnMutex.Unlock()
	return activeMockOracleConn, nil
}

func init() {
	sql.Register("oracle_mock_bridge", oracleMockBridge{})
}

func TestOracleDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "oracle", d.Type())
	assert.Equal(t, dialect.Oracle, d.Dialect())
	assert.Equal(t, "oracle", d.Dialect().Name())
}

func TestOracleDriver_Introspect(t *testing.T) {
	queries := map[string]*mockOracleRows{
		"all_users": {
			cols: []string{"username"},
			rows: [][]driver.Value{
				{"ANALYTICS"},
				{"SALES"},
			},
		},
		"all_tables": {
			cols: []string{"owner", "table_name", "table_type", "num_rows", "comments"},
			rows: [][]driver.Value{
				{"ANALYTICS", "CUSTOMERS", "BASE TABLE", int64(42), "Customer master"},
				{"SALES", "ORDERS", "BASE TABLE", int64(100), "Orders"},
			},
		},
		"all_tab_columns": {
			cols: []string{
				"owner", "table_name", "column_name", "data_type", "nullable",
				"column_id", "char_length", "data_precision", "data_scale", "column_default",
			},
			rows: [][]driver.Value{
				{"ANALYTICS", "CUSTOMERS", "ID", "NUMBER", int64(0), int64(1), int64(0), int64(10), int64(0), ""},
				{"ANALYTICS", "CUSTOMERS", "NAME", "VARCHAR2", int64(1), int64(2), int64(255), nil, nil, ""},
				{"SALES", "ORDERS", "CUSTOMER_ID", "NUMBER", int64(0), int64(1), int64(0), int64(10), int64(0), ""},
			},
		},
		"constraint_type = 'p'": {
			cols: []string{"owner", "table_name", "column_name"},
			rows: [][]driver.Value{
				{"ANALYTICS", "CUSTOMERS", "ID"},
			},
		},
		"constraint_type = 'r'": {
			cols: []string{"constraint_name", "owner", "table_name", "column_name", "r_owner", "r_table_name", "r_column_name"},
			rows: [][]driver.Value{
				{"FK_ORDERS_CUSTOMERS", "SALES", "ORDERS", "CUSTOMER_ID", "ANALYTICS", "CUSTOMERS", "ID"},
			},
		},
	}

	db, err := sql.Open("oracle_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	activeMockOracleConnMutex.Lock()
	activeMockOracleConn = &mockOracleConn{queries: queries}
	activeMockOracleConnMutex.Unlock()

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Len(t, result.Schemas, 2)
	assert.Equal(t, "ANALYTICS", result.Schemas[0].Name)
	assert.Equal(t, "SALES", result.Schemas[1].Name)

	assert.Len(t, result.Tables, 2)
	assert.Equal(t, "CUSTOMERS", result.Tables[0].TableName)
	assert.Equal(t, "Customer master", result.Tables[0].Comment)
	assert.Equal(t, "ORDERS", result.Tables[1].TableName)

	assert.Len(t, result.Columns, 3)
	assert.Equal(t, "ID", result.Columns[0].ColumnName)
	assert.True(t, result.Columns[0].IsPrimaryKey)
	assert.False(t, result.Columns[1].IsPrimaryKey)
	assert.Equal(t, "VARCHAR2", result.Columns[1].DataType)

	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "FK_ORDERS_CUSTOMERS", result.Relations[0].ConstraintName)
	assert.Equal(t, "SALES", result.Relations[0].FromSchema)
	assert.Equal(t, "ORDERS", result.Relations[0].FromTable)
	assert.Equal(t, "CUSTOMER_ID", result.Relations[0].FromColumn)
	assert.Equal(t, "ANALYTICS", result.Relations[0].ToSchema)
	assert.Equal(t, "CUSTOMERS", result.Relations[0].ToTable)
	assert.Equal(t, "ID", result.Relations[0].ToColumn)
}
