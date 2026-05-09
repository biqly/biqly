package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/go-sql-driver/mysql"
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

func (d *Driver) Type() string {
	return "mysql"
}

func (d *Driver) Ping(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open mysql connection: %w", err)
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func (d *Driver) Dialect() dialect.Dialect {
	return d.dialect
}

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
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, NULL FROM information_schema.tables WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []datasource.TableInfo
	for rows.Next() {
		var t datasource.TableInfo
		if err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, CASE WHEN IS_NULLABLE='YES' THEN 1 ELSE 0 END, ORDINAL_POSITION, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLUMN_DEFAULT FROM information_schema.columns WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []datasource.ColumnInfo
	for rows.Next() {
		var c datasource.ColumnInfo
		var nullable int
		if err := rows.Scan(&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault); err != nil {
			return nil, err
		}
		c.Nullable = nullable == 1
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

func (d *Driver) introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	query := `SELECT CONSTRAINT_NAME, TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []datasource.RelationInfo
	for rows.Next() {
		var r datasource.RelationInfo
		r.RelationshipType = "many_to_one"
		if err := rows.Scan(&r.ConstraintName, &r.FromSchema, &r.FromTable, &r.FromColumn, &r.ToSchema, &r.ToTable, &r.ToColumn); err != nil {
			return nil, err
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// Ensure compile-time checks
var _ datasource.Driver = (*Driver)(nil)
var _ = strings.Contains
