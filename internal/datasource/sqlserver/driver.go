// Package sqlserver provides a SQL Server datasource driver.
package sqlserver

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/microsoft/go-mssqldb" // SQL Server driver registration
)

// Driver implements datasource.Driver for SQL Server.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new SQL Server driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("sqlserver", "sqlserver", dialect.SQLServerDialect{}, datasource.DefaultPoolLimits()),
	}
}

// Introspect extracts schema metadata from the database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, datasource.IntrospectSteps{
		Schemas:   d.introspectSchemas,
		Tables:    d.introspectTables,
		Columns:   d.introspectColumns,
		Relations: d.introspectRelations,
	})
}

func (d *Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `SELECT name FROM sys.schemas WHERE name NOT IN ('dbo', 'guest', 'INFORMATION_SCHEMA', 'sys')`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.SchemaInfo, error) {
		var s datasource.SchemaInfo
		err := rows.Scan(&s.Name)
		return s, err
	})
}

func (d *Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT s.name, t.name, CASE t.type WHEN 'U' THEN 'BASE TABLE' WHEN 'V' THEN 'VIEW' END, NULL FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id UNION ALL SELECT s.name, v.name, 'VIEW', NULL FROM sys.views v JOIN sys.schemas s ON v.schema_id = s.schema_id`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate)
		return t, err
	})
}

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	// Join sys.objects (not sys.tables only): VIEW columns live under type 'V', tables under 'U'.
	query := `SELECT s.name, o.name, c.name, TYPE_NAME(c.user_type_id), CASE c.is_nullable WHEN 1 THEN 1 ELSE 0 END, c.column_id, c.max_length, c.precision, c.scale, OBJECT_DEFINITION(c.default_object_id)
FROM sys.columns c
JOIN sys.objects o ON c.object_id = o.object_id AND o.type IN ('U', 'V') AND o.is_ms_shipped = 0
JOIN sys.schemas s ON o.schema_id = s.schema_id
ORDER BY s.name, o.name, c.column_id`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
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
	query := `SELECT fk.name, s1.name, OBJECT_NAME(fk.parent_object_id), c1.name, s2.name, OBJECT_NAME(fk.referenced_object_id), c2.name FROM sys.foreign_keys fk JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id JOIN sys.schemas s1 ON fk.schema_id = s1.schema_id JOIN sys.schemas s2 ON fk.referenced_schema_id = s2.schema_id JOIN sys.columns c1 ON fkc.parent_object_id = c1.object_id AND fkc.parent_column_id = c1.column_id JOIN sys.columns c2 ON fkc.referenced_object_id = c2.object_id AND fkc.referenced_column_id = c2.column_id`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.RelationInfo, error) {
		var r datasource.RelationInfo
		r.RelationshipType = "many_to_one"
		err := rows.Scan(&r.ConstraintName, &r.FromSchema, &r.FromTable, &r.FromColumn, &r.ToSchema, &r.ToTable, &r.ToColumn)
		return r, err
	})
}

// Compile-time check
var _ datasource.Driver = (*Driver)(nil)
