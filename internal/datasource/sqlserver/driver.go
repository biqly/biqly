package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/microsoft/go-mssqldb"
)

// Driver implements datasource.Driver for SQL Server.
type Driver struct {
	dialect dialect.Dialect
}

// NewDriver creates a new SQL Server driver.
func NewDriver() *Driver {
	return &Driver{
		dialect: dialect.SQLServerDialect{},
	}
}

func (d *Driver) Type() string {
	return "sqlserver"
}

func (d *Driver) Ping(ctx context.Context, dsn string) error {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlserver connection: %w", err)
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlserver connection: %w", err)
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
	query := `SELECT name FROM sys.schemas WHERE name NOT IN ('dbo', 'guest', 'INFORMATION_SCHEMA', 'sys')`
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
	query := `SELECT s.name, t.name, CASE t.type WHEN 'U' THEN 'BASE TABLE' WHEN 'V' THEN 'VIEW' END, NULL FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id UNION ALL SELECT s.name, v.name, 'VIEW', NULL FROM sys.views v JOIN sys.schemas s ON v.schema_id = s.schema_id`
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
	query := `SELECT s.name, t.name, c.name, TYPE_NAME(c.user_type_id), CASE c.is_nullable WHEN 1 THEN 1 ELSE 0 END, c.column_id, c.max_length, c.precision, c.scale, OBJECT_DEFINITION(c.default_object_id) FROM sys.columns c JOIN sys.tables t ON c.object_id = t.object_id JOIN sys.schemas s ON t.schema_id = s.schema_id ORDER BY s.name, t.name, c.column_id`
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
	query := `SELECT fk.name, s1.name, OBJECT_NAME(fk.parent_object_id), c1.name, s2.name, OBJECT_NAME(fk.referenced_object_id), c2.name FROM sys.foreign_keys fk JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id JOIN sys.schemas s1 ON fk.schema_id = s1.schema_id JOIN sys.schemas s2 ON fk.referenced_schema_id = s2.schema_id JOIN sys.columns c1 ON fkc.parent_object_id = c1.object_id AND fkc.parent_column_id = c1.column_id JOIN sys.columns c2 ON fkc.referenced_object_id = c2.object_id AND fkc.referenced_column_id = c2.column_id`
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

// Compile-time check
var _ datasource.Driver = (*Driver)(nil)
var _ = strings.Contains
