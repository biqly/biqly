package query

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// Compiler compiles a LogicalQuery into dialect-specific SQL.
type Compiler struct {
	dialect dialect.Dialect
}

// NewCompiler creates a new SQL compiler for the given dialect.
func NewCompiler(d dialect.Dialect) *Compiler {
	return &Compiler{dialect: d}
}

// Compile converts a LogicalQuery + semantic model into SQL.
func (c *Compiler) Compile(ctx context.Context, lq LogicalQuery, model *semantic.SemanticModel) (*CompiledQuery, error) {
	args := make([]any, 0, 8)
	withPrefix, err := c.buildWithClause(lq.CTEs, model, &args)
	if err != nil {
		return nil, err
	}
	fromClause, err := c.resolveFromClause(lq, model, &args)
	if err != nil {
		return nil, err
	}
	return c.compileStatement(ctx, lq, model, fromClause, withPrefix, &args)
}

// CompileWithPermissions compiles a LogicalQuery with row-level security filters injected.
func (c *Compiler) CompileWithPermissions(
	ctx context.Context,
	lq LogicalQuery,
	model *semantic.SemanticModel,
	rowFilters []security.RowFilter,
) (*CompiledQuery, error) {
	// Build dimension map for field resolution
	dimMap := make(map[string]string)
	for _, d := range model.Dimensions {
		dimMap[d.Name] = d.ColumnRef
	}
	for _, m := range model.Metrics {
		dimMap[m.Name] = m.Expression
	}

	// If no row filters, compile normally
	if len(rowFilters) == 0 {
		return c.Compile(ctx, lq, model)
	}

	// Compile normally first
	cq, err := c.Compile(ctx, lq, model)
	if err != nil {
		return nil, err
	}

	// Check if original SQL has WHERE clause
	upperSQL := strings.ToUpper(cq.SQL)
	hasWhere := strings.Contains(upperSQL, " WHERE ")

	filterParts, filterArgs, err := security.BuildRowFilterPredicates(
		c.dialect, dimMap, rowFilters, len(cq.Args), true,
	)
	if err != nil {
		return nil, err
	}

	if len(filterParts) == 0 {
		return cq, nil
	}

	rowFilterSQL := strings.Join(filterParts, " AND ")

	if hasWhere {
		// Find where the WHERE clause content ends (before GROUP BY/ORDER BY/LIMIT)
		whereIdx := strings.Index(upperSQL, " WHERE ")
		whereEnd := whereIdx + len(" WHERE ")
		// Find the end of WHERE content (start of next clause)
		contentEnd := len(cq.SQL)
		for _, kw := range []string{" GROUP BY ", " ORDER BY ", " LIMIT ", " OFFSET "} {
			if idx := strings.Index(upperSQL, kw); idx != -1 && idx > whereEnd && idx < contentEnd {
				contentEnd = idx
			}
		}
		// Insert row filter between existing WHERE content and next clause
		existingWhereContent := cq.SQL[whereEnd:contentEnd]
		afterContent := cq.SQL[contentEnd:]
		cq.SQL = cq.SQL[:whereEnd] + existingWhereContent + " AND " + rowFilterSQL + afterContent
		cq.Args = append(cq.Args, filterArgs...)
	} else {
		// Insert WHERE before GROUP BY / ORDER BY / LIMIT / OFFSET
		injectPoint := len(cq.SQL)
		for _, kw := range []string{"GROUP BY", "ORDER BY", "LIMIT", "OFFSET"} {
			if idx := strings.Index(upperSQL, kw); idx != -1 && idx < injectPoint {
				injectPoint = idx
			}
		}

		whereClause := " WHERE " + rowFilterSQL + " "
		cq.SQL = cq.SQL[:injectPoint] + whereClause + cq.SQL[injectPoint:]
		cq.Args = append(cq.Args, filterArgs...)
	}

	return cq, nil
}

func addTableFromColumnRef(tables map[string]bool, colRef string, resolver *SchemaResolver) {
	if p, ok := resolver.ParseColumnRef(colRef); ok {
		tables[TableKey(p.Schema, p.Table)] = true
	}
}

func tablesReferencedInLogicalQuery(
	lq LogicalQuery,
	model *semantic.SemanticModel,
	dimMap map[string]semantic.Dimension,
	metricMap map[string]semantic.Metric,
	resolver *SchemaResolver,
) map[string]bool {
	tables := make(map[string]bool)
	tables[TableKey(model.BaseSchema, model.BaseTable)] = true

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
	lq LogicalQuery,
	model *semantic.SemanticModel,
	dimMap map[string]semantic.Dimension,
	metricMap map[string]semantic.Metric,
	resolver *SchemaResolver,
) []string {
	neededTables := tablesReferencedInLogicalQuery(lq, model, dimMap, metricMap, resolver)
	if len(model.Joins) == 0 {
		return nil
	}

	neighbors := make(map[string][]joinNeighbor)
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
	parent := make(map[string]parentInfo)
	var joinDiscovery []string
	joinFirst := make(map[string]bool)

	queue := []string{base}
	parent[base] = parentInfo{"", ""}
	visited := map[string]bool{base: true}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, nb := range neighbors[u] {
			if visited[nb.table] {
				continue
			}
			visited[nb.table] = true
			parent[nb.table] = parentInfo{u, nb.joinName}
			if !joinFirst[nb.joinName] {
				joinFirst[nb.joinName] = true
				joinDiscovery = append(joinDiscovery, nb.joinName)
			}
			queue = append(queue, nb.table)
		}
	}

	required := make(map[string]bool)
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
			required[pi.join] = true
			cur = pi.prev
		}
	}

	var out []string
	for _, jn := range joinDiscovery {
		if required[jn] {
			out = append(out, jn)
		}
	}
	return out
}

func (c *Compiler) dimensionSQL(dim semantic.Dimension, resolver *SchemaResolver) string {
	// If a calculated expression is defined, use it directly.
	if strings.TrimSpace(dim.CalculatedExpression) != "" {
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

func (c *Compiler) metricExpressionRef(expr string, resolver *SchemaResolver) string {
	if expr == "*" {
		return expr
	}
	if _, ok := resolver.ParseColumnRef(expr); ok {
		return resolver.PhysicalColumnRef(expr)
	}
	return expr
}

func (c *Compiler) qualifyMetricExpression(expr string, resolver *SchemaResolver) string {
	if expr == "*" {
		return expr
	}
	if _, ok := resolver.ParseColumnRef(expr); ok {
		return c.dialect.QuoteIdent(resolver.PhysicalColumnRef(expr))
	}
	return c.dialect.QuoteIdent(expr)
}

func (c *Compiler) buildSelect(items []SelectItem, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver, args *[]any) ([]string, error) {
	var parts []string
	for _, item := range items {
		switch item.Type {
		case SelectTypeDimension:
			dim, ok := dimMap[item.Name]
			if !ok {
				return nil, validationErr("select", errmsg.UnknownDimensionMsg(item.Name))
			}
			col := c.dimensionSQL(dim, resolver)
			alias := item.Alias
			if alias == "" {
				alias = dim.Name
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", col, c.dialect.QuoteIdent(alias)))

		case SelectTypeMetric:
			metric, ok := metricMap[item.Name]
			if !ok {
				return nil, validationErr("select", errmsg.UnknownMetricMsg(item.Name))
			}
			agg := c.dialect.Aggregate(metric.Aggregation, c.metricExpressionRef(metric.Expression, resolver))
			alias := item.Alias
			if alias == "" {
				alias = metric.Name
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", agg, c.dialect.QuoteIdent(alias)))

		case SelectTypeWindow:
			windowSQL, err := c.buildWindowExpr(item, dimMap, metricMap, resolver)
			if err != nil {
				return nil, err
			}
			alias := item.Alias
			if alias == "" {
				alias = item.Name
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", windowSQL, c.dialect.QuoteIdent(alias)))

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
				return nil, fmt.Errorf("case select item requires name or alias")
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", caseSQL, c.dialect.QuoteIdent(alias)))
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
	dimMap map[string]semantic.Dimension,
	metricMap map[string]semantic.Metric,
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
		expr = c.metricExpressionRef(expr, resolver)
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
		if expr == "" {
			return "", fmt.Errorf("window aggregation %q requires expression", agg)
		}
		head = c.dialect.Aggregate(agg, expr)
	}

	var clauses []string
	if len(w.PartitionBy) > 0 {
		var cols []string
		for _, name := range w.PartitionBy {
			dim, ok := dimMap[name]
			if !ok {
				return "", fmt.Errorf("unknown partition_by dimension: %s", name)
			}
			cols = append(cols, c.dimensionSQL(dim, resolver))
		}
		clauses = append(clauses, "PARTITION BY "+strings.Join(cols, ", "))
	}
	if len(w.OrderBy) > 0 {
		var parts []string
		for _, ob := range w.OrderBy {
			dir := strings.ToUpper(strings.TrimSpace(ob.Direction))
			if dir != "DESC" {
				dir = "ASC"
			}
			ref := ""
			if dim, ok := dimMap[ob.Field]; ok {
				ref = c.dimensionSQL(dim, resolver)
			} else if metric, ok := metricMap[ob.Field]; ok {
				ref = c.dialect.Aggregate(metric.Aggregation, c.metricExpressionRef(metric.Expression, resolver))
			} else {
				return "", fmt.Errorf("unknown window order_by field: %s", ob.Field)
			}
			parts = append(parts, fmt.Sprintf("%s %s", ref, dir))
		}
		clauses = append(clauses, "ORDER BY "+strings.Join(parts, ", "))
	}
	if frame := strings.TrimSpace(w.Frame); frame != "" {
		if !isValidFrame(frame) {
			return "", fmt.Errorf("invalid window frame clause: %q", frame)
		}
		clauses = append(clauses, frame)
	}

	if len(clauses) == 0 {
		return head + " OVER ()", nil
	}
	return head + " OVER (" + strings.Join(clauses, " ") + ")", nil
}

// buildHaving renders post-aggregation filters. Each filter's Field must be a
// metric name; the aggregate expression is substituted so dialects emit e.g.
// SUM("orders"."total_amount") > $1. Placeholder indices start at startArg+1.
func (c *Compiler) buildHaving(
	filters []Filter,
	metricMap map[string]semantic.Metric,
	resolver *SchemaResolver,
	startArg int,
) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var parts []string
	args := make([]any, 0, len(filters))
	argCount := startArg
	emitPlaceholder := func() string {
		argCount++
		return c.dialect.Placeholder(argCount)
	}
	for _, f := range filters {
		metric, ok := metricMap[f.Field]
		if !ok {
			return "", nil, fmt.Errorf("unknown having field (must be a metric): %s", f.Field)
		}
		aggSQL := c.dialect.Aggregate(metric.Aggregation, c.metricExpressionRef(metric.Expression, resolver))
		switch f.Operator {
		case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
			args = append(args, f.Value)
			parts = append(parts, fmt.Sprintf("%s %s %s", aggSQL, sqlComparator(f.Operator), emitPlaceholder()))
		case OpBetween:
			vals, ok := f.Value.([]any)
			if !ok || len(vals) != 2 {
				return "", nil, fmt.Errorf("having between expects 2 values for metric %q", f.Field)
			}
			args = append(args, vals[0], vals[1])
			p1 := emitPlaceholder()
			p2 := emitPlaceholder()
			parts = append(parts, fmt.Sprintf("%s BETWEEN %s AND %s", aggSQL, p1, p2))
		case OpIsNull:
			parts = append(parts, fmt.Sprintf("%s IS NULL", aggSQL))
		case OpIsNotNull:
			parts = append(parts, fmt.Sprintf("%s IS NOT NULL", aggSQL))
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
	return fmt.Sprintf("%s.%s", schema, table)
}

func (c *Compiler) buildJoins(joinNames []string, joinMap map[string]semantic.Join, model *semantic.SemanticModel, resolver *SchemaResolver) []string {
	baseKey := TableKey(model.BaseSchema, model.BaseTable)
	inSet := map[string]bool{baseKey: true}

	var clauses []string
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
		if inSet[toKey] && !inSet[fromKey] {
			fromSchema, toSchema = toSchema, fromSchema
			fromTable, toTable = toTable, fromTable
			fromCol, toCol = toCol, fromCol
			fromKey, toKey = toKey, fromKey
		} else if inSet[toKey] && inSet[fromKey] {
			continue
		}
		inSet[toKey] = true

		fromTableSQL := resolver.QualifyTable(c.dialect, fromSchema, fromTable)
		toTableSQL := resolver.QualifyTable(c.dialect, toSchema, toTable)

		clause := fmt.Sprintf("%s JOIN %s ON %s.%s = %s.%s",
			joinType, toTableSQL,
			fromTableSQL, c.dialect.QuoteIdentSegment(fromCol),
			toTableSQL, c.dialect.QuoteIdentSegment(toCol))
		clauses = append(clauses, clause)
	}
	return clauses
}

func (c *Compiler) buildWhere(filters []Filter, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver, args *[]any) (string, error) {
	if len(filters) == 0 {
		return "", nil
	}

	var parts []string

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

		colSQL, err := c.resolveFilterLHS(f.Field, dimMap, metricMap, resolver)
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
func (c *Compiler) resolveFilterLHS(field string, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric, resolver *SchemaResolver) (string, error) {
	if dim, ok := dimMap[field]; ok {
		return c.dimensionSQL(dim, resolver), nil
	}
	if metric, ok := metricMap[field]; ok {
		return c.qualifyMetricExpression(metric.Expression, resolver), nil
	}
	return "", validationErr("filters", errmsg.UnknownFieldMsg(field))
}

func (c *Compiler) buildFilterPart(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	switch f.Operator {
	case OpEq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s = %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpNeq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s != %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpGt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s > %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpGte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s >= %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpLt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s < %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpLte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s <= %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpIn:
		if f.Subquery != nil {
			return c.buildInSubqueryFilter(lhsSQL, f, model, true, args)
		}
		return c.buildInFilter(lhsSQL, f.Value, args)
	case OpNotIn:
		if f.Subquery != nil {
			return c.buildInSubqueryFilter(lhsSQL, f, model, false, args)
		}
		return c.buildNotInFilter(lhsSQL, f.Value, args)
	case OpContains:
		*args = append(*args, fmt.Sprintf("%%%v%%", f.Value))
		return c.dialect.ILike(lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpStartsWith:
		*args = append(*args, fmt.Sprintf("%v%%", f.Value))
		return c.dialect.ILike(lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpEndsWith:
		*args = append(*args, fmt.Sprintf("%%%v", f.Value))
		return c.dialect.ILike(lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
	case OpBetween:
		return c.buildBetweenFilter(lhsSQL, f.Value, args)
	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", lhsSQL), nil, nil
	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", lhsSQL), nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", f.Operator)
	}
}

func (c *Compiler) buildInFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, fmt.Errorf("in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return fmt.Sprintf("%s IN (%s)", lhsSQL, strings.Join(placeholders, ", ")), nil, nil
}

func (c *Compiler) buildNotInFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, fmt.Errorf("not_in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return fmt.Sprintf("%s NOT IN (%s)", lhsSQL, strings.Join(placeholders, ", ")), nil, nil
}

func (c *Compiler) buildBetweenFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok || len(vals) != 2 {
		return "", nil, fmt.Errorf("between operator expects 2 values")
	}
	*args = append(*args, vals[0], vals[1])
	p1 := c.dialect.Placeholder(len(*args) - 1)
	p2 := c.dialect.Placeholder(len(*args))
	return fmt.Sprintf("%s BETWEEN %s AND %s", lhsSQL, p1, p2), nil, nil
}

func (c *Compiler) buildGroupBy(groupBy []GroupBy, dimMap map[string]semantic.Dimension, resolver *SchemaResolver) (string, error) {
	if len(groupBy) == 0 {
		return "", nil
	}

	var parts []string
	for _, gb := range groupBy {
		dim, ok := dimMap[gb.Field]
		if !ok {
			return "", validationErr("group_by", errmsg.UnknownDimensionMsg(gb.Field))
		}
		parts = append(parts, c.dimensionSQL(dim, resolver))
	}
	return strings.Join(parts, ", "), nil
}

func (c *Compiler) buildOrderBy(orderBy []OrderBy, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric, resolver *SchemaResolver) (string, error) {
	if len(orderBy) == 0 {
		return "", nil
	}

	var parts []string
	for _, ob := range orderBy {
		if dim, ok := dimMap[ob.Field]; ok {
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			parts = append(parts, fmt.Sprintf("%s %s", c.dimensionSQL(dim, resolver), dir))
		} else if metric, ok := metricMap[ob.Field]; ok {
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			parts = append(parts, fmt.Sprintf("%s %s", c.dialect.QuoteIdent(metric.Name), dir))
		} else {
			return "", validationErr("order_by", errmsg.UnknownFieldMsg(ob.Field))
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
