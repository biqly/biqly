package logicalquery

import pkgsemantic "github.com/biqly/biqly/pkg/semantic"

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
	Version      string `json:"version,omitempty"`
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id"`
	// CompositeID, when set, selects a composite semantic model. The query
	// pipeline resolves it into a merged SemanticModel before compilation.
	// Backward compatible with ModelID: if CompositeID is set the merged model
	// is used, otherwise the single model referenced by ModelID is used.
	CompositeID string       `json:"composite_id,omitempty"`
	Select      []SelectItem `json:"select"`
	Filters     []Filter     `json:"filters,omitempty"`
	GroupBy     []GroupBy    `json:"group_by,omitempty"`
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
func (c *CTE) Subquery() SubqueryBody {
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
	Type  string `json:"type"` // dimension | metric | window | case | formula
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	// Filters, when set on a Type == SelectTypeMetric item, scopes that single
	// aggregate to a row subset independent of the query-level WHERE. The
	// compiler emits a conditional aggregate (e.g. COUNT(CASE WHEN <filters>
	// THEN 1 END)), so one query can place differently-filtered measures side
	// by side — the basis for share-of-total and time-windowed comparisons.
	// All filters are ANDed. Ignored for non-metric select types.
	Filters []Filter `json:"filters,omitempty"`
	// Case is populated only when Type == SelectTypeCase.
	Case *CaseExpr `json:"case,omitempty"`
	// Formula is populated only when Type == SelectTypeFormula. It combines two
	// measures (each optionally filtered) with an arithmetic operator —
	// difference, ratio, percentage, or percent-change — so questions comparing
	// two differently-filtered aggregates (today vs yesterday, this vs last
	// month, part vs whole) compile to a single expression. Division ops are
	// floating-point and guard division-by-zero via NULLIF.
	Formula *FormulaSpec `json:"formula,omitempty"`
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
	Aggregation string               `json:"aggregation"`            // sum | avg | count | min | max | row_number | rank | dense_rank | percent_rank | cume_dist | ntile | lag | lead | first_value | last_value
	Expression  string               `json:"expression,omitempty"`   // raw column ref; the value read by lag/lead/first_value/last_value; the bucket count for ntile; optional for ranking functions
	Expr        pkgsemantic.ExprNode `json:"expr,omitempty"`         // parsed expression AST, optional for ranking functions
	Metric      string               `json:"metric,omitempty"`       // metric name to inherit aggregation+expression from
	PartitionBy []string             `json:"partition_by,omitempty"` // dimension names
	OrderBy     []OrderBy            `json:"order_by,omitempty"`     // dimension or metric names
	// Offset is the row offset for lag/lead (rows back / forward). Defaults to 1
	// when zero or negative. Ignored by other window functions.
	Offset int    `json:"offset,omitempty"`
	Frame  string `json:"frame,omitempty"` // optional raw frame clause, e.g. "ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW"
}

// Filter represents a WHERE condition.
type Filter struct {
	Field         string `json:"field"`
	Operator      string `json:"operator"`
	Value         any    `json:"value"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
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

// FormulaSpec combines two measures with an arithmetic operator. Each side is a
// measure (a metric optionally constrained by its own filters), so the two
// sides can aggregate different row subsets — e.g. today's count vs yesterday's
// count — which a single shared WHERE cannot express. Division operators
// (Divide, PercentOf, PercentChange) compute in floating point and guard
// division by zero with NULLIF, yielding NULL when the right side is zero.
//
// Op semantics (L = Left, R = Right):
//
//	add            → L + R
//	subtract       → L - R          ("fark" / difference)
//	divide         → L / R          ("oran" / ratio, as a fraction e.g. 0.2)
//	percent_of     → L / R * 100    ("yüzde" / share as a percentage e.g. 20)
//	percent_change → (L - R) / R * 100  ("değişim/büyüme oranı" / growth %)
type FormulaSpec struct {
	Op    string     `json:"op"`
	Left  MeasureRef `json:"left"`
	Right MeasureRef `json:"right"`
}

// MeasureRef names a metric and optional per-measure filters. When Filters is
// non-empty the metric is aggregated conditionally over the matching rows; when
// empty the metric aggregates over the full (query-level filtered) row set.
type MeasureRef struct {
	Metric  string   `json:"metric"`
	Filters []Filter `json:"filters,omitempty"`
}

// GroupBy represents a GROUP BY field.
//
// TimeGrain optionally buckets a date/timestamp dimension into calendar parts
// (hour | day | week | month | quarter | year). When non-empty, the compiler applies
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
	TimeGrainHour    = "hour"
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
	case "", TimeGrainHour, TimeGrainDay, TimeGrainWeek, TimeGrainMonth, TimeGrainQuarter, TimeGrainYear:
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
	OpIsEmpty    = "is_empty"
	OpIsNotEmpty = "is_not_empty"
)

// Select types.
const (
	SelectTypeDimension = "dimension"
	SelectTypeMetric    = "metric"
	SelectTypeWindow    = "window"
	SelectTypeCase      = "case"
	SelectTypeFormula   = "formula"
)

// Formula operators for FormulaSpec.Op.
const (
	FormulaOpAdd           = "add"
	FormulaOpSubtract      = "subtract"
	FormulaOpDivide        = "divide"
	FormulaOpPercentOf     = "percent_of"
	FormulaOpPercentChange = "percent_change"
)

// IsValidFormulaOp reports whether op is a supported formula operator.
func IsValidFormulaOp(op string) bool {
	switch op {
	case FormulaOpAdd, FormulaOpSubtract, FormulaOpDivide, FormulaOpPercentOf, FormulaOpPercentChange:
		return true
	}
	return false
}

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
