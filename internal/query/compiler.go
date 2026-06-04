package query

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// reBracket matches square-bracketed identifiers in SQL Server style.
// The previous reWhere/reGroupBy/reOrderBy/reLimit/reOffset regexes were
// used by an earlier CompileWithPermissions implementation that performed
// regex surgery on the assembled SQL — they were removed once filters were
// merged into the WHERE clause at compile time.
var reBracket = regexp.MustCompile(`\[([^\]]+)\]`)

// Compiler compiles a LogicalQuery into dialect-specific SQL.
type Compiler struct {
	dialect dialect.Dialect
	// pii holds the per-user PII masking policy for the current compilation.
	// Nil means no masking. Set via CompileWithPermissions only.
	pii *PIIMaskingConfig
}

// NewCompiler creates a new SQL compiler for the given dialect.
func NewCompiler(d dialect.Dialect) *Compiler {
	switch concrete := d.(type) {
	case dialect.PostgresDialect:
		if concrete.QuoteLeft == "" {
			d = dialect.Postgres
		}
	case dialect.MySQLDialect:
		if concrete.QuoteLeft == "" {
			d = dialect.MySQL
		}
	case dialect.SQLServerDialect:
		if concrete.QuoteLeft == "" {
			d = dialect.SQLServer
		}
	case dialect.ClickHouseDialect:
		if concrete.QuoteLeft == "" {
			d = dialect.ClickHouse
		}
	}
	return &Compiler{dialect: d}
}

// Compile converts a LogicalQuery + semantic model into SQL.
func (c *Compiler) Compile(ctx context.Context, lq *LogicalQuery, model *semantic.SemanticModel) (*CompiledQuery, error) {
	args := make([]any, 0, 8)
	withPrefix, err := c.buildWithClause(lq.CTEs, model, &args)
	if err != nil {
		return nil, err
	}
	fromClause, err := c.resolveFromClause(lq, model, &args)
	if err != nil {
		return nil, err
	}
	return c.compileStatement(ctx, lq, model, fromClause, withPrefix, &args, nil)
}

// CompileWithPermissions compiles a LogicalQuery with row-level security
// filters merged into the WHERE clause at assembly time. This replaces an
// earlier implementation that injected the filters via regex surgery on the
// finished SQL — that approach could match the wrong WHERE keyword (e.g.
// one inside a CTE) and produce dangerous SQL.
//
// piiConfig applies column-level PII masking to the projection (and rejects
// filters on hidden columns). Nil piiConfig means no masking.
func (c *Compiler) CompileWithPermissions(
	ctx context.Context,
	lq *LogicalQuery,
	model *semantic.SemanticModel,
	rowFilters []security.RowFilter,
	piiConfig *PIIMaskingConfig,
) (*CompiledQuery, error) {
	if piiConfig != nil {
		// Clone so the masking policy is scoped to this compilation; nested
		// subquery/CTE compilation reuses the same compiler and must apply
		// the same policy.
		c = &Compiler{dialect: c.dialect, pii: piiConfig}
	}
	if len(rowFilters) == 0 && piiConfig == nil {
		return c.Compile(ctx, lq, model)
	}
	args := make([]any, 0, 8)
	withPrefix, err := c.buildWithClause(lq.CTEs, model, &args)
	if err != nil {
		return nil, err
	}
	fromClause, err := c.resolveFromClause(lq, model, &args)
	if err != nil {
		return nil, err
	}
	return c.compileStatement(ctx, lq, model, fromClause, withPrefix, &args, rowFilters)
}

// buildRowFilterPreds turns the policy row filters into SQL predicate
// fragments, appending their bind args to args. Returns nil when no filters
// apply (model has no matching dimension/metric).
func (c *Compiler) buildRowFilterPreds(filters []security.RowFilter, model *semantic.SemanticModel, args *[]any) ([]string, error) {
	if len(filters) == 0 {
		return nil, nil
	}
	dimMap := make(map[string]string, len(model.Dimensions)+len(model.Metrics))
	for _, d := range model.Dimensions {
		dimMap[d.Name] = d.ColumnRef
	}
	for _, m := range model.Metrics {
		dimMap[m.Name] = m.Expression
	}
	preds, extraArgs, err := security.BuildRowFilterPredicates(c.dialect, dimMap, filters, len(*args), true)
	if err != nil {
		return nil, err
	}
	*args = append(*args, extraArgs...)
	return preds, nil
}

func addTableFromColumnRef(tables map[string]struct{}, colRef string, resolver *SchemaResolver) {
	if p, ok := resolver.ParseColumnRef(colRef); ok {
		tables[TableKey(p.Schema, p.Table)] = struct{}{}
	}
}

func tablesReferencedInLogicalQuery(
	lq *LogicalQuery,
	model *semantic.SemanticModel,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) map[string]struct{} {
	tables := make(map[string]struct{}, len(lq.Select)+len(lq.Filters)+1)
	tables[TableKey(model.BaseSchema, model.BaseTable)] = struct{}{}

	for _, item := range lq.Select {
		switch item.Type {
		case SelectTypeDimension:
			if dim, ok := dimMap[item.Name]; ok {
				addTableFromColumnRef(tables, dim.ColumnRef, resolver)
			}
		case SelectTypeMetric:
			if m, ok := metricMap[item.Name]; ok {
				addTableFromColumnRef(tables, m.Expression, resolver)
			}
		case SelectTypeWindow:
			if item.Window == nil {
				continue
			}
			if mname := item.Window.Metric; mname != "" {
				if m, ok := metricMap[mname]; ok {
					addTableFromColumnRef(tables, m.Expression, resolver)
				}
			}
			if expr := item.Window.Expression; expr != "" {
				addTableFromColumnRef(tables, expr, resolver)
			}
			for _, p := range item.Window.PartitionBy {
				if dim, ok := dimMap[p]; ok {
					addTableFromColumnRef(tables, dim.ColumnRef, resolver)
				}
			}
			for _, ob := range item.Window.OrderBy {
				if dim, ok := dimMap[ob.Field]; ok {
					addTableFromColumnRef(tables, dim.ColumnRef, resolver)
				}
				if m, ok := metricMap[ob.Field]; ok {
					addTableFromColumnRef(tables, m.Expression, resolver)
				}
			}
		}
	}

	for _, f := range lq.Having {
		if m, ok := metricMap[f.Field]; ok {
			addTableFromColumnRef(tables, m.Expression, resolver)
		}
	}

	for _, f := range lq.Filters {
		if dim, ok := dimMap[f.Field]; ok {
			addTableFromColumnRef(tables, dim.ColumnRef, resolver)
		}
		if m, ok := metricMap[f.Field]; ok {
			addTableFromColumnRef(tables, m.Expression, resolver)
		}
	}

	for _, gb := range lq.GroupBy {
		if dim, ok := dimMap[gb.Field]; ok {
			addTableFromColumnRef(tables, dim.ColumnRef, resolver)
		}
	}

	for _, ob := range lq.OrderBy {
		if dim, ok := dimMap[ob.Field]; ok {
			addTableFromColumnRef(tables, dim.ColumnRef, resolver)
		}
		if m, ok := metricMap[ob.Field]; ok {
			addTableFromColumnRef(tables, m.Expression, resolver)
		}
	}

	return tables
}

type joinNeighbor struct {
	table    string
	joinName string
}

// determineJoins returns joins on paths from the base table to every table referenced
// in the logical query. This avoids emitting duplicate joins to the same physical table
// when multiple FKs exist but the query only uses base-table columns.
func (c *Compiler) determineJoins(
	lq *LogicalQuery,
	model *semantic.SemanticModel,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) []string {
	neededTables := tablesReferencedInLogicalQuery(lq, model, dimMap, metricMap, resolver)
	if len(model.Joins) == 0 {
		return nil
	}

	neighbors := make(map[string][]joinNeighbor, len(model.Joins))
	for _, j := range model.Joins {
		fromKey := resolver.JoinSideKey(j.FromSchema, j.FromTable)
		toKey := resolver.JoinSideKey(j.ToSchema, j.ToTable)
		neighbors[fromKey] = append(neighbors[fromKey], joinNeighbor{toKey, j.Name})
		neighbors[toKey] = append(neighbors[toKey], joinNeighbor{fromKey, j.Name})
	}

	base := TableKey(model.BaseSchema, model.BaseTable)
	type parentInfo struct {
		prev string
		join string
	}
	parent := make(map[string]parentInfo, len(model.Joins))
	joinDiscovery := make([]string, 0, len(model.Joins))
	joinFirst := make(map[string]struct{}, len(model.Joins))

	queue := []string{base}
	parent[base] = parentInfo{"", ""}
	visited := map[string]struct{}{base: {}}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, nb := range neighbors[u] {
			if _, ok := visited[nb.table]; ok {
				continue
			}
			visited[nb.table] = struct{}{}
			parent[nb.table] = parentInfo{u, nb.joinName}
			if _, ok := joinFirst[nb.joinName]; !ok {
				joinFirst[nb.joinName] = struct{}{}
				joinDiscovery = append(joinDiscovery, nb.joinName)
			}
			queue = append(queue, nb.table)
		}
	}

	required := make(map[string]struct{}, len(lq.Select)+len(lq.Filters)+1)
	for t := range neededTables {
		if t == base {
			continue
		}
		cur := t
		for cur != base && cur != "" {
			pi := parent[cur]
			if pi.join == "" {
				break
			}
			required[pi.join] = struct{}{}
			cur = pi.prev
		}
	}

	out := make([]string, 0, len(required))
	for _, jn := range joinDiscovery {
		if _, ok := required[jn]; ok {
			out = append(out, jn)
		}
	}
	return out
}

func (c *Compiler) dimensionSQL(dim *semantic.Dimension, resolver *SchemaResolver) string {
	if dim.CalculatedExpr != nil {
		return CompileExpr(dim.CalculatedExpr, c.dialect, resolver)
	}
	if expr := strings.TrimSpace(dim.CalculatedExpression); expr != "" {
		if parsed, err := ParseExpression(expr); err == nil {
			return CompileExpr(parsed, c.dialect, resolver)
		}
		return dim.CalculatedExpression
	}
	colRef := resolver.PhysicalColumnRef(dim.ColumnRef)
	if strings.TrimSpace(dim.TimeGrain) == "" {
		return c.dialect.QuoteIdent(colRef)
	}
	part := strings.ToLower(strings.TrimSpace(dim.TimeGrain))
	switch part {
	case "year", "quarter", "month":
		return c.dialect.CalendarPart(part, colRef)
	default:
		return c.dialect.DateTrunc(part, colRef)
	}
}

func (c *Compiler) resolveBracketExpressions(
	expr string,
	resolver *SchemaResolver,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
) string {
	if expr == "*" {
		return expr
	}
	if !strings.Contains(expr, "[") {
		if _, ok := resolver.ParseColumnRef(expr); ok {
			return resolver.PhysicalColumnRef(expr)
		}
		return expr
	}

	return reBracket.ReplaceAllStringFunc(expr, func(match string) string {
		token := match[1 : len(match)-1]
		return c.resolveCustomToken(token, resolver, dimMap, metricMap, model)
	})
}

func (c *Compiler) resolveCustomToken(
	token string,
	resolver *SchemaResolver,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
) string {
	token = strings.TrimSpace(token)
	for name, dim := range dimMap {
		if strings.EqualFold(name, token) {
			return c.dimensionOutputSQL(dim, resolver)
		}
	}
	for name, m := range metricMap {
		if strings.EqualFold(name, token) {
			return c.metricAggregate(m, resolver, dimMap, metricMap, model)
		}
	}
	if strings.Contains(token, ".") {
		return resolver.QualifyColumn(c.dialect, token)
	}
	if model != nil && model.BaseTable != "" {
		return resolver.QualifyColumn(c.dialect, model.BaseTable+"."+token)
	}
	return c.dialect.QuoteIdent(token)
}

func (c *Compiler) metricExpressionRef(
	metric *semantic.Metric,
	expr string,
	resolver *SchemaResolver,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
) string {
	if metric != nil && metric.Expr != nil && strings.TrimSpace(expr) == strings.TrimSpace(metric.Expression) {
		return CompileExpr(metric.Expr, c.dialect, resolver)
	}
	expr = strings.TrimSpace(expr)
	if expr == "*" {
		return expr
	}
	return c.resolveBracketExpressions(expr, resolver, dimMap, metricMap, model)
}

func (c *Compiler) metricAggregate(
	metric *semantic.Metric,
	resolver *SchemaResolver,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
) string {
	expr := c.metricExpressionRef(metric, metric.Expression, resolver, dimMap, metricMap, model)
	if metric.Expr != nil {
		return c.aggregateExpr(metric.Aggregation, expr)
	}
	return c.dialect.Aggregate(metric.Aggregation, expr)
}

func (c *Compiler) aggregateExpr(fn, expr string) string {
	fnLower := strings.ToLower(strings.TrimSpace(fn))
	if fnLower == "custom" || fnLower == "none" || fnLower == "" {
		return expr
	}
	if fnLower == "count" && expr == "*" {
		if c.dialect.Name() == "clickhouse" {
			return "count()"
		}
		return "COUNT(*)"
	}
	switch fnLower {
	case "count":
		if c.dialect.Name() == "clickhouse" {
			return "count(" + expr + ")"
		}
		return "COUNT(" + expr + ")"
	case "count_distinct":
		if c.dialect.Name() == "clickhouse" {
			return "uniq(" + expr + ")"
		}
		return "COUNT(DISTINCT " + expr + ")"
	case "sum":
		if c.dialect.Name() == "clickhouse" {
			return "sum(" + expr + ")"
		}
		return "SUM(" + expr + ")"
	case "avg":
		if c.dialect.Name() == "clickhouse" {
			return "avg(" + expr + ")"
		}
		return "AVG(" + expr + ")"
	case "min":
		if c.dialect.Name() == "clickhouse" {
			return "min(" + expr + ")"
		}
		return "MIN(" + expr + ")"
	case "max":
		if c.dialect.Name() == "clickhouse" {
			return "max(" + expr + ")"
		}
		return "MAX(" + expr + ")"
	default:
		if c.dialect.Name() == "clickhouse" {
			return "count(" + expr + ")"
		}
		return "COUNT(" + expr + ")"
	}
}

func (c *Compiler) qualifyMetricExpression(
	metric *semantic.Metric,
	expr string,
	resolver *SchemaResolver,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
) string {
	if metric != nil && metric.Expr != nil && strings.TrimSpace(expr) == strings.TrimSpace(metric.Expression) {
		return CompileExpr(metric.Expr, c.dialect, resolver)
	}
	expr = strings.TrimSpace(expr)
	if expr == "*" {
		return expr
	}
	if strings.Contains(expr, "[") {
		return c.resolveBracketExpressions(expr, resolver, dimMap, metricMap, model)
	}
	if _, ok := resolver.ParseColumnRef(expr); ok {
		return c.dialect.QuoteIdent(resolver.PhysicalColumnRef(expr))
	}
	return c.dialect.QuoteIdent(expr)
}

func (c *Compiler) buildSelect(items []SelectItem, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver, args *[]any) ([]string, error) {
	parts := make([]string, 0, len(items))
	var sb strings.Builder
	for _, item := range items {
		switch item.Type {
		case SelectTypeDimension:
			dim, ok := dimMap[item.Name]
			if !ok {
				dimKeys := make([]string, 0, len(dimMap))
				for k := range dimMap {
					dimKeys = append(dimKeys, k)
				}
				return nil, validationErrWithCode("select", errmsg.UnknownDimensionMsg(item.Name), errmsg.CodeUnknownDimension, item.Name, suggestAlternatives(item.Name, dimKeys))
			}
			col := c.dimensionOutputSQL(dim, resolver)
			alias := item.Alias
			if alias == "" {
				alias = dim.Name
			}
			quotedAlias := c.dialect.QuoteIdent(alias)
			sb.Reset()
			sb.Grow(len(col) + len(quotedAlias) + 4)
			sb.WriteString(col)
			sb.WriteString(" AS ")
			sb.WriteString(quotedAlias)
			parts = append(parts, sb.String())

		case SelectTypeMetric:
			metric, ok := metricMap[item.Name]
			if !ok {
				var metricKeys []string
				for k := range metricMap {
					metricKeys = append(metricKeys, k)
				}
				return nil, validationErrWithCode("select", errmsg.UnknownMetricMsg(item.Name), errmsg.CodeUnknownMetric, item.Name, suggestAlternatives(item.Name, metricKeys))
			}
			agg := c.metricAggregate(metric, resolver, dimMap, metricMap, model)
			alias := item.Alias
			if alias == "" {
				alias = metric.Name
			}
			quotedAlias := c.dialect.QuoteIdent(alias)
			sb.Reset()
			sb.Grow(len(agg) + len(quotedAlias) + 4)
			sb.WriteString(agg)
			sb.WriteString(" AS ")
			sb.WriteString(quotedAlias)
			parts = append(parts, sb.String())

		case SelectTypeWindow:
			windowSQL, err := c.buildWindowExpr(item, dimMap, metricMap, model, resolver)
			if err != nil {
				return nil, err
			}
			alias := item.Alias
			if alias == "" {
				alias = item.Name
			}
			quotedAlias := c.dialect.QuoteIdent(alias)
			sb.Reset()
			sb.Grow(len(windowSQL) + len(quotedAlias) + 4)
			sb.WriteString(windowSQL)
			sb.WriteString(" AS ")
			sb.WriteString(quotedAlias)
			parts = append(parts, sb.String())

		case SelectTypeCase:
			caseSQL, err := c.buildCaseExpr(item, dimMap, metricMap, model, resolver, args)
			if err != nil {
				return nil, err
			}
			alias := item.Alias
			if alias == "" {
				alias = item.Name
			}
			if alias == "" {
				return nil, errors.New("case select item requires name or alias")
			}
			quotedAlias := c.dialect.QuoteIdent(alias)
			sb.Reset()
			sb.Grow(len(caseSQL) + len(quotedAlias) + 4)
			sb.WriteString(caseSQL)
			sb.WriteString(" AS ")
			sb.WriteString(quotedAlias)
			parts = append(parts, sb.String())
		}
	}
	return parts, nil
}

// buildWindowExpr renders a window/analytic expression:
//
//	<AGG>(<expr>) OVER (PARTITION BY ... ORDER BY ... <frame>)
//
// Aggregation/Expression are sourced from the SelectItem.Window or, when
// Window.Metric is set, inherited from the named metric in the semantic model.
// Ranking functions (row_number, rank, dense_rank, ntile) ignore Expression.
func (c *Compiler) buildWindowExpr(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
) (string, error) {
	if item.Window == nil {
		return "", fmt.Errorf("window select item %q missing window spec", item.Name)
	}
	w := item.Window

	agg := strings.ToLower(strings.TrimSpace(w.Aggregation))
	expr := strings.TrimSpace(w.Expression)

	// Inherit aggregation+expression from a named metric when requested.
	if mname := strings.TrimSpace(w.Metric); mname != "" {
		m, ok := metricMap[mname]
		if !ok {
			return "", fmt.Errorf("window metric not found: %s", mname)
		}
		if agg == "" {
			agg = strings.ToLower(m.Aggregation)
		}
		if expr == "" {
			expr = m.Expression
		}
	}
	if expr != "" && expr != "*" {
		expr = c.metricExpressionRef(nil, expr, resolver, dimMap, metricMap, model)
	}
	if agg == "" {
		return "", fmt.Errorf("window select item %q missing aggregation", item.Name)
	}

	var head string
	switch agg {
	case "row_number", "rank", "dense_rank":
		head = strings.ToUpper(agg) + "()"
	case "ntile":
		bucket := expr
		if bucket == "" {
			bucket = "4"
		}
		if !isPositiveInt(bucket) {
			return "", fmt.Errorf("ntile bucket must be a positive integer, got: %q", bucket)
		}
		head = fmt.Sprintf("NTILE(%s)", bucket)
	default:
		head = c.dialect.Aggregate(agg, expr)
	}

	clauses := make([]string, 0, 4)
	if len(w.PartitionBy) > 0 {
		cols := make([]string, 0, len(w.PartitionBy))
		for _, p := range w.PartitionBy {
			if dim, ok := dimMap[p]; ok {
				cols = append(cols, c.dimensionSQL(dim, resolver))
			} else {
				return "", fmt.Errorf("unknown window partition_by dimension: %s", p)
			}
		}
		clauses = append(clauses, "PARTITION BY "+strings.Join(cols, ", "))
	}
	if len(w.OrderBy) > 0 {
		parts := make([]string, 0, len(w.OrderBy))
		for _, ob := range w.OrderBy {
			dir := strings.ToUpper(strings.TrimSpace(ob.Direction))
			if dir != "DESC" {
				dir = "ASC"
			}
			var ref string
			if dim, ok := dimMap[ob.Field]; ok {
				ref = c.dimensionSQL(dim, resolver)
			} else if metric, ok := metricMap[ob.Field]; ok {
				ref = c.metricAggregate(metric, resolver, dimMap, metricMap, model)
			} else {
				return "", fmt.Errorf("unknown window order_by field: %s", ob.Field)
			}
			parts = append(parts, ref+" "+dir)
		}
		clauses = append(clauses, "ORDER BY "+strings.Join(parts, ", "))
	}
	if frame := strings.TrimSpace(w.Frame); frame != "" {
		if !isValidFrame(frame) {
			return "", fmt.Errorf("invalid window frame clause: %q", frame)
		}
		clauses = append(clauses, frame)
	}
	return head + " OVER (" + strings.Join(clauses, " ") + ")", nil
}

// buildHaving renders post-aggregation filters. Each filter's Field must be a
// metric name; the aggregate expression is substituted so dialects emit e.g.
// SUM("orders"."total_amount") > $1. Placeholder indices start at startArg+1.
func (c *Compiler) buildHaving(
	filters []Filter,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
	startArg int,
) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	argCount := startArg
	emitPlaceholder := func() string {
		argCount++
		return c.dialect.Placeholder(argCount)
	}
	var sb strings.Builder
	for _, f := range filters {
		metric, ok := metricMap[f.Field]
		if !ok {
			return "", nil, fmt.Errorf("unknown having field (must be a metric): %s", f.Field)
		}
		aggSQL := c.metricAggregate(metric, resolver, dimMap, metricMap, model)
		switch f.Operator {
		case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
			args = append(args, f.Value)
			op := sqlComparator(f.Operator)
			p := emitPlaceholder()
			sb.Reset()
			sb.Grow(len(aggSQL) + len(op) + len(p) + 2)
			sb.WriteString(aggSQL)
			sb.WriteByte(' ')
			sb.WriteString(op)
			sb.WriteByte(' ')
			sb.WriteString(p)
			parts = append(parts, sb.String())
		case OpBetween:
			vals, ok := f.Value.([]any)
			if !ok || len(vals) != 2 {
				return "", nil, fmt.Errorf("having between expects 2 values for metric %q", f.Field)
			}
			args = append(args, vals[0], vals[1])
			p1 := emitPlaceholder()
			p2 := emitPlaceholder()
			sb.Reset()
			sb.Grow(len(aggSQL) + len(p1) + len(p2) + 14)
			sb.WriteString(aggSQL)
			sb.WriteString(" BETWEEN ")
			sb.WriteString(p1)
			sb.WriteString(" AND ")
			sb.WriteString(p2)
			parts = append(parts, sb.String())
		case OpIsNull:
			sb.Reset()
			sb.Grow(len(aggSQL) + 8)
			sb.WriteString(aggSQL)
			sb.WriteString(" IS NULL")
			parts = append(parts, sb.String())
		case OpIsNotNull:
			sb.Reset()
			sb.Grow(len(aggSQL) + 12)
			sb.WriteString(aggSQL)
			sb.WriteString(" IS NOT NULL")
			parts = append(parts, sb.String())
		default:
			return "", nil, fmt.Errorf("operator %q not supported in HAVING for metric %q", f.Operator, f.Field)
		}
	}
	return strings.Join(parts, " AND "), args, nil
}

// sqlComparator translates a logical operator to a SQL comparator. Only
// basic scalar operators are mapped; HAVING only uses these.
func sqlComparator(op string) string {
	switch op {
	case OpEq:
		return "="
	case OpNeq:
		return "!="
	case OpGt:
		return ">"
	case OpGte:
		return ">="
	case OpLt:
		return "<"
	case OpLte:
		return "<="
	default:
		return "="
	}
}

func (c *Compiler) buildFrom(model *semantic.SemanticModel) string {
	schema := c.dialect.QuoteIdent(model.BaseSchema)
	table := c.dialect.QuoteIdent(model.BaseTable)
	return schema + "." + table
}

func (c *Compiler) buildJoins(joinNames []string, joinMap map[string]semantic.Join, model *semantic.SemanticModel, resolver *SchemaResolver) []string {
	baseKey := TableKey(model.BaseSchema, model.BaseTable)
	inSet := map[string]struct{}{baseKey: {}}

	clauses := make([]string, 0, len(joinNames))
	for _, name := range joinNames {
		j, ok := joinMap[name]
		if !ok {
			continue
		}

		joinType := strings.ToUpper(j.JoinType)
		if joinType == "" {
			joinType = semantic.DefaultJoinType
		}

		fromSchema, fromTable, fromCol := j.FromSchema, j.FromTable, j.FromColumn
		toSchema, toTable, toCol := j.ToSchema, j.ToTable, j.ToColumn
		fromKey := resolver.JoinSideKey(fromSchema, fromTable)
		toKey := resolver.JoinSideKey(toSchema, toTable)

		_, toInSet := inSet[toKey]
		_, fromInSet := inSet[fromKey]

		if toInSet && !fromInSet {
			fromSchema, toSchema = toSchema, fromSchema
			fromTable, toTable = toTable, fromTable
			fromCol, toCol = toCol, fromCol
			toKey = fromKey
		} else if toInSet && fromInSet {
			continue
		}
		inSet[toKey] = struct{}{}

		fromTableSQL := resolver.QualifyTable(c.dialect, fromSchema, fromTable)
		toTableSQL := resolver.QualifyTable(c.dialect, toSchema, toTable)

		clause := joinType + " JOIN " + toTableSQL +
			" ON " + fromTableSQL + "." + c.dialect.QuoteIdentSegment(fromCol) +
			" = " + toTableSQL + "." + c.dialect.QuoteIdentSegment(toCol)
		clauses = append(clauses, clause)
	}
	return clauses
}

func (c *Compiler) buildWhere(filters []Filter, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver, args *[]any) (string, error) {
	if len(filters) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(filters))

	for _, f := range filters {
		if dim, ok := dimMap[f.Field]; ok && monthGrainFilterUsesDateTrunc(dim, f) {
			anchor, ok := calendarAnchorTime(f.Value)
			if !ok {
				return "", fmt.Errorf("month grain filter on %q: expected calendar anchor value", f.Field)
			}
			expr, err := c.dateTruncCompareExpr(TimeGrainMonth, resolver.PhysicalColumnRef(dim.ColumnRef), f.Operator, len(*args)+1)
			if err != nil {
				return "", err
			}
			*args = append(*args, anchor.UTC())
			parts = append(parts, expr)
			continue
		}
		if dim, ok := dimMap[f.Field]; ok && quarterGrainFilterUsesDateTrunc(dim, f) {
			anchor, ok := calendarAnchorTime(f.Value)
			if !ok {
				return "", fmt.Errorf("quarter grain filter on %q: expected calendar anchor value", f.Field)
			}
			expr, err := c.dateTruncCompareExpr(TimeGrainQuarter, resolver.PhysicalColumnRef(dim.ColumnRef), f.Operator, len(*args)+1)
			if err != nil {
				return "", err
			}
			*args = append(*args, anchor.UTC())
			parts = append(parts, expr)
			continue
		}

		colSQL, err := c.resolveFilterLHS(f.Field, dimMap, metricMap, model, resolver)
		if err != nil {
			return "", err
		}

		part, newArgs, err := c.buildFilterPart(f, colSQL, model, args)
		if err != nil {
			return "", err
		}
		*args = append(*args, newArgs...)
		parts = append(parts, part)
	}

	return strings.Join(parts, " AND "), nil
}

// resolveFilterLHS returns SQL for the left-hand side of a filter (quoted column, metric expression, or date_trunc).
func (c *Compiler) resolveFilterLHS(field string, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver) (string, error) {
	if dim, ok := dimMap[field]; ok {
		if c.filterFieldHidden(dim, resolver) {
			return "", validationErrWithCode("filters", errmsg.HiddenPIIFieldMsg(field), errmsg.CodeHiddenPIIField, field, nil)
		}
		return c.dimensionSQL(dim, resolver), nil
	}
	if metric, ok := metricMap[field]; ok {
		return c.qualifyMetricExpression(metric, metric.Expression, resolver, dimMap, metricMap, model), nil
	}
	var fieldKeys []string
	for k := range dimMap {
		fieldKeys = append(fieldKeys, k)
	}
	for k := range metricMap {
		fieldKeys = append(fieldKeys, k)
	}
	return "", validationErrWithCode("filters", errmsg.UnknownFieldMsg(field), errmsg.CodeUnknownField, field, suggestAlternatives(field, fieldKeys))
}

func sliceOfStrings(val any) ([]string, bool) {
	if val == nil {
		return nil, false
	}
	switch v := val.(type) {
	case []string:
		return v, true
	case []any:
		res := make([]string, 0, len(v))
		for _, item := range v {
			if item != nil {
				res = append(res, fmt.Sprint(item))
			}
		}
		return res, true
	default:
		return nil, false
	}
}

func (c *Compiler) likeExpression(lhsSQL, placeholder string, caseSensitive bool) string {
	if caseSensitive {
		switch c.dialect.Name() {
		case "mysql":
			return fmt.Sprintf("%s LIKE BINARY %s", lhsSQL, placeholder)
		case "sqlserver":
			return fmt.Sprintf("%s LIKE %s COLLATE Latin1_General_CS_AS", lhsSQL, placeholder)
		default:
			return fmt.Sprintf("%s LIKE %s", lhsSQL, placeholder)
		}
	}
	return c.dialect.ILike(lhsSQL, placeholder)
}

func (c *Compiler) buildGroupBy(groupBy []GroupBy, dimMap map[string]*semantic.Dimension, resolver *SchemaResolver) (string, error) {
	if len(groupBy) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(groupBy))
	for _, gb := range groupBy {
		dim, ok := dimMap[gb.Field]
		if !ok {
			var dimKeys []string
			for k := range dimMap {
				dimKeys = append(dimKeys, k)
			}
			return "", validationErrWithCode("group_by", errmsg.UnknownDimensionMsg(gb.Field), errmsg.CodeUnknownDimension, gb.Field, suggestAlternatives(gb.Field, dimKeys))
		}
		if c.dimensionFullyHidden(dim, resolver) {
			return "", validationErrWithCode("group_by", errmsg.HiddenPIIFieldMsg(gb.Field), errmsg.CodeHiddenPIIField, gb.Field, nil)
		}
		parts = append(parts, c.dimensionOutputSQL(dim, resolver))
	}
	return strings.Join(parts, ", "), nil
}

func (c *Compiler) buildOrderBy(orderBy []OrderBy, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric, resolver *SchemaResolver) (string, error) {
	if len(orderBy) == 0 {
		return "", nil
	}

	parts := make([]string, 0, len(orderBy))
	var sb strings.Builder
	for _, ob := range orderBy {
		if dim, ok := dimMap[ob.Field]; ok {
			if c.dimensionFullyHidden(dim, resolver) {
				return "", validationErrWithCode("order_by", errmsg.HiddenPIIFieldMsg(ob.Field), errmsg.CodeHiddenPIIField, ob.Field, nil)
			}
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			dimSQL := c.dimensionOutputSQL(dim, resolver)
			sb.Reset()
			sb.Grow(len(dimSQL) + len(dir) + 1)
			sb.WriteString(dimSQL)
			sb.WriteByte(' ')
			sb.WriteString(dir)
			parts = append(parts, sb.String())
		} else if metric, ok := metricMap[ob.Field]; ok {
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			quotedName := c.dialect.QuoteIdent(metric.Name)
			sb.Reset()
			sb.Grow(len(quotedName) + len(dir) + 1)
			sb.WriteString(quotedName)
			sb.WriteByte(' ')
			sb.WriteString(dir)
			parts = append(parts, sb.String())
		} else {
			var fieldKeys []string
			for k := range dimMap {
				fieldKeys = append(fieldKeys, k)
			}
			for k := range metricMap {
				fieldKeys = append(fieldKeys, k)
			}
			return "", validationErrWithCode("order_by", errmsg.UnknownFieldMsg(ob.Field), errmsg.CodeUnknownField, ob.Field, suggestAlternatives(ob.Field, fieldKeys))
		}
	}

	return strings.Join(parts, ", "), nil
}

var validFramePattern = regexp.MustCompile(`(?i)^\s*(ROWS|RANGE|GROUPS)\s+BETWEEN\s+(UNBOUNDED\s+PRECEDING|\d+\s+PRECEDING|CURRENT\s+ROW)\s+AND\s+(UNBOUNDED\s+FOLLOWING|\d+\s+PRECEDING|\d+\s+FOLLOWING|CURRENT\s+ROW)\s*$`)

func isValidFrame(frame string) bool {
	return validFramePattern.MatchString(frame)
}

func isPositiveInt(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != "0"
}
