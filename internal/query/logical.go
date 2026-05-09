package query

// LogicalQuery is the database-independent query representation.
// The AI layer produces LogicalQuery JSON, which the backend validates and compiles to SQL.
type LogicalQuery struct {
	DatasourceID string       `json:"datasource_id"`
	ModelID      string       `json:"model_id"`
	Select       []SelectItem `json:"select"`
	Filters      []Filter     `json:"filters,omitempty"`
	GroupBy      []GroupBy    `json:"group_by,omitempty"`
	OrderBy      []OrderBy    `json:"order_by,omitempty"`
	Limit        int          `json:"limit"`
	Offset       int          `json:"offset,omitempty"`
}

// SelectItem represents a field in the SELECT clause.
type SelectItem struct {
	Type  string `json:"type"` // dimension | metric
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

// Filter represents a WHERE condition.
type Filter struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// GroupBy represents a GROUP BY field.
type GroupBy struct {
	Field string `json:"field"`
}

// OrderBy represents an ORDER BY field.
type OrderBy struct {
	Field     string `json:"field"`
	Direction string `json:"direction"` // asc | desc
}

// Supported filter operators.
const (
	OpEq         = "eq"
	OpNeq        = "neq"
	OpGt         = "gt"
	OpGte        = "gte"
	OpLt         = "lt"
	OpLte        = "lte"
	OpIn         = "in"
	OpNotIn      = "not_in"
	OpContains   = "contains"
	OpStartsWith = "starts_with"
	OpEndsWith   = "ends_with"
	OpBetween    = "between"
	OpIsNull   = "is_null"
	OpIsNotNull = "is_not_null"
)

// Select types.
const (
	SelectTypeDimension = "dimension"
	SelectTypeMetric    = "metric"
)

// Order directions.
const (
	OrderAsc  = "asc"
	OrderDesc = "desc"
)
