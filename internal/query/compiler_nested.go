package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	cq, err := c.compileStatement(context.Background(), inner, model, c.buildFrom(model), "", args, nil)
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
	_ = ctx
	lq.EnsureGroupBySelected()

	dimMap := make(map[string]*semantic.Dimension, len(model.Dimensions))
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
	metricMap := make(map[string]*semantic.Metric, len(model.Metrics))
	for _, m := range model.Metrics {
		metricMap[m.Name] = new(m)
	}
	joinMap := make(map[string]semantic.Join)
	for _, j := range model.Joins {
		joinMap[j.Name] = j
	}

	resolver := NewSchemaResolver(model, lq)

	neededJoins := c.determineJoins(lq, model, dimMap, metricMap, resolver)

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

	var sql strings.Builder
	sql.Grow(256)
	sql.WriteString(withPrefix)
	sql.WriteString("SELECT ")
	sql.WriteString(strings.Join(selectParts, ", "))
	sql.WriteString(" FROM ")
	sql.WriteString(fromClause)

	for _, jc := range joinClauses {
		sql.WriteString(" ")
		sql.WriteString(jc)
	}
	if whereClause != "" || len(rowFilterPreds) > 0 {
		sql.WriteString(" WHERE ")
		if whereClause != "" {
			sql.WriteString(whereClause)
			if len(rowFilterPreds) > 0 {
				sql.WriteString(" AND ")
			}
		}
		if len(rowFilterPreds) > 0 {
			sql.WriteString(strings.Join(rowFilterPreds, " AND "))
		}
	}
	if groupByClause != "" {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(groupByClause)
	}
	if havingClause != "" {
		sql.WriteString(" HAVING ")
		sql.WriteString(havingClause)
	}
	if orderByClause != "" {
		sql.WriteString(" ORDER BY ")
		sql.WriteString(orderByClause)
	} else if defaultOrder := c.dialect.DefaultOrderBy(); defaultOrder != "" && limitClause != "" {
		sql.WriteString(" ORDER BY ")
		sql.WriteString(defaultOrder)
	}
	if limitClause != "" {
		sql.WriteString(" ")
		sql.WriteString(limitClause)
	}

	return &CompiledQuery{SQL: sql.String(), Args: append([]any(nil), *args...)}, nil
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
	_ = f.Subquery.ResultField // validated at compile time via select list in body
	op := "IN"
	if !positive {
		op = "NOT IN"
	}
	return fmt.Sprintf("%s %s (%s)", lhsSQL, op, subSQL), nil, nil
}
