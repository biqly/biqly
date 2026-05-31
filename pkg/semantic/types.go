package semantic

import "time"

// SemanticModel defines a business-friendly view over a physical table.
type SemanticModel struct {
	ID              string      `json:"id" db:"id"`
	DatasourceID    string      `json:"datasource_id" db:"datasource_id"`
	Name            string      `json:"name" db:"name"`
	Label           *string     `json:"label" db:"label"`
	Description     *string     `json:"description" db:"description"`
	BaseSchema      string      `json:"base_schema" db:"base_schema"`
	BaseTable       string      `json:"base_table" db:"base_table"`
	Synonyms        []string    `json:"synonyms" db:"synonyms"`
	ExcludedSchemas []string    `json:"excluded_schemas" db:"excluded_schemas"`
	IsActive        bool        `json:"is_active" db:"is_active"`
	Status          string      `json:"status" db:"status"`
	Version         int         `json:"version" db:"version"`
	PublishedAt     *time.Time  `json:"published_at,omitempty" db:"published_at"`
	PublishedBy     *string     `json:"published_by,omitempty" db:"published_by"`
	DraftUpdatedAt  time.Time   `json:"draft_updated_at" db:"draft_updated_at"`
	CreatedBy       *string     `json:"created_by" db:"created_by"`
	Dimensions      []Dimension `json:"dimensions,omitempty"`
	Metrics         []Metric    `json:"metrics,omitempty"`
	Joins           []Join      `json:"joins,omitempty"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
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
	// TimeGrain drives calendar bucketing in SQL: year/quarter/month use integer parts (e.g. 2024, 1–4, 1–12);
	// other values fall back to dialect DateTrunc (timestamp buckets).
	TimeGrain string `json:"time_grain,omitempty" db:"time_grain"`
	// IsDisplay marks this dimension as the preferred human-readable display
	// column for its table (e.g. customer.name over customer.id). The AI prompt
	// builder and query planner prioritise display dimensions for SELECT when
	// the user asks for readable labels ("list customers", "names").
	IsDisplay bool `json:"is_display,omitempty" db:"is_display"`
	// CalculatedExpression is an optional SQL expression that derives this
	// dimension's value from other columns. When set, ColumnRef is ignored
	// during compilation. Supported: simple arithmetic (+,-,*,/), scalar
	// functions (COALESCE, CONCAT, UPPER, LOWER, ROUND), CASE WHEN, and
	// dialect-specific date functions.
	CalculatedExpression string `json:"calculated_expression,omitempty" db:"calculated_expression"`
	// EnumValues maps stored raw values to human-readable labels for low
	// cardinality coded columns (e.g. status 1=pending, 2=shipped). They are
	// surfaced to the AI prompt so the model can translate user language into
	// the underlying codes, and their labels feed table routing as synonyms.
	EnumValues []EnumMapping `json:"enum_values,omitempty"`
}

// EnumMapping describes a single raw-value → label pairing for a coded
// dimension. SortOrder controls the presentation order in the prompt and UI.
type EnumMapping struct {
	ID          string  `json:"id" db:"id"`
	DimensionID string  `json:"dimension_id" db:"dimension_id"`
	RawValue    string  `json:"raw_value" db:"raw_value"`
	Label       string  `json:"label" db:"label"`
	Description *string `json:"description,omitempty" db:"description"`
	SortOrder   int     `json:"sort_order" db:"sort_order"`
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
	FromSchema   string    `json:"from_schema,omitempty" db:"from_schema"`
	FromTable    string    `json:"from_table" db:"from_table"`
	FromColumn   string    `json:"from_column" db:"from_column"`
	ToSchema     string    `json:"to_schema,omitempty" db:"to_schema"`
	ToTable      string    `json:"to_table" db:"to_table"`
	ToColumn     string    `json:"to_column" db:"to_column"`
	JoinType     string    `json:"join_type" db:"join_type"`       // LEFT, INNER, RIGHT
	Relationship string    `json:"relationship" db:"relationship"` // many_to_one, one_to_many, one_to_one, many_to_many
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// DimensionType enumerates supported dimension types.
type DimensionType string

// Supported dimension types.
const (
	DimensionTypeText    DimensionType = "text"
	DimensionTypeNumber  DimensionType = "number"
	DimensionTypeDate    DimensionType = "date"
	DimensionTypeBoolean DimensionType = "boolean"
	DimensionTypeGeo     DimensionType = "geo"
)

// AggregationType enumerates supported aggregation functions.
type AggregationType string

// Supported aggregation functions.
const (
	AggCount         AggregationType = "count"
	AggSum           AggregationType = "sum"
	AggAvg           AggregationType = "avg"
	AggMin           AggregationType = "min"
	AggMax           AggregationType = "max"
	AggCountDistinct AggregationType = "count_distinct"
)

// Join defaults and relationship cardinality strings.
const (
	DefaultJoinType         = "LEFT"
	RelationshipManyToOne   = "many_to_one"
	RelationshipOneToMany   = "one_to_many"
	RelationshipOneToOne    = "one_to_one"
	RelationshipManyToMany  = "many_to_many"
	DefaultRelationshipType = RelationshipManyToOne
)
