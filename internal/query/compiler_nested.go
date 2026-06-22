package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/errmsg"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

func logicalQueryFromBody(body SubqueryBody) *LogicalQuery {
	return &LogicalQuery{
		Select:  body.Select,
		Filters: body.Filters,
		GroupBy: body.GroupBy,
		Having:  body.Having,
		OrderBy: body.OrderBy,
		Limit:   body.Limit,
		Offset:  body.Offset,
	}
}

// compileSubqueryBody builds a nested SELECT statement and appends bind args to args.
func (c *Compiler) compileSubqueryBody(
	body SubqueryBody,
	model *semantic.SemanticModel,
	args *[]any,
) (string, error) {
	inner := logicalQueryFromBody(body)
	cq, err := c.compileStatement(c.compileCtx, inner, model, c.buildFrom(model), "", args, c.rowFilters)
	if err != nil {
		return "", err
	}
	return cq.SQL, nil
}

func (c *Compiler) buildWithClause(ctes []CTE, model *semantic.SemanticModel, args *[]any) (string, error) {
	if len(ctes) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(ctes))
	for _, cte := range ctes {
		name := strings.TrimSpace(cte.Name)
		if name == "" {
			return "", errors.New("cte name is required")
		}
		innerSQL, err := c.compileSubqueryBody(cte.Subquery(), model, args)
		if err != nil {
			return "", fmt.Errorf("cte %q: %w", name, err)
		}
		parts = append(parts, fmt.Sprintf("%s AS (%s)", c.dialect.QuoteIdent(name), innerSQL))
	}
	return "WITH " + strings.Join(parts, ", ") + " ", nil
}

func (c *Compiler) resolveFromClause(lq *LogicalQuery, model *semantic.SemanticModel, args *[]any) (string, error) {
	if lq.FromSubquery != nil {
		innerSQL, err := c.compileSubqueryBody(*lq.FromSubquery, model, args)
		if err != nil {
			return "", fmt.Errorf("from_subquery: %w", err)
		}
		alias := strings.TrimSpace(lq.FromAlias)
		if alias == "" {
			alias = "_sub"
		}
		return fmt.Sprintf("(%s) AS %s", innerSQL, c.dialect.QuoteIdent(alias)), nil
	}
	if cte := strings.TrimSpace(lq.FromCTE); cte != "" {
		return c.dialect.QuoteIdent(cte), nil
	}
	return c.buildFrom(model), nil
}

// compileStatement assembles SELECT..FROM..[joins]..WHERE.. with bind args appended to args.
//
// rowFilters are mandatory row-level security predicates that must be ANDed
// into the WHERE clause. They are appended last so their placeholders take
// the highest indices, and they share the same WHERE block as user filters
// (no regex injection on assembled SQL).
func (c *Compiler) compileStatement(
	ctx context.Context,
	lq *LogicalQuery,
	model *semantic.SemanticModel,
	fromClause string,
	withPrefix string,
	args *[]any,
	rowFilters []security.RowFilter,
) (*CompiledQuery, error) {
	c = c.withCompileCtx(ctx)
	lq.EnsureGroupBySelected()

	dimMap, metricMap, joinMap := buildCompilerMaps(lq, model)
	resolver := NewSchemaResolver(model, lq)

	neededJoins, unreachableTables := c.determineJoins(lq, model, dimMap, metricMap, resolver)
	if err := validateReachableTables(unreachableTables, lq); err != nil {
		return nil, err
	}

	selectParts, err := c.buildSelect(lq.Select, dimMap, metricMap, model, resolver, args)
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	joinClauses := c.buildJoins(neededJoins, joinMap, model, resolver)

	whereClause, err := c.buildWhere(lq.Filters, dimMap, metricMap, model, resolver, args)
	if err != nil {
		return nil, fmt.Errorf("build where: %w", err)
	}

	groupByClause, err := c.buildGroupBy(lq.GroupBy, dimMap, resolver)
	if err != nil {
		return nil, fmt.Errorf("build group by: %w", err)
	}

	havingClause, havingArgs, err := c.buildHaving(lq.Having, dimMap, metricMap, model, resolver, len(*args))
	if err != nil {
		return nil, fmt.Errorf("build having: %w", err)
	}
	*args = append(*args, havingArgs...)

	// Row-level security predicates: built once we know how many bind args
	// preceded them so the placeholders take indices after user filters.
	// Empty rowFilters → no-op.
	rowFilterPreds, err := c.buildRowFilterPreds(rowFilters, model, args)
	if err != nil {
		return nil, fmt.Errorf("build row filters: %w", err)
	}

	orderByClause, err := c.buildOrderBy(lq.OrderBy, dimMap, metricMap, resolver)
	if err != nil {
		return nil, fmt.Errorf("build order by: %w", err)
	}

	limitClause := c.dialect.LimitOffset(lq.Limit, lq.Offset)

	chunks := make([]string, 0, 16+len(joinClauses))
	chunks = append(chunks, withPrefix, "SELECT ", strings.Join(selectParts, ", "), " FROM ", fromClause)

	for _, jc := range joinClauses {
		chunks = append(chunks, " ", jc)
	}
	if whereClause != "" || len(rowFilterPreds) > 0 {
		chunks = append(chunks, " WHERE ")
		if whereClause != "" {
			chunks = append(chunks, whereClause)
			if len(rowFilterPreds) > 0 {
				chunks = append(chunks, " AND ")
			}
		}
		if len(rowFilterPreds) > 0 {
			chunks = append(chunks, strings.Join(rowFilterPreds, " AND "))
		}
	}
	if groupByClause != "" {
		chunks = append(chunks, " GROUP BY ", groupByClause)
	}
	if havingClause != "" {
		chunks = append(chunks, " HAVING ", havingClause)
	}
	if orderByClause != "" {
		chunks = append(chunks, " ORDER BY ", orderByClause)
	} else if defaultOrder := c.dialect.DefaultOrderBy(); defaultOrder != "" && limitClause != "" {
		chunks = append(chunks, " ORDER BY ", defaultOrder)
	}
	if limitClause != "" {
		chunks = append(chunks, " ", limitClause)
	}

	return &CompiledQuery{SQL: strings.Join(chunks, ""), Args: append([]any(nil), *args...)}, nil
}

func (c *Compiler) buildInSubqueryFilter(lhsSQL string, f Filter, model *semantic.SemanticModel, positive bool, args *[]any) (string, []any, error) {
	if f.Subquery == nil {
		return "", nil, errors.New("subquery filter missing body")
	}
	if strings.TrimSpace(f.Subquery.ResultField) == "" {
		return "", nil, errors.New("subquery filter requires result_field")
	}
	subSQL, err := c.compileSubqueryBody(f.Subquery.Body, model, args)
	if err != nil {
		return "", nil, err
	}
	if len(f.Subquery.Body.Select) != 1 {
		return "", nil, errors.New("subquery filter body must select exactly one field")
	}
	sel := f.Subquery.Body.Select[0]
	if sel.Name != f.Subquery.ResultField && sel.Alias != f.Subquery.ResultField {
		return "", nil, fmt.Errorf("subquery filter body does not select the required result_field %q", f.Subquery.ResultField)
	}
	op := "IN"
	if !positive {
		op = "NOT IN"
	}
	return fmt.Sprintf("%s %s (%s)", lhsSQL, op, subSQL), nil, nil
}

// validateReachableTables checks whether every table referenced in the query is
// reachable from the base table through the model's join graph. Subqueries and
// CTEs provide their own FROM clause and are not checked here.
//
// Table keys that look like mis-parsed calculated expressions (containing brackets)
// are silently skipped — they are artifacts of addTableFromColumnRef's naive
// splitDot parser on metric Expression fields.
func validateReachableTables(unreachableTables []string, lq *LogicalQuery) error {
	if len(unreachableTables) == 0 || lq.FromSubquery != nil || lq.FromCTE != "" {
		return nil
	}
	filtered := make([]string, 0, len(unreachableTables))
	for _, t := range unreachableTables {
		if !strings.ContainsAny(t, "[]") {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return &ValidationError{
		Field:   "filters",
		Code:    errmsg.CodeUnreachableTable,
		Message: fmt.Sprintf("query references tables not reachable through model relations: %s; add these tables to the model's relations or remove the fields that reference them", strings.Join(filtered, ", ")),
	}
}

// buildCompilerMaps builds the dimMap, metricMap, and joinMap lookup maps used
// by the query compiler. dimMap values are cloned so time grain overrides from
// the logical query don't mutate the model.
func buildCompilerMaps(lq *LogicalQuery, model *semantic.SemanticModel) (
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	joinMap map[string]semantic.Join,
) {
	dimMap = make(map[string]*semantic.Dimension, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dimMap[d.Name] = new(d)
	}
	for _, gb := range lq.GroupBy {
		if gb.TimeGrain == "" {
			continue
		}
		if dim, ok := dimMap[gb.Field]; ok {
			dim.TimeGrain = gb.TimeGrain
		}
	}
	metricMap = make(map[string]*semantic.Metric, len(model.Metrics))
	for _, m := range model.Metrics {
		metricMap[m.Name] = new(m)
	}
	joinMap = make(map[string]semantic.Join)
	for _, j := range model.Joins {
		joinMap[j.Name] = j
	}
	return
}
