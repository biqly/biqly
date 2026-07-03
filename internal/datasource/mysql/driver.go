// Package mysql provides a MySQL datasource driver.
package mysql

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/go-sql-driver/mysql" // MySQL driver registration
)

// Driver implements datasource.Driver for MySQL.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new MySQL driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("mysql", "mysql", dialect.MySQL, datasource.DefaultPoolLimits()),
	}
}

// SupportsReadOnlyTx reports whether go-sql-driver/mysql issues START TRANSACTION READ ONLY for
// sql.TxOptions{ReadOnly:true}, so writes inside the transaction are rejected.
func (*Driver) SupportsReadOnlyTx() bool {
	return true
}

// Introspect extracts schema metadata from the database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   d.introspectSchemas,
		Tables:    d.introspectTables,
		Columns:   d.introspectColumns,
		Relations: d.introspectRelations,
	})
}

func (*Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `SELECT DISTINCT TABLE_SCHEMA FROM information_schema.tables WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, nil, datasource.ScanSchemaName)
}

func (*Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, NULL FROM information_schema.tables WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, nil, datasource.ScanTableInfo)
}

func (*Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, DATA_TYPE, CASE WHEN IS_NULLABLE='YES' THEN 1 ELSE 0 END, ORDINAL_POSITION, CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE, COLUMN_DEFAULT FROM information_schema.columns WHERE TABLE_SCHEMA NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')`
	return datasource.QueryAll(ctx, db, query, nil, datasource.ScanStandardColumnInfo)
}

func (*Driver) introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	query := `SELECT CONSTRAINT_NAME, TABLE_SCHEMA, TABLE_NAME, COLUMN_NAME, REFERENCED_TABLE_SCHEMA, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_NAME IS NOT NULL`
	return datasource.QueryAll(ctx, db, query, nil, datasource.ScanForeignKeyRelation)
}

var _ datasource.Driver = (*Driver)(nil)
