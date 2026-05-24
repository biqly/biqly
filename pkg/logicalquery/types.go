package logicalquery

// CurrentLogicalQueryVersion is the schema version applied to LogicalQuery
// payloads when callers leave Version empty. Bump this only when the LogicalQuery
// shape changes in a way that affects compilation or replay; older values remain
// valid for history/eval traversal and must be migrated explicitly.
const CurrentLogicalQueryVersion = "v1"

// LogicalQuery is the database-independent query representation.
// The AI layer produces LogicalQuery JSON, which the backend validates and compiles to SQL.
type LogicalQuery struct {
	// Version identifies the LogicalQuery schema revision. Persisted in query
	// history and audit logs so eval/replay tools can filter by the shape that
	// produced a given run.
	Version      string       `json:"version,omitempty"`
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
	// FromSubquery wraps a nested query as the primary FROM source instead of
	// the semantic model base table. Use for derived tables / inline views.
	FromSubquery *SubqueryBody `json:"from_subquery,omitempty"`
	// FromCTE names a CTE (defined in CTEs) as the primary FROM source.
	FromCTE string `json:"from_cte,omitempty"`
	// FromAlias is the SQL alias for from_subquery (default: "_sub").
	FromAlias string `json:"from_alias,omitempty"`
	// DefaultSchema overrides the semantic model base_schema when resolving
	// two-part column references (table.column). Three-part refs
	// (schema.table.column) are always explicit.
	DefaultSchema string `json:"default_schema,omitempty"`
	// TableSchemas maps physical table name to schema when it differs from
	// DefaultSchema or the model base schema.
	TableSchemas map[string]string `json:"table_schemas,omitempty"`
}

// SubqueryBody is a nested LogicalQuery fragment without datasource metadata.
// It reuses the parent query's semantic model for field resolution.
type SubqueryBody struct {
	Select  []SelectItem `json:"select"`
	Filters []Filter     `json:"filters,omitempty"`
	GroupBy []GroupBy    `json:"group_by,omitempty"`
	Having  []Filter     `json:"having,omitempty"`
	OrderBy []OrderBy    `json:"order_by,omitempty"`
	Limit   int          `json:"limit,omitempty"`
	Offset  int          `json:"offset,omitempty"`
}

// CTE represents a Common Table Expression (WITH ... AS ...).
type CTE struct {
	Name    string       `json:"name"`
	Select  []SelectItem `json:"select,omitempty"`
	Filters []Filter     `json:"filters,omitempty"`
	GroupBy []GroupBy    `json:"group_by,omitempty"`
	Having  []Filter     `json:"having,omitempty"`
	OrderBy []OrderBy    `json:"order_by,omitempty"`
	Limit   int          `json:"limit,omitempty"`
	Offset  int          `json:"offset,omitempty"`
}

// Subquery returns a SubqueryBody copy of this CTE for shared compilation.
func (c CTE) Subquery() SubqueryBody {
	return SubqueryBody{
		Select:  c.Select,
		Filters: c.Filters,
		GroupBy: c.GroupBy,
		Having:  c.Having,
		OrderBy: c.OrderBy,
		Limit:   c.Limit,
		Offset:  c.Offset,
	}
}

// SelectItem represents a field in the SELECT clause.
type SelectItem struct {
	Type  string `json:"type"` // dimension | metric | window | case
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	// Case is populated only when Type == SelectTypeCase.
	Case *CaseExpr `json:"case,omitempty"`
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
	Aggregation string    `json:"aggregation"`            // sum | avg | count | count_distinct | min | max | row_number | rank | dense_rank | ntile
	Expression  string    `json:"expression,omitempty"`   // raw column ref, optional for ranking functions
	Metric      string    `json:"metric,omitempty"`       // metric name to inherit aggregation+expression from
	PartitionBy []string  `json:"partition_by,omitempty"` // dimension names
	OrderBy     []OrderBy `json:"order_by,omitempty"`     // dimension or metric names
	Frame       string    `json:"frame,omitempty"`        // optional raw frame clause, e.g. "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW"
}

// Filter represents a WHERE condition.
type Filter struct {
	Field         string          `json:"field"`
	Operator      string          `json:"operator"`
	Value         any             `json:"value"`
	CaseSensitive bool            `json:"case_sensitive,omitempty"`
	// Subquery is used with operator in/not_in instead of Value: the outer Field
	// is compared to the single column projected by the nested query.
	Subquery *SubqueryFilter `json:"subquery,omitempty"`
}

// SubqueryFilter embeds a nested query for IN / NOT IN predicates.
type SubqueryFilter struct {
	Body        SubqueryBody `json:"body"`
	ResultField string       `json:"result_field"`
}

// CaseExpr is a structured CASE WHEN for select items (type "case").
type CaseExpr struct {
	Branches []CaseBranch `json:"branches"`
	Else     *CaseThen    `json:"else,omitempty"`
}

// CaseBranch is one WHEN ... THEN arm. All When filters are ANDed.
type CaseBranch struct {
	When []Filter `json:"when"`
	Then CaseThen `json:"then"`
}

// CaseThen is the result of a CASE branch or ELSE.
type CaseThen struct {
	Type      string `json:"type"` // dimension | literal
	Dimension string `json:"dimension,omitempty"`
	Literal   any    `json:"literal,omitempty"`
}

// GroupBy represents a GROUP BY field.
//
// TimeGrain optionally buckets a date/timestamp dimension into calendar parts
// (day | week | month | quarter | year). When non-empty, the compiler applies
// the dialect's DateTrunc/CalendarPart wrapping to the dimension's SELECT
// projection and the GROUP BY expression so both stay consistent. Callers that
// also list the dimension in `select` do NOT need to repeat the grain there —
// the compiler propagates from GroupBy.
type GroupBy struct {
	Field     string `json:"field"`
	TimeGrain string `json:"time_grain,omitempty"`
}

// Supported time grains for GroupBy.TimeGrain.
const (
	TimeGrainDay     = "day"
	TimeGrainWeek    = "week"
	TimeGrainMonth   = "month"
	TimeGrainQuarter = "quarter"
	TimeGrainYear    = "year"
)

// IsValidTimeGrain reports whether the supplied value matches a supported
// grain. Empty is considered valid (means "no bucketing").
func IsValidTimeGrain(grain string) bool {
	switch grain {
	case "", TimeGrainDay, TimeGrainWeek, TimeGrainMonth, TimeGrainQuarter, TimeGrainYear:
		return true
	}
	return false
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
	SelectTypeCase      = "case"
)

// CaseThen kinds.
const (
	CaseThenTypeDimension = "dimension"
	CaseThenTypeLiteral   = "literal"
)

// Order directions.
const (
	OrderAsc  = "asc"
	OrderDesc = "desc"
)

// EnsureVersion stamps the LogicalQuery with the current schema version if the
// caller left Version blank. Callers entering the query pipeline (HTTP handlers,
// AI service, eval runner) should invoke this before persistence so history and
// audit rows always carry an explicit version.
func (lq *LogicalQuery) EnsureVersion() {
	if lq == nil {
		return
	}
	if lq.Version == "" {
		lq.Version = CurrentLogicalQueryVersion
	}
}

// EnsureGroupBySelected makes grouped result sets self-describing by projecting
// every GROUP BY dimension. LLMs sometimes emit metrics in select and put the
// date/category only in group_by; that SQL is valid, but the returned rows lose
// the label that explains each aggregate.
func (lq *LogicalQuery) EnsureGroupBySelected() {
	if lq == nil || len(lq.GroupBy) == 0 {
		return
	}

	selectedDims := make(map[string]bool, len(lq.Select))
	firstMetric := len(lq.Select)
	for i, item := range lq.Select {
		if item.Type == SelectTypeDimension {
			selectedDims[item.Name] = true
		}
		if firstMetric == len(lq.Select) && item.Type == SelectTypeMetric {
			firstMetric = i
		}
	}

	missing := make([]SelectItem, 0, len(lq.GroupBy))
	for _, gb := range lq.GroupBy {
		if selectedDims[gb.Field] {
			continue
		}
		missing = append(missing, SelectItem{Type: SelectTypeDimension, Name: gb.Field})
		selectedDims[gb.Field] = true
	}
	if len(missing) == 0 {
		return
	}

	selectItems := make([]SelectItem, 0, len(lq.Select)+len(missing))
	selectItems = append(selectItems, lq.Select[:firstMetric]...)
	selectItems = append(selectItems, missing...)
	selectItems = append(selectItems, lq.Select[firstMetric:]...)
	lq.Select = selectItems
}
