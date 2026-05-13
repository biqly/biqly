// Package clickhouse provides a ClickHouse datasource driver.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/ClickHouse/clickhouse-go/v2" // register clickhouse driver
)

// Driver implements the datasource.Driver interface for ClickHouse.
type Driver struct {
	dialect dialect.Dialect
}

// NewDriver creates a new ClickHouse driver.
func NewDriver() *Driver {
	return &Driver{
		dialect: dialect.ClickHouseDialect{},
	}
}

// Type returns the driver type identifier.
func (d *Driver) Type() string {
	return "clickhouse"
}

// Ping tests connectivity to a ClickHouse instance.
func (d *Driver) Ping(ctx context.Context, dsn string) error {
	if err := datasource.Ping(ctx, "clickhouse", dsn); err != nil {
		return fmt.Errorf("failed to open clickhouse connection: %w", err)
	}
	return nil
}

// Open establishes a connection pool to ClickHouse.
func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := datasource.OpenPool(ctx, "clickhouse", dsn, datasource.ClickHousePoolLimits())
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}
	return db, nil
}

// Dialect returns the ClickHouse SQL dialect.
func (d *Driver) Dialect() dialect.Dialect {
	return d.dialect
}

// Introspect discovers the schema of a ClickHouse database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	schemas, err := d.introspectSchemas(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect schemas: %w", err)
	}

	tables, err := d.introspectTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}

	columns, err := d.introspectColumns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect columns: %w", err)
	}

	relations := []datasource.RelationInfo{}

	return &datasource.IntrospectionResult{
		Schemas:   schemas,
		Tables:    tables,
		Columns:   columns,
		Relations: relations,
	}, nil
}

func (d *Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `SELECT DISTINCT database FROM system.tables WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.SchemaInfo, error) {
		var s datasource.SchemaInfo
		err := rows.Scan(&s.Name)
		return s, err
	})
}

func (d *Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT database, name, engine, 0 FROM system.tables WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.TableInfo, error) {
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

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT database, table, name, type, 0, position, 0, 0, 0, '' FROM system.columns WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY database, table, position`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
		var c datasource.ColumnInfo
		var nullable int
		err := rows.Scan(&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault)
		return c, err
	})
}

var _ datasource.Driver = (*Driver)(nil)
