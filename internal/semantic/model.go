// Package semantic provides business-friendly semantic models over physical database tables.
package semantic

import "time"

// Model defines a business-friendly view over a physical table.
// Deprecated: Use SemanticModel instead; this alias exists for backward compatibility.
type Model = SemanticModel

// SemanticModel defines a business-friendly view over a physical table.
//nolint:revive // 'SemanticModel' is clearer than 'Model' in the semantic package context
type SemanticModel struct {
	ID           string      `json:"id" db:"id"`
	DatasourceID string      `json:"datasource_id" db:"datasource_id"`
	Name         string      `json:"name" db:"name"`
	Label        *string     `json:"label" db:"label"`
	Description  *string     `json:"description" db:"description"`
	BaseSchema   string      `json:"base_schema" db:"base_schema"`
	BaseTable    string      `json:"base_table" db:"base_table"`
	Synonyms     []string    `json:"synonyms" db:"synonyms"`
	IsActive     bool        `json:"is_active" db:"is_active"`
	CreatedBy    *string     `json:"created_by" db:"created_by"`
	Dimensions   []Dimension `json:"dimensions,omitempty"`
	Metrics      []Metric    `json:"metrics,omitempty"`
	Joins        []Join      `json:"joins,omitempty"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" db:"updated_at"`
}

// Dimension represents a group-by-able field in a semantic model.
type Dimension struct {
	ID          string    `json:"id" db:"id"`
	ModelID     string    `json:"model_id" db:"model_id"`
	Name        string    `json:"name" db:"name"`
	Label       *string   `json:"label" db:"label"`
	ColumnRef   string    `json:"column_ref" db:"column_ref"`
	Type        string    `json:"type" db:"type"` // text, number, date, boolean, geo
	Synonyms    []string  `json:"synonyms" db:"synonyms"`
	Description *string   `json:"description" db:"description"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Metric represents an aggregatable field in a semantic model.
type Metric struct {
	ID          string    `json:"id" db:"id"`
	ModelID     string    `json:"model_id" db:"model_id"`
	Name        string    `json:"name" db:"name"`
	Label       *string   `json:"label" db:"label"`
	Expression  string    `json:"expression" db:"expression"`
	Aggregation string    `json:"aggregation" db:"aggregation"` // count, sum, avg, min, max, count_distinct
	Format      *string   `json:"format" db:"format"`
	Synonyms    []string  `json:"synonyms" db:"synonyms"`
	Description *string   `json:"description" db:"description"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// Join defines how tables are joined in a semantic model.
type Join struct {
	ID           string    `json:"id" db:"id"`
	ModelID      string    `json:"model_id" db:"model_id"`
	Name         string    `json:"name" db:"name"`
	FromTable    string    `json:"from_table" db:"from_table"`
	FromColumn   string    `json:"from_column" db:"from_column"`
	ToTable      string    `json:"to_table" db:"to_table"`
	ToColumn     string    `json:"to_column" db:"to_column"`
	JoinType     string    `json:"join_type" db:"join_type"` // LEFT, INNER, RIGHT
	Relationship string    `json:"relationship" db:"relationship"` // many_to_one, one_to_many, one_to_one, many_to_many
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// DimensionType enumerates supported dimension types.
type DimensionType string

// Supported dimension types.
const (
	DimensionTypeText     DimensionType = "text"
	DimensionTypeNumber   DimensionType = "number"
	DimensionTypeDate     DimensionType = "date"
	DimensionTypeBoolean  DimensionType = "boolean"
	DimensionTypeGeo      DimensionType = "geo"
)

// AggregationType enumerates supported aggregation functions.
type AggregationType string

// Supported aggregation functions.
const (
	AggCount          AggregationType = "count"
	AggSum            AggregationType = "sum"
	AggAvg            AggregationType = "avg"
	AggMin            AggregationType = "min"
	AggMax            AggregationType = "max"
	AggCountDistinct  AggregationType = "count_distinct"
)
