package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
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
	// Build dimension map
	dimMap := make(map[string]semantic.Dimension)
	for _, d := range model.Dimensions {
		dimMap[d.Name] = d
	}

	// Build metric map
	metricMap := make(map[string]semantic.Metric)
	for _, m := range model.Metrics {
		metricMap[m.Name] = m
	}

	// Build join map
	joinMap := make(map[string]semantic.Join)
	for _, j := range model.Joins {
		joinMap[j.Name] = j
	}

	// Determine which joins are needed by inspecting select columns
	neededJoins := c.determineJoins(lq, model, dimMap)

	// Build SELECT clause
	selectParts, err := c.buildSelect(lq.Select, dimMap, metricMap)
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}

	// Build FROM clause
	fromClause := c.buildFrom(model)

	// Build JOIN clauses
	joinClauses := c.buildJoins(neededJoins, joinMap, model)

	// Build WHERE clause
	whereClause, whereArgs, err := c.buildWhere(lq.Filters, dimMap, metricMap)
	if err != nil {
		return nil, fmt.Errorf("build where: %w", err)
	}
	args := make([]any, 0, len(whereArgs))
	args = append(args, whereArgs...)

	// Build GROUP BY
	groupByClause := c.buildGroupBy(lq.GroupBy, dimMap)

	// Build ORDER BY
	orderByClause := c.buildOrderBy(lq.OrderBy, dimMap, metricMap)

	// Build LIMIT/OFFSET
	limitClause := c.dialect.LimitOffset(lq.Limit, lq.Offset)

	// Assemble SQL
	var sql strings.Builder
	sql.WriteString("SELECT ")
	sql.WriteString(strings.Join(selectParts, ", "))
	sql.WriteString(" FROM ")
	sql.WriteString(fromClause)

	for _, jc := range joinClauses {
		sql.WriteString(" ")
		sql.WriteString(jc)
	}

	if whereClause != "" {
		sql.WriteString(" WHERE ")
		sql.WriteString(whereClause)
	}

	if groupByClause != "" {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(groupByClause)
	}

	if orderByClause != "" {
		sql.WriteString(" ORDER BY ")
		sql.WriteString(orderByClause)
	}

	if limitClause != "" {
		sql.WriteString(" ")
		sql.WriteString(limitClause)
	}

	return &CompiledQuery{
		SQL:  sql.String(),
		Args: args,
	}, nil
}

// CompileWithPermissions compiles a LogicalQuery with row-level security filters injected.
func (c *Compiler) CompileWithPermissions(
	ctx context.Context,
	lq LogicalQuery,
	model *semantic.SemanticModel,
	rowFilters []security.RowFilter,
) (*CompiledQuery, error) {
	// First compile normally
	cq, err := c.Compile(ctx, lq, model)
	if err != nil {
		return nil, err
	}

	// If no row filters, return as-is
	if len(rowFilters) == 0 {
		return cq, nil
	}

	// Build dimension map for field resolution
	dimMap := make(map[string]string)
	for _, d := range model.Dimensions {
		dimMap[d.Name] = d.ColumnRef
	}
	for _, m := range model.Metrics {
		dimMap[m.Name] = m.Expression
	}

	// Inject row filters into WHERE clause
	injector := security.NewPermissionInjector()
	filteredWhere, newArgs, err := injector.InjectRowFilters(
		c.dialect, rowFilters, dimMap, cq.SQL, cq.Args,
	)
	if err != nil {
		return nil, fmt.Errorf("inject row filters: %w", err)
	}

	// If the original SQL has WHERE, append AND filters
	// Otherwise, insert WHERE clause
	if strings.Contains(strings.ToUpper(cq.SQL), " WHERE ") {
		cq.SQL = filteredWhere
		cq.Args = newArgs
	} else {
		// Insert WHERE before GROUP BY / ORDER BY / LIMIT
		injectPoint := len(cq.SQL)
		for _, kw := range []string{"GROUP BY", "ORDER BY", "LIMIT", "OFFSET"} {
			if idx := strings.Index(strings.ToUpper(cq.SQL), kw); idx != -1 && idx < injectPoint {
				injectPoint = idx
			}
		}

		var filterParts []string
		for _, rf := range rowFilters {
			colRef, ok := dimMap[rf.Field]
			if !ok {
				continue
			}
			quoted := c.dialect.QuoteIdent(colRef)
			filterParts = append(filterParts, quoted)
		}

		whereClause := fmt.Sprintf(" WHERE %s IS NOT NULL", filterParts[0])
		cq.SQL = cq.SQL[:injectPoint] + whereClause + cq.SQL[injectPoint:]
		cq.Args = append(cq.Args, newArgs...)
	}

	return cq, nil
}

func (c *Compiler) determineJoins(lq LogicalQuery, model *semantic.SemanticModel, dimMap map[string]semantic.Dimension) []string {
	// Collect all tables referenced in select + filters
	tables := make(map[string]bool)
	tables[model.BaseTable] = true // Always include base table

	for _, item := range lq.Select {
		if dim, ok := dimMap[item.Name]; ok {
			// column_ref can be "table.column" or just "column"
			parts := strings.Split(dim.ColumnRef, ".")
			if len(parts) > 1 {
				tables[parts[0]] = true
			}
		}
	}

	for _, f := range lq.Filters {
		if dim, ok := dimMap[f.Field]; ok {
			parts := strings.Split(dim.ColumnRef, ".")
			if len(parts) > 1 {
				tables[parts[0]] = true
			}
		}
	}

	// Find joins that connect base table to needed tables
	var neededJoins []string
	for _, j := range model.Joins {
		if tables[j.FromTable] || tables[j.ToTable] {
			neededJoins = append(neededJoins, j.Name)
		}
	}

	return neededJoins
}

func (c *Compiler) buildSelect(items []SelectItem, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric) ([]string, error) {
	var parts []string
	for _, item := range items {
		switch item.Type {
		case SelectTypeDimension:
			dim, ok := dimMap[item.Name]
			if !ok {
				return nil, fmt.Errorf("unknown dimension: %s", item.Name)
			}
			col := c.dialect.QuoteIdent(dim.ColumnRef)
			alias := item.Alias
			if alias == "" {
				alias = dim.Name
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", col, c.dialect.QuoteIdent(alias)))

		case SelectTypeMetric:
			metric, ok := metricMap[item.Name]
			if !ok {
				return nil, fmt.Errorf("unknown metric: %s", item.Name)
			}
			agg := c.dialect.Aggregate(metric.Aggregation, metric.Expression)
			alias := item.Alias
			if alias == "" {
				alias = metric.Name
			}
			parts = append(parts, fmt.Sprintf("%s AS %s", agg, c.dialect.QuoteIdent(alias)))
		}
	}
	return parts, nil
}

func (c *Compiler) buildFrom(model *semantic.SemanticModel) string {
	schema := c.dialect.QuoteIdent(model.BaseSchema)
	table := c.dialect.QuoteIdent(model.BaseTable)
	return fmt.Sprintf("%s.%s", schema, table)
}

func (c *Compiler) buildJoins(joinNames []string, joinMap map[string]semantic.Join, model *semantic.SemanticModel) []string {
	var clauses []string
	for _, name := range joinNames {
		j, ok := joinMap[name]
		if !ok {
			continue
		}

		joinType := strings.ToUpper(j.JoinType)
		if joinType == "" {
			joinType = "LEFT"
		}

		fromTable := c.dialect.QuoteIdent(model.BaseSchema) + "." + c.dialect.QuoteIdent(j.FromTable)
		fromCol := c.dialect.QuoteIdent(j.FromColumn)
		toTable := c.dialect.QuoteIdent(model.BaseSchema) + "." + c.dialect.QuoteIdent(j.ToTable)
		toCol := c.dialect.QuoteIdent(j.ToColumn)

		clause := fmt.Sprintf("%s JOIN %s ON %s.%s = %s.%s",
			joinType, toTable, fromTable, fromCol, toTable, toCol)
		clauses = append(clauses, clause)
	}
	return clauses
}

func (c *Compiler) buildWhere(filters []Filter, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	var parts []string
	var args []any

	for _, f := range filters {
		colRef, err := c.resolveField(f.Field, dimMap, metricMap)
		if err != nil {
			return "", nil, err
		}

		part, newArgs, err := c.buildFilterPart(f, colRef, &args)
		if err != nil {
			return "", nil, err
		}
		args = append(args, newArgs...)
		parts = append(parts, part)
	}

	return strings.Join(parts, " AND "), args, nil
}

func (c *Compiler) resolveField(field string, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric) (string, error) {
	if dim, ok := dimMap[field]; ok {
		return dim.ColumnRef, nil
	}
	if metric, ok := metricMap[field]; ok {
		return metric.Expression, nil
	}
	return "", fmt.Errorf("unknown field: %s", field)
}

func (c *Compiler) buildFilterPart(f Filter, colRef string, args *[]any) (string, []any, error) {
	quoted := c.dialect.QuoteIdent(colRef)

	switch f.Operator {
	case OpEq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s = %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpNeq:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s != %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpGt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s > %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpGte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s >= %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpLt:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s < %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpLte:
		*args = append(*args, f.Value)
		return fmt.Sprintf("%s <= %s", quoted, c.dialect.Placeholder(len(*args))), nil, nil
	case OpIn:
		return c.buildInFilter(quoted, f.Value, args)
	case OpNotIn:
		return c.buildNotInFilter(quoted, f.Value, args)
	case OpContains:
		*args = append(*args, fmt.Sprintf("%%%v%%", f.Value))
		return c.dialect.ILike(colRef, c.dialect.Placeholder(len(*args))), nil, nil
	case OpStartsWith:
		*args = append(*args, fmt.Sprintf("%v%%", f.Value))
		return c.dialect.ILike(colRef, c.dialect.Placeholder(len(*args))), nil, nil
	case OpEndsWith:
		*args = append(*args, fmt.Sprintf("%%%v", f.Value))
		return c.dialect.ILike(colRef, c.dialect.Placeholder(len(*args))), nil, nil
	case OpBetween:
		return c.buildBetweenFilter(quoted, f.Value, args)
	case OpIsNull:
		return fmt.Sprintf("%s IS NULL", quoted), nil, nil
	case OpIsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", quoted), nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", f.Operator)
	}
}

func (c *Compiler) buildInFilter(quoted string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, fmt.Errorf("in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return fmt.Sprintf("%s IN (%s)", quoted, strings.Join(placeholders, ", ")), nil, nil
}

func (c *Compiler) buildNotInFilter(quoted string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, fmt.Errorf("not_in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return fmt.Sprintf("%s NOT IN (%s)", quoted, strings.Join(placeholders, ", ")), nil, nil
}

func (c *Compiler) buildBetweenFilter(quoted string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok || len(vals) != 2 {
		return "", nil, fmt.Errorf("between operator expects 2 values")
	}
	*args = append(*args, vals[0], vals[1])
	p1 := c.dialect.Placeholder(len(*args) - 1)
	p2 := c.dialect.Placeholder(len(*args))
	return fmt.Sprintf("%s BETWEEN %s AND %s", quoted, p1, p2), nil, nil
}

func (c *Compiler) buildGroupBy(groupBy []GroupBy, dimMap map[string]semantic.Dimension) string {
	if len(groupBy) == 0 {
		return ""
	}

	var parts []string
	for _, gb := range groupBy {
		if dim, ok := dimMap[gb.Field]; ok {
			parts = append(parts, c.dialect.QuoteIdent(dim.ColumnRef))
		}
	}
	return strings.Join(parts, ", ")
}

func (c *Compiler) buildOrderBy(orderBy []OrderBy, dimMap map[string]semantic.Dimension, metricMap map[string]semantic.Metric) string {
	if len(orderBy) == 0 {
		return ""
	}

	var parts []string
	for _, ob := range orderBy {
		// Could be a metric alias or a dimension
		if dim, ok := dimMap[ob.Field]; ok {
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			parts = append(parts, fmt.Sprintf("%s %s", c.dialect.QuoteIdent(dim.ColumnRef), dir))
		} else if metric, ok := metricMap[ob.Field]; ok {
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			parts = append(parts, fmt.Sprintf("%s %s", c.dialect.QuoteIdent(metric.Name), dir))
		} else {
			// Could be a metric alias from select
			dir := strings.ToUpper(ob.Direction)
			if dir == "" {
				dir = "ASC"
			}
			parts = append(parts, fmt.Sprintf("%s %s", c.dialect.QuoteIdent(ob.Field), dir))
		}
	}

	return strings.Join(parts, ", ")
}
