package databricks

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

type mockDatabricksConn struct {
	queries map[string]*mockDatabricksRows
	errors  map[string]error
}

func (*mockDatabricksConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not implemented")
}

func (*mockDatabricksConn) Close() error {
	return nil
}

func (*mockDatabricksConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not implemented")
}

func (c *mockDatabricksConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	for k, err := range c.errors {
		if strings.Contains(normalized, strings.ToLower(k)) {
			return nil, err
		}
	}
	for k, v := range c.queries {
		if strings.Contains(normalized, strings.ToLower(k)) {
			v.pos = 0
			return v, nil
		}
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type mockDatabricksRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockDatabricksRows) Columns() []string {
	return r.cols
}

func (*mockDatabricksRows) Close() error {
	return nil
}

func (r *mockDatabricksRows) Next(dest []driver.Value) error {
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
	activeMockConn      *mockDatabricksConn
	activeMockConnMutex sync.Mutex
)

type databricksMockBridge struct{}

func (databricksMockBridge) Open(_ string) (driver.Conn, error) {
	activeMockConnMutex.Lock()
	defer activeMockConnMutex.Unlock()
	return activeMockConn, nil
}

func init() {
	sql.Register("databricks_mock_bridge", databricksMockBridge{})
}

func TestDatabricksDriver_Metadata(t *testing.T) {
	d := NewDriver()
	assert.NotNil(t, d)
	assert.Equal(t, "databricks", d.Type())
	assert.Equal(t, "databricks", d.Dialect().Name())
}

func TestDatabricksDriver_Introspect(t *testing.T) {
	queries := databricksQueries()
	queries["information_schema.referential_constraints"] = &mockDatabricksRows{
		cols: []string{
			"constraint_name", "table_schema", "table_name", "column_name",
			"foreign_table_schema", "foreign_table_name", "foreign_column_name",
		},
		rows: [][]driver.Value{
			{"orders_customer_id_fk", "main", "orders", "customer_id", "main", "customers", "id"},
		},
	}

	db := openMockDB(t, &mockDatabricksConn{queries: queries})

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Len(t, result.Schemas, 1)
	assert.Equal(t, "main", result.Schemas[0].Name)
	assert.Len(t, result.Tables, 1)
	assert.Equal(t, "orders", result.Tables[0].TableName)
	assert.Len(t, result.Columns, 1)
	assert.Equal(t, "customer_id", result.Columns[0].ColumnName)
	assert.Len(t, result.Relations, 1)
	assert.Equal(t, "orders_customer_id_fk", result.Relations[0].ConstraintName)
	assert.Equal(t, "orders", result.Relations[0].FromTable)
	assert.Equal(t, "customers", result.Relations[0].ToTable)
}

func TestDatabricksDriver_IntrospectRelationsDegradesOnError(t *testing.T) {
	db := openMockDB(t, &mockDatabricksConn{
		queries: databricksQueries(),
		errors: map[string]error{
			"information_schema.referential_constraints": errors.New("referential constraints unavailable"),
		},
	})

	d := NewDriver()
	result, err := d.Introspect(context.Background(), db)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Relations)
}

func databricksQueries() map[string]*mockDatabricksRows {
	return map[string]*mockDatabricksRows{
		"information_schema.schemata": {
			cols: []string{"schema_name"},
			rows: [][]driver.Value{
				{"main"},
			},
		},
		"information_schema.tables": {
			cols: []string{"table_schema", "table_name", "table_type", "row_count", "comment"},
			rows: [][]driver.Value{
				{"main", "orders", "BASE TABLE", nil, "Orders table"},
			},
		},
		"information_schema.columns": {
			cols: []string{
				"table_schema", "table_name", "column_name", "data_type", "is_nullable",
				"ordinal_position", "character_maximum_length", "numeric_precision", "numeric_scale", "column_default",
			},
			rows: [][]driver.Value{
				{"main", "orders", "customer_id", "BIGINT", int64(0), int64(1), nil, nil, nil, ""},
			},
		},
	}
}

func openMockDB(t *testing.T, conn *mockDatabricksConn) *sql.DB {
	t.Helper()

	db, err := sql.Open("databricks_mock_bridge", "mock-dsn")
	assert.NoError(t, err)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
		activeMockConnMutex.Lock()
		activeMockConn = nil
		activeMockConnMutex.Unlock()
	})

	activeMockConnMutex.Lock()
	activeMockConn = conn
	activeMockConnMutex.Unlock()

	return db
}
