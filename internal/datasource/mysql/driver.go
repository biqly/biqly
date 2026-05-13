// Package mysql provides a MySQL datasource driver.
package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/go-sql-driver/mysql" // MySQL driver registration
)

// Driver implements datasource.Driver for MySQL.
type Driver struct {
	dialect dialect.Dialect
}

// NewDriver creates a new MySQL driver.
func NewDriver() *Driver {
	return &Driver{
		dialect: dialect.MySQLDialect{},
	}
}

// Type returns the datasource type.
func (d *Driver) Type() string {
	return "mysql"
}

// Ping verifies connectivity to the database.
func (d *Driver) Ping(ctx context.Context, dsn string) error {
	if err := datasource.Ping(ctx, "mysql", dsn); err != nil {
		return fmt.Errorf("failed to open mysql connection: %w", err)
	}
	return nil
}

// Open creates a new database connection pool.
func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := datasource.OpenPool(ctx, "mysql", dsn, datasource.DefaultPoolLimits())
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	return db, nil
}

// Dialect returns the SQL dialect for this datasource.
func (d *Driver) Dialect() dialect.Dialect {
	return d.dialect
}

// Introspect extracts schema metadata from the database.
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

	relations, err := d.introspectRelations(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect relations: %w", err)
	}

	return &datasource.IntrospectionResult{
		Schemas:   schemas,
		Tables:    tables,
		Columns:   columns,
		Relations: relations,
	}, nil
}

func (d *Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `SELECT DISTINCT TABLE_SCHEMA FROM information_schema.tables WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.SchemaInfo, error) {
		var s datasource.SchemaInfo
		err := rows.Scan(&s.Name)
		return s, err
	})
}

func (d *Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, NULL FROM information_schema.tables WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate)
		return t, err
	})
}

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, CASE WHEN IS_NULLABLE='YES' THEN 1 ELSE 0 END, ORDINAL_POSITION, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLUMN_DEFAULT FROM information_schema.columns WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
		var c datasource.ColumnInfo
		var nullable int
		err := rows.Scan(&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault)
		if err != nil {
			return c, err
		}
		c.Nullable = nullable == 1
		return c, nil
	})
}

func (d *Driver) introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	query := `SELECT CONSTRAINT_NAME, TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL`
	return datasource.QueryAll(ctx, db, query, func(rows *sql.Rows) (datasource.RelationInfo, error) {
		var r datasource.RelationInfo
		r.RelationshipType = "many_to_one"
		err := rows.Scan(&r.ConstraintName, &r.FromSchema, &r.FromTable, &r.FromColumn, &r.ToSchema, &r.ToTable, &r.ToColumn)
		return r, err
	})
}

// Ensure compile-time check
var _ datasource.Driver = (*Driver)(nil)
