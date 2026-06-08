package query

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/platform/observability"
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
	dialect    dialect.Dialect
	pii        *PIIMaskingConfig
	compileCtx context.Context
	rowFilters []security.RowFilter
	err        error
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
	if ctx == nil {
		return nil, errors.New("query compiler requires non-nil context")
	}
	ctx, span := otel.Tracer("biqly/query").Start(ctx, "query.Compile")
	compileStart := time.Now()
	defer func() {
		span.SetAttributes(attribute.Int64("query.compile.duration_ms", time.Since(compileStart).Milliseconds()))
		span.End()
	}()
	observability.SetDBSystemAttributes(span, observability.DBSystem(ctx))
	if model != nil {
		span.SetAttributes(
			attribute.String("model.id", model.ID),
			attribute.String("dialect", c.dialect.Name()),
		)
	}

	comp := c.withCompileCtx(ctx)
	if comp.err != nil {
		span.RecordError(comp.err)
		span.SetStatus(codes.Error, comp.err.Error())
		return nil, comp.err
	}
	args := make([]any, 0, 8)
	withPrefix, err := comp.buildWithClause(lq.CTEs, model, &args)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	fromClause, err := comp.resolveFromClause(lq, model, &args)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	cq, err := comp.compileStatement(ctx, lq, model, fromClause, withPrefix, &args, comp.rowFilters)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if comp.err != nil {
		span.RecordError(comp.err)
		span.SetStatus(codes.Error, comp.err.Error())
		return nil, comp.err
	}
	if lq != nil {
		if fp, fpErr := LogicalQueryFingerprint(lq, model); fpErr == nil && fp != "" {
			span.SetAttributes(attribute.String("query.fingerprint", fp))
		}
	}
	return cq, nil
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
	if piiConfig != nil || len(rowFilters) > 0 {
		// Clone so the masking policy and rowFilters are scoped to this compilation; nested
		// subquery/CTE compilation reuses the same compiler and must apply
		// the same policy.
		c = &Compiler{
			dialect:    c.dialect,
			pii:        piiConfig,
			compileCtx: c.compileCtx,
			rowFilters: rowFilters,
			err:        c.err,
		}
	}
	if len(rowFilters) == 0 && piiConfig == nil {
		return c.Compile(ctx, lq, model)
	}
	comp := c.withCompileCtx(ctx)
	if comp.err != nil {
		return nil, comp.err
	}
	args := make([]any, 0, 8)
	withPrefix, err := comp.buildWithClause(lq.CTEs, model, &args)
	if err != nil {
		return nil, err
	}
	fromClause, err := comp.resolveFromClause(lq, model, &args)
	if err != nil {
		return nil, err
	}
	cq, err := comp.compileStatement(ctx, lq, model, fromClause, withPrefix, &args, rowFilters)
	if err != nil {
		return nil, err
	}
	if comp.err != nil {
		return nil, comp.err
	}
	return cq, nil
}

func (c *Compiler) withCompileCtx(ctx context.Context) *Compiler {
	if ctx == nil {
		return &Compiler{
			dialect:    c.dialect,
			pii:        c.pii,
			compileCtx: context.Background(),
			rowFilters: c.rowFilters,
			err:        errors.New("query compiler requires non-nil context"),
		}
	}
	return &Compiler{dialect: c.dialect, pii: c.pii, compileCtx: ctx, rowFilters: c.rowFilters, err: c.err}
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

func addTableForField(
	tables map[string]struct{},
	field string,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	if dim, ok := dimMap[field]; ok {
		addTableFromColumnRef(tables, dim.ColumnRef, resolver)
	}
	if metric, ok := metricMap[field]; ok {
		addTableFromColumnRef(tables, metric.Expression, resolver)
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

	addTablesFromSelectItems(tables, lq.Select, dimMap, metricMap, resolver)
	addTablesFromHaving(tables, lq.Having, metricMap, resolver)
	addTablesFromFilters(tables, lq.Filters, dimMap, metricMap, resolver)
	addTablesFromGroupBy(tables, lq.GroupBy, dimMap, resolver)
	addTablesFromOrderBy(tables, lq.OrderBy, dimMap, metricMap, resolver)

	return tables
}

func addTablesFromSelectItems(
	tables map[string]struct{},
	items []SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	for _, item := range items {
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
			addTablesFromWindowSelect(tables, item.Window, dimMap, metricMap, resolver)
		}
	}
}

func addTablesFromWindowSelect(
	tables map[string]struct{},
	w *WindowSpec,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	if w == nil {
		return
	}
	if mname := w.Metric; mname != "" {
		if m, ok := metricMap[mname]; ok {
			addTableFromColumnRef(tables, m.Expression, resolver)
		}
	}
	if expr := w.Expression; expr != "" {
		addTableFromColumnRef(tables, expr, resolver)
	}
	for _, p := range w.PartitionBy {
		if dim, ok := dimMap[p]; ok {
			addTableFromColumnRef(tables, dim.ColumnRef, resolver)
		}
	}
	for _, ob := range w.OrderBy {
		addTableForField(tables, ob.Field, dimMap, metricMap, resolver)
	}
}

func addTablesFromHaving(
	tables map[string]struct{},
	having []Filter,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	for _, f := range having {
		if m, ok := metricMap[f.Field]; ok {
			addTableFromColumnRef(tables, m.Expression, resolver)
		}
	}
}

func addTablesFromFilters(
	tables map[string]struct{},
	filters []Filter,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	for _, f := range filters {
		addTableForField(tables, f.Field, dimMap, metricMap, resolver)
	}
}

func addTablesFromGroupBy(
	tables map[string]struct{},
	groupBy []GroupBy,
	dimMap map[string]*semantic.Dimension,
	resolver *SchemaResolver,
) {
	for _, gb := range groupBy {
		if dim, ok := dimMap[gb.Field]; ok {
			addTableFromColumnRef(tables, dim.ColumnRef, resolver)
		}
	}
}

func addTablesFromOrderBy(
	tables map[string]struct{},
	orderBy []OrderBy,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) {
	for _, ob := range orderBy {
		addTableForField(tables, ob.Field, dimMap, metricMap, resolver)
	}
}

type joinNeighbor struct {
	table    string
	joinName string
}

// determineJoins returns joins on paths from the base table to every table referenced
// in the logical query. This avoids emitting duplicate joins to the same physical table
// when multiple FKs exist but the query only uses base-table columns.
func (*Compiler) determineJoins(
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

func (c *Compiler) dimensionSQL(dim *semantic.Dimension, resolver *SchemaResolver) (string, error) {
	if dim.CalculatedExpr != nil {
		return CompileExpr(dim.CalculatedExpr, c.dialect, resolver, nil, c.pii)
	}
	if expr := strings.TrimSpace(dim.CalculatedExpression); expr != "" {
		parsed, err := ParseExpression(expr)
		if err != nil {
			return "", err
		}
		return CompileExpr(parsed, c.dialect, resolver, nil, c.pii)
	}
	colRef := resolver.PhysicalColumnRef(dim.ColumnRef)
	if strings.TrimSpace(dim.TimeGrain) == "" {
		return c.dialect.QuoteIdent(colRef), nil
	}
	part := strings.ToLower(strings.TrimSpace(dim.TimeGrain))
	switch part {
	case "year", "quarter", "month":
		return c.dialect.CalendarPart(part, colRef), nil
	default:
		return c.dialect.DateTrunc(part, colRef), nil
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
		sql, err := CompileExpr(metric.Expr, c.dialect, resolver, nil, c.pii)
		if err != nil {
			c.err = err
			return ""
		}
		return sql
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
	if !isSupportedAggregation(metric.Aggregation) {
		c.err = fmt.Errorf("unsupported aggregation function: %s", metric.Aggregation)
		return ""
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
		c.err = fmt.Errorf("unsupported aggregation function: %s", fn)
		return ""
	}
}

func isSupportedAggregation(fn string) bool {
	switch strings.ToLower(strings.TrimSpace(fn)) {
	case "", "custom", "none", "count", "count_distinct", "sum", "avg", "min", "max":
		return true
	default:
		return false
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
		sql, err := CompileExpr(metric.Expr, c.dialect, resolver, nil, c.pii)
		if err != nil {
			c.err = err
			return ""
		}
		return sql
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
	for _, item := range items {
		part, err := c.buildSelectItem(item, dimMap, metricMap, model, resolver, args)
		if err != nil {
			return nil, err
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts, nil
}

func (c *Compiler) buildSelectItem(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
	args *[]any,
) (string, error) {
	switch item.Type {
	case SelectTypeDimension:
		return c.buildSelectDimension(item, dimMap, resolver)
	case SelectTypeMetric:
		return c.buildSelectMetric(item, dimMap, metricMap, model, resolver)
	case SelectTypeWindow:
		return c.buildSelectWindow(item, dimMap, metricMap, model, resolver)
	case SelectTypeCase:
		return c.buildSelectCase(item, dimMap, metricMap, model, resolver, args)
	default:
		return "", nil
	}
}

func (c *Compiler) buildSelectDimension(item SelectItem, dimMap map[string]*semantic.Dimension, resolver *SchemaResolver) (string, error) {
	dim, ok := dimMap[item.Name]
	if !ok {
		dimKeys := make([]string, 0, len(dimMap))
		for k := range dimMap {
			dimKeys = append(dimKeys, k)
		}
		return "", validationErrWithCode("select", errmsg.UnknownDimensionMsg(item.Name), errmsg.CodeUnknownDimension, item.Name, suggestAlternatives(item.Name, dimKeys))
	}
	return selectItemSQL(c.dimensionOutputSQL(dim, resolver), selectItemAlias(item.Alias, dim.Name), c.dialect), nil
}

func (c *Compiler) buildSelectMetric(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
) (string, error) {
	metric, ok := metricMap[item.Name]
	if !ok {
		metricKeys := make([]string, 0, len(metricMap))
		for k := range metricMap {
			metricKeys = append(metricKeys, k)
		}
		return "", validationErrWithCode("select", errmsg.UnknownMetricMsg(item.Name), errmsg.CodeUnknownMetric, item.Name, suggestAlternatives(item.Name, metricKeys))
	}
	agg := c.metricAggregate(metric, resolver, dimMap, metricMap, model)
	if c.err != nil {
		return "", c.err
	}
	return selectItemSQL(agg, selectItemAlias(item.Alias, metric.Name), c.dialect), nil
}

func (c *Compiler) buildSelectWindow(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
) (string, error) {
	windowSQL, err := c.buildWindowExpr(item, dimMap, metricMap, model, resolver)
	if err != nil {
		return "", err
	}
	return selectItemSQL(windowSQL, selectItemAlias(item.Alias, item.Name), c.dialect), nil
}

func (c *Compiler) buildSelectCase(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
	args *[]any,
) (string, error) {
	caseSQL, err := c.buildCaseExpr(item, dimMap, metricMap, model, resolver, args)
	if err != nil {
		return "", err
	}
	alias := selectItemAlias(item.Alias, item.Name)
	if alias == "" {
		return "", errors.New("case select item requires name or alias")
	}
	return selectItemSQL(caseSQL, alias, c.dialect), nil
}

func selectItemAlias(alias, fallback string) string {
	if alias != "" {
		return alias
	}
	return fallback
}

func selectItemSQL(expr, alias string, d dialect.Dialect) string {
	return expr + " AS " + d.QuoteIdent(alias)
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
	exprFromAST := false
	if w.Expr != nil {
		var err error
		expr, err = CompileExpr(w.Expr, c.dialect, resolver, nil, c.pii)
		if err != nil {
			return "", err
		}
		exprFromAST = true
	}

	var err error
	agg, expr, exprFromAST, err = c.inheritWindowMetricFields(w.Metric, agg, expr, exprFromAST, metricMap, resolver)
	if err != nil {
		return "", err
	}
	if expr != "" && expr != "*" && !exprFromAST {
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
		if exprFromAST {
			head = c.aggregateExpr(agg, expr)
		} else {
			head = c.dialect.Aggregate(agg, expr)
		}
	}

	clauses := make([]string, 0, 4)
	if partBy, err := c.buildWindowPartitionBy(w.PartitionBy, dimMap, resolver); err != nil {
		return "", err
	} else if partBy != "" {
		clauses = append(clauses, partBy)
	}
	if orderBy, err := c.buildWindowOrderBy(w.OrderBy, dimMap, metricMap, model, resolver); err != nil {
		return "", err
	} else if orderBy != "" {
		clauses = append(clauses, orderBy)
	}
	if frame := strings.TrimSpace(w.Frame); frame != "" {
		if !isValidFrame(frame) {
			return "", fmt.Errorf("invalid window frame clause: %q", frame)
		}
		clauses = append(clauses, frame)
	}
	return head + " OVER (" + strings.Join(clauses, " ") + ")", nil
}

func (c *Compiler) inheritWindowMetricFields(
	metricName, agg, expr string,
	exprFromAST bool,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) (string, string, bool, error) {
	mname := strings.TrimSpace(metricName)
	if mname == "" {
		return agg, expr, exprFromAST, nil
	}
	m, ok := metricMap[mname]
	if !ok {
		return "", "", false, fmt.Errorf("window metric not found: %s", mname)
	}
	if agg == "" {
		agg = strings.ToLower(m.Aggregation)
	}
	if expr != "" {
		return agg, expr, exprFromAST, nil
	}
	if m.Expr != nil {
		compiled, err := CompileExpr(m.Expr, c.dialect, resolver, nil, c.pii)
		if err != nil {
			return "", "", false, err
		}
		return agg, compiled, true, nil
	}
	return agg, m.Expression, exprFromAST, nil
}

func (c *Compiler) buildWindowPartitionBy(
	partitionBy []string,
	dimMap map[string]*semantic.Dimension,
	resolver *SchemaResolver,
) (string, error) {
	if len(partitionBy) == 0 {
		return "", nil
	}
	cols := make([]string, 0, len(partitionBy))
	for _, p := range partitionBy {
		if dim, ok := dimMap[p]; ok {
			dimSQL, err := c.dimensionSQL(dim, resolver)
			if err != nil {
				return "", err
			}
			cols = append(cols, dimSQL)
		} else {
			return "", fmt.Errorf("unknown window partition_by dimension: %s", p)
		}
	}
	return "PARTITION BY " + strings.Join(cols, ", "), nil
}

func (c *Compiler) buildWindowOrderBy(
	orderBy []OrderBy,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
) (string, error) {
	if len(orderBy) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(orderBy))
	for _, ob := range orderBy {
		dir := strings.ToUpper(strings.TrimSpace(ob.Direction))
		if dir != "DESC" {
			dir = "ASC"
		}
		var ref string
		if dim, ok := dimMap[ob.Field]; ok {
			dimSQL, err := c.dimensionSQL(dim, resolver)
			if err != nil {
				return "", err
			}
			ref = dimSQL
		} else if metric, ok := metricMap[ob.Field]; ok {
			ref = c.metricAggregate(metric, resolver, dimMap, metricMap, model)
		} else {
			return "", fmt.Errorf("unknown window order_by field: %s", ob.Field)
		}
		parts = append(parts, ref+" "+dir)
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
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
	for _, f := range filters {
		metric, ok := metricMap[f.Field]
		if !ok {
			return "", nil, fmt.Errorf("unknown having field (must be a metric): %s", f.Field)
		}
		aggSQL := c.metricAggregate(metric, resolver, dimMap, metricMap, model)
		switch f.Operator {
		case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
			args = append(args, f.Value)
			op, err := sqlComparator(f.Operator)
			if err != nil {
				return "", nil, fmt.Errorf("having field %q: %w", f.Field, err)
			}
			p := emitPlaceholder()
			parts = append(parts, aggSQL+" "+op+" "+p)
		case OpBetween:
			vals, ok := f.Value.([]any)
			if !ok || len(vals) != 2 {
				return "", nil, fmt.Errorf("having between expects 2 values for metric %q", f.Field)
			}
			args = append(args, vals[0], vals[1])
			p1 := emitPlaceholder()
			p2 := emitPlaceholder()
			parts = append(parts, aggSQL+" BETWEEN "+p1+" AND "+p2)
		case OpIsNull:
			parts = append(parts, aggSQL+" IS NULL")
		case OpIsNotNull:
			parts = append(parts, aggSQL+" IS NOT NULL")
		default:
			return "", nil, fmt.Errorf("operator %q not supported in HAVING for metric %q", f.Operator, f.Field)
		}
	}
	return strings.Join(parts, " AND "), args, nil
}

// sqlComparator translates a logical operator to a SQL comparator. Only
// basic scalar operators are mapped; HAVING only uses these.
func sqlComparator(op string) (string, error) {
	switch op {
	case OpEq:
		return "=", nil
	case OpNeq:
		return "!=", nil
	case OpGt:
		return ">", nil
	case OpGte:
		return ">=", nil
	case OpLt:
		return "<", nil
	case OpLte:
		return "<=", nil
	default:
		return "", fmt.Errorf("unsupported comparator operator: %s", op)
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
		if expr, ok, err := c.calendarGrainFilterExpr(f, dimMap, resolver, args); err != nil {
			return "", err
		} else if ok {
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

func (c *Compiler) calendarGrainFilterExpr(
	f Filter,
	dimMap map[string]*semantic.Dimension,
	resolver *SchemaResolver,
	args *[]any,
) (string, bool, error) {
	dim, ok := dimMap[f.Field]
	if !ok {
		return "", false, nil
	}
	var grain string
	switch {
	case monthGrainFilterUsesDateTrunc(dim, f):
		grain = TimeGrainMonth
	case quarterGrainFilterUsesDateTrunc(dim, f):
		grain = TimeGrainQuarter
	default:
		return "", false, nil
	}
	anchor, ok := calendarAnchorTime(f.Value)
	if !ok {
		return "", false, fmt.Errorf("%s grain filter on %q: expected calendar anchor value", grain, f.Field)
	}
	expr, err := c.dateTruncCompareExpr(grain, resolver.PhysicalColumnRef(dim.ColumnRef), f.Operator, len(*args)+1)
	if err != nil {
		return "", false, err
	}
	*args = append(*args, anchor.UTC())
	return expr, true, nil
}

// resolveFilterLHS returns SQL for the left-hand side of a filter (quoted column, metric expression, or date_trunc).
func (c *Compiler) resolveFilterLHS(field string, dimMap map[string]*semantic.Dimension, metricMap map[string]*semantic.Metric, model *semantic.SemanticModel, resolver *SchemaResolver) (string, error) {
	if dim, ok := dimMap[field]; ok {
		if c.filterFieldHidden(dim, resolver) {
			return "", validationErrWithCode("filters", errmsg.HiddenPIIFieldMsg(field), errmsg.CodeHiddenPIIField, field, nil)
		}
		dimSQL, err := c.dimensionSQL(dim, resolver)
		if err != nil {
			return "", err
		}
		return dimSQL, nil
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
	for _, ob := range orderBy {
		part, err := c.buildOrderByField(ob, dimMap, metricMap, resolver)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, ", "), nil
}

func (c *Compiler) buildOrderByField(
	ob OrderBy,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	resolver *SchemaResolver,
) (string, error) {
	if dim, ok := dimMap[ob.Field]; ok {
		return c.buildOrderByDimension(dim, ob, resolver)
	}
	if metric, ok := metricMap[ob.Field]; ok {
		return orderByMetricSQL(metric.Name, ob.Direction, c.dialect), nil
	}
	var fieldKeys []string
	for k := range dimMap {
		fieldKeys = append(fieldKeys, k)
	}
	for k := range metricMap {
		fieldKeys = append(fieldKeys, k)
	}
	return "", validationErrWithCode("order_by", errmsg.UnknownFieldMsg(ob.Field), errmsg.CodeUnknownField, ob.Field, suggestAlternatives(ob.Field, fieldKeys))
}

func (c *Compiler) buildOrderByDimension(dim *semantic.Dimension, ob OrderBy, resolver *SchemaResolver) (string, error) {
	if c.dimensionFullyHidden(dim, resolver) {
		return "", validationErrWithCode("order_by", errmsg.HiddenPIIFieldMsg(ob.Field), errmsg.CodeHiddenPIIField, ob.Field, nil)
	}
	return orderByDimensionSQL(c.dimensionOutputSQL(dim, resolver), ob.Direction), nil
}

func orderByDirection(direction string) string {
	dir := strings.ToUpper(direction)
	if dir == "" {
		return "ASC"
	}
	return dir
}

func orderByDimensionSQL(dimSQL, direction string) string {
	return dimSQL + " " + orderByDirection(direction)
}

func orderByMetricSQL(metricName, direction string, d dialect.Dialect) string {
	return d.QuoteIdent(metricName) + " " + orderByDirection(direction)
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
