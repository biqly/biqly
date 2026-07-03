// Package datasource defines interfaces and types for database driver abstraction.
package datasource

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/dialect"
)

// IntrospectionResult holds the metadata discovered from a database.
type IntrospectionResult struct {
	Schemas   []SchemaInfo   `json:"schemas"`
	Tables    []TableInfo    `json:"tables"`
	Columns   []ColumnInfo   `json:"columns"`
	Relations []RelationInfo `json:"relations"`
}

// SchemaInfo represents a discovered database schema.
type SchemaInfo struct {
	Name string `json:"name"`
}

// TableInfo represents a discovered table.
type TableInfo struct {
	SchemaName  string `json:"schema_name"`
	TableName   string `json:"table_name"`
	TableType   string `json:"table_type"`
	RowEstimate *int64 `json:"row_estimate"`
	Comment     string `json:"comment"`
}

// ColumnInfo represents a discovered column.
type ColumnInfo struct {
	SchemaName       string `json:"schema_name"`
	TableName        string `json:"table_name"`
	ColumnName       string `json:"column_name"`
	DataType         string `json:"data_type"`
	Nullable         bool   `json:"nullable"`
	OrdinalPosition  int    `json:"ordinal_position"`
	CharMaxLength    *int   `json:"character_maximum_length"`
	NumericPrecision *int   `json:"numeric_precision"`
	NumericScale     *int   `json:"numeric_scale"`
	ColumnDefault    string `json:"column_default"`
	Comment          string `json:"comment"`
	IsPrimaryKey     bool   `json:"is_primary_key"`
	IsForeignKey     bool   `json:"is_foreign_key"`
}

// RelationInfo represents a discovered relationship.
type RelationInfo struct {
	ConstraintName   string `json:"constraint_name"`
	FromSchema       string `json:"from_schema"`
	FromTable        string `json:"from_table"`
	FromColumn       string `json:"from_column"`
	ToSchema         string `json:"to_schema"`
	ToTable          string `json:"to_table"`
	ToColumn         string `json:"to_column"`
	RelationshipType string `json:"relationship_type"`
}

// DefaultRelationshipType is the relationship cardinality assumed for foreign
// keys discovered via driver introspection when the database itself does not
// expose cardinality metadata.
const DefaultRelationshipType = "many_to_one"

// Driver is the interface for database datasource operations.
type Driver interface {
	// Type returns the datasource type name (e.g. "postgres", "mysql").
	Type() string

	// Ping tests the connection to the database.
	Ping(ctx context.Context, dsn string) error

	// Open returns a database/sql connection pool.
	Open(ctx context.Context, dsn string) (*sql.DB, error)

	// Introspect reads schema metadata from the database.
	Introspect(ctx context.Context, db *sql.DB) (*IntrospectionResult, error)

	// Dialect returns the SQL dialect for this datasource.
	Dialect() dialect.Dialect

	// SupportsReadOnlyTx reports whether the driver honours a read-only
	// transaction (sql.TxOptions{ReadOnly: true}) as a hard, DB-enforced
	// guarantee. When true, the executor runs queries inside one so a write
	// that slips past the SQL keyword checker still fails at the database.
	SupportsReadOnlyTx() bool
}
