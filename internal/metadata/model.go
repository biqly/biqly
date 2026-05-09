// Package metadata defines types for datasource metadata storage.
package metadata

import "time"

// Datasource represents a configured database connection.
type Datasource struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Type         string    `json:"type" db:"type"`
	DSNEncrypted string    `json:"-" db:"dsn_encrypted"`
	Config       string    `json:"config" db:"config"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	LastSyncAt   *time.Time `json:"last_sync_at" db:"last_sync_at"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Schema represents a database schema within a datasource.
type Schema struct {
	ID           string    `json:"id" db:"id"`
	DatasourceID string    `json:"datasource_id" db:"datasource_id"`
	SchemaName   string    `json:"schema_name" db:"schema_name"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Table represents a database table.
type Table struct {
	ID           string    `json:"id" db:"id"`
	DatasourceID string    `json:"datasource_id" db:"datasource_id"`
	SchemaID     string    `json:"schema_id" db:"schema_id"`
	SchemaName   string    `json:"schema_name" db:"schema_name"`
	TableName    string    `json:"table_name" db:"table_name"`
	TableType    string    `json:"table_type" db:"table_type"`
	RowEstimate  *int64    `json:"row_estimate" db:"row_estimate"`
	Description  *string   `json:"description" db:"description"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// Column represents a column within a table.
type Column struct {
	ID                     string    `json:"id" db:"id"`
	DatasourceID           string    `json:"datasource_id" db:"datasource_id"`
	TableID                string    `json:"table_id" db:"table_id"`
	SchemaName             string    `json:"schema_name" db:"schema_name"`
	TableName              string    `json:"table_name" db:"table_name"`
	ColumnName             string    `json:"column_name" db:"column_name"`
	DataType               string    `json:"data_type" db:"data_type"`
	Nullable               bool      `json:"nullable" db:"nullable"`
	OrdinalPosition        *int      `json:"ordinal_position" db:"ordinal_position"`
	CharMaxLength          *int      `json:"character_maximum_length" db:"character_maximum_length"`
	NumericPrecision       *int      `json:"numeric_precision" db:"numeric_precision"`
	NumericScale           *int      `json:"numeric_scale" db:"numeric_scale"`
	ColumnDefault          *string   `json:"column_default" db:"column_default"`
	Description            *string   `json:"description" db:"description"`
	IsPrimaryKey           bool      `json:"is_primary_key" db:"is_primary_key"`
	IsForeignKey           bool      `json:"is_foreign_key" db:"is_foreign_key"`
	ReferencedSchema       *string   `json:"referenced_schema" db:"referenced_schema"`
	ReferencedTable        *string   `json:"referenced_table" db:"referenced_table"`
	ReferencedColumn       *string   `json:"referenced_column" db:"referenced_column"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
}

// Relation represents a foreign key relationship.
type Relation struct {
	ID             string    `json:"id" db:"id"`
	DatasourceID   string    `json:"datasource_id" db:"datasource_id"`
	ConstraintName string    `json:"constraint_name" db:"constraint_name"`
	FromSchema     string    `json:"from_schema" db:"from_schema"`
	FromTable      string    `json:"from_table" db:"from_table"`
	FromColumn     string    `json:"from_column" db:"from_column"`
	ToSchema       string    `json:"to_schema" db:"to_schema"`
	ToTable        string    `json:"to_table" db:"to_table"`
	ToColumn       string    `json:"to_column" db:"to_column"`
	RelationshipType string  `json:"relationship_type" db:"relationship_type"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}
