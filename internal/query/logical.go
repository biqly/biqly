package query

// LogicalQuery is the database-independent query representation.
// The AI layer produces LogicalQuery JSON, which the backend validates and compiles to SQL.
type LogicalQuery struct {
	DatasourceID string       `json:"datasource_id"`
	ModelID      string       `json:"model_id"`
	Select       []SelectItem `json:"select"`
	Filters      []Filter     `json:"filters,omitempty"`
	GroupBy      []GroupBy    `json:"group_by,omitempty"`
	// Having filters apply AFTER aggregation. Each Field references a metric
	// name in the semantic model; the compiler substitutes the aggregate
	// expression so the dialect emits e.g. SUM(orders.total_amount) > $1.
	Having  []Filter  `json:"having,omitempty"`
	OrderBy []OrderBy `json:"order_by,omitempty"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset,omitempty"`
	// CTEs (Common Table Expressions) — WITH ... AS clauses.
	CTEs []CTE `json:"ctes,omitempty"`
}

// CTE represents a Common Table Expression (WITH ... AS ...).
type CTE struct {
	Name  string       `json:"name"`
	Select []SelectItem `json:"select,omitempty"`
	Filters []Filter   `json:"filters,omitempty"`
	GroupBy []GroupBy  `json:"group_by,omitempty"`
	OrderBy []OrderBy  `json:"order_by,omitempty"`
	Limit   int        `json:"limit,omitempty"`
}

// SelectItem represents a field in the SELECT clause.
type SelectItem struct {
	Type  string `json:"type"` // dimension | metric | window
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	// Window is populated only when Type == SelectTypeWindow. It describes a
	// window/analytic function, e.g. SUM(orders.total_amount) OVER (PARTITION
	// BY customers.country ORDER BY orders.created_at). Field names refer to
	// dimensions; Aggregation/Expression may either reference a metric name
	// (Window.Metric) or be supplied directly (Window.Aggregation +
	// Window.Expression). Ranking functions (row_number, rank, dense_rank,
	// ntile) ignore Expression.
	Window *WindowSpec `json:"window,omitempty"`
}

// WindowSpec describes an analytic window expression.
type WindowSpec struct {
	Aggregation string    `json:"aggregation"`           // sum | avg | count | count_distinct | min | max | row_number | rank | dense_rank | ntile
	Expression  string    `json:"expression,omitempty"`  // raw column ref, optional for ranking functions
	Metric      string    `json:"metric,omitempty"`      // metric name to inherit aggregation+expression from
	PartitionBy []string  `json:"partition_by,omitempty"` // dimension names
	OrderBy     []OrderBy `json:"order_by,omitempty"`     // dimension or metric names
	Frame       string    `json:"frame,omitempty"`        // optional raw frame clause, e.g. "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW"
}

// Filter represents a WHERE condition.
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
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
	OpIsNull     = "is_null"
	OpIsNotNull  = "is_not_null"
)

// Select types.
const (
	SelectTypeDimension = "dimension"
	SelectTypeMetric    = "metric"
	SelectTypeWindow    = "window"
)

// Order directions.
const (
	OrderAsc  = "asc"
	OrderDesc = "desc"
)
