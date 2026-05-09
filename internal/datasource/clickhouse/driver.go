// Package clickhouse provides a ClickHouse datasource driver.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

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
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return fmt.Errorf("failed to open clickhouse connection: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("failed to close clickhouse connection", "error", closeErr)
		}
	}()
	return db.PingContext(ctx)
}

// Open establishes a connection pool to ClickHouse.
func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
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
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var schemas []datasource.SchemaInfo
	for rows.Next() {
		var s datasource.SchemaInfo
		if err := rows.Scan(&s.Name); err != nil {
			return nil, err
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
}

func (d *Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT database, name, engine, 0 FROM system.tables WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []datasource.TableInfo
	for rows.Next() {
		var t datasource.TableInfo
		var engine string
		if err := rows.Scan(&t.SchemaName, &t.TableName, &engine, &t.RowEstimate); err != nil {
			return nil, err
		}
		t.TableType = "BASE TABLE"
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT database, table, name, type, 0, position, 0, 0, 0, '' FROM system.columns WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY database, table, position`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []datasource.ColumnInfo
	for rows.Next() {
		var c datasource.ColumnInfo
		var nullable int
		if err := rows.Scan(&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault); err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

var _ datasource.Driver = (*Driver)(nil)

var _ = strings.Contains
