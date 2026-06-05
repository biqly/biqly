// Package clickhouse provides a ClickHouse datasource driver.
package clickhouse

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/ClickHouse/clickhouse-go/v2" // register clickhouse driver
)

// Driver implements the datasource.Driver interface for ClickHouse.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new ClickHouse driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("clickhouse", "clickhouse", dialect.ClickHouse, datasource.ClickHousePoolLimits()),
	}
}

// Introspect discovers the schema of a ClickHouse database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, datasource.IntrospectSteps{
		Schemas:   d.introspectSchemas,
		Tables:    d.introspectTables,
		Columns:   d.introspectColumns,
		Relations: d.introspectRelations,
	})
}

func (*Driver) introspectRelations(_ context.Context, _ *sql.DB) ([]datasource.RelationInfo, error) {
	return nil, nil
}

func (*Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `SELECT DISTINCT database FROM system.tables WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.SchemaInfo, error) {
		var s datasource.SchemaInfo
		err := rows.Scan(&s.Name)
		return s, err
	})
}

func (*Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT database, name, engine, 0 FROM system.tables WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		var engine string
		err := rows.Scan(&t.SchemaName, &t.TableName, &engine, &t.RowEstimate)
		if err != nil {
			return t, err
		}
		t.TableType = "BASE TABLE"
		return t, nil
	})
}

func (*Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT database, table, name, type, 0, position, 0, 0, 0, '' FROM system.columns WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY database, table, position`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
		var c datasource.ColumnInfo
		var nullable int
		err := rows.Scan(&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault)
		return c, err
	})
}

var _ datasource.Driver = (*Driver)(nil)
