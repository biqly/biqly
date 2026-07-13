package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

type filterHandler func(c *Compiler, f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error)

var filterHandlers map[string]filterHandler

func init() {
	filterHandlers = map[string]filterHandler{
		OpEq:         (*Compiler).buildEqFilter,
		OpNeq:        (*Compiler).buildNeqFilter,
		OpGt:         (*Compiler).buildGtFilter,
		OpGte:        (*Compiler).buildGteFilter,
		OpLt:         (*Compiler).buildLtFilter,
		OpLte:        (*Compiler).buildLteFilter,
		OpIn:         (*Compiler).buildInOperatorFilter,
		OpNotIn:      (*Compiler).buildNotInOperatorFilter,
		OpContains:   (*Compiler).buildContainsFilter,
		OpStartsWith: (*Compiler).buildStartsWithFilter,
		OpEndsWith:   (*Compiler).buildEndsWithFilter,
		OpBetween:    (*Compiler).buildBetweenOperatorFilter,
		OpIsNull:     (*Compiler).buildIsNullFilter,
		OpIsNotNull:  (*Compiler).buildIsNotNullFilter,
		OpIsEmpty:    (*Compiler).buildIsEmptyFilter,
		OpIsNotEmpty: (*Compiler).buildIsNotEmptyFilter,
	}
}

func (c *Compiler) buildFilterPart(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	if handler, ok := filterHandlers[f.Operator]; ok {
		return handler(c, f, lhsSQL, model, args)
	}
	return "", nil, fmt.Errorf("unsupported operator: %s", f.Operator)
}

func caseSensitiveComparison(dialectName, lhsSQL, op, placeholder string) string {
	switch dialectName {
	case "mysql":
		return lhsSQL + " " + op + " BINARY " + placeholder
	case "sqlserver":
		return lhsSQL + " " + op + " " + placeholder + " COLLATE Latin1_General_CS_AS"
	default:
		return lhsSQL + " " + op + " " + placeholder
	}
}

func (c *Compiler) buildEqualityFilter(f Filter, lhsSQL string, args *[]any, op, sliceJoin string) (string, []any, error) {
	vals, isSlice := sliceOfStrings(f.Value)
	if isSlice {
		if len(vals) == 0 {
			return "1=1", nil, nil
		}
		parts := make([]string, 0, len(vals))
		for _, valStr := range vals {
			*args = append(*args, valStr)
			var part string
			if f.CaseSensitive {
				part = caseSensitiveComparison(c.dialect.Name(), lhsSQL, op, c.dialect.Placeholder(len(*args)))
			} else {
				part = lhsSQL + " " + op + " " + c.dialect.Placeholder(len(*args))
			}
			parts = append(parts, part)
		}
		return "(" + strings.Join(parts, " "+sliceJoin+" ") + ")", nil, nil
	}
	*args = append(*args, f.Value)
	if f.CaseSensitive {
		return caseSensitiveComparison(c.dialect.Name(), lhsSQL, op, c.dialect.Placeholder(len(*args))), nil, nil
	}
	return lhsSQL + " " + op + " " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildEqFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildEqualityFilter(f, lhsSQL, args, "=", "OR")
}

func (c *Compiler) buildNeqFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildEqualityFilter(f, lhsSQL, args, "!=", "AND")
}

func (c *Compiler) buildGtFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildComparisonFilter(f, lhsSQL, args, ">")
}

func (c *Compiler) buildGteFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildComparisonFilter(f, lhsSQL, args, ">=")
}

func (c *Compiler) buildLtFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildComparisonFilter(f, lhsSQL, args, "<")
}

func (c *Compiler) buildLteFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildComparisonFilter(f, lhsSQL, args, "<=")
}

func (c *Compiler) buildComparisonFilter(f Filter, lhsSQL string, args *[]any, op string) (string, []any, error) {
	*args = append(*args, f.Value)
	return lhsSQL + " " + op + " " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildInOperatorFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	if f.Subquery != nil {
		return c.buildInSubqueryFilter(lhsSQL, f, model, true, args)
	}
	return c.buildInFilter(lhsSQL, f.Value, args)
}

func (c *Compiler) buildNotInOperatorFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	if f.Subquery != nil {
		return c.buildInSubqueryFilter(lhsSQL, f, model, false, args)
	}
	return c.buildNotInFilter(lhsSQL, f.Value, args)
}

// buildLikeFilter renders a LIKE predicate (case-sensitivity per f) whose bound
// value is produced by wrap(v). Contains/StartsWith/EndsWith differ only in
// where the % wildcard sits, so they share this body.
func (c *Compiler) buildLikeFilter(f Filter, lhsSQL string, args *[]any, wrap func(string) string) (string, []any, error) {
	vals, isSlice := sliceOfStrings(f.Value)
	if isSlice {
		if len(vals) == 0 {
			return "1=1", nil, nil
		}
		parts := make([]string, 0, len(vals))
		for _, valStr := range vals {
			*args = append(*args, wrap(valStr))
			parts = append(parts, c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive))
		}
		return "(" + strings.Join(parts, " OR ") + ")", nil, nil
	}
	valStr, ok := f.Value.(string)
	if !ok {
		valStr = fmt.Sprint(f.Value)
	}
	*args = append(*args, wrap(valStr))
	return c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive), nil, nil
}

func (c *Compiler) buildContainsFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildLikeFilter(f, lhsSQL, args, func(v string) string { return "%" + v + "%" })
}

func (c *Compiler) buildStartsWithFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildLikeFilter(f, lhsSQL, args, func(v string) string { return v + "%" })
}

func (c *Compiler) buildEndsWithFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildLikeFilter(f, lhsSQL, args, func(v string) string { return "%" + v })
}

func (c *Compiler) buildBetweenOperatorFilter(f Filter, lhsSQL string, _ *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildBetweenFilter(lhsSQL, f.Value, args)
}

func (*Compiler) buildIsNullFilter(_ Filter, lhsSQL string, _ *semantic.SemanticModel, _ *[]any) (string, []any, error) {
	return lhsSQL + " IS NULL", nil, nil
}

func (*Compiler) buildIsNotNullFilter(_ Filter, lhsSQL string, _ *semantic.SemanticModel, _ *[]any) (string, []any, error) {
	return lhsSQL + " IS NOT NULL", nil, nil
}

func (*Compiler) buildIsEmptyFilter(_ Filter, lhsSQL string, _ *semantic.SemanticModel, _ *[]any) (string, []any, error) {
	return lhsSQL + " = ''", nil, nil
}

func (*Compiler) buildIsNotEmptyFilter(_ Filter, lhsSQL string, _ *semantic.SemanticModel, _ *[]any) (string, []any, error) {
	return lhsSQL + " != ''", nil, nil
}

func (c *Compiler) buildInFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, errors.New("in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return lhsSQL + " IN (" + strings.Join(placeholders, ", ") + ")", nil, nil
}

func (c *Compiler) buildNotInFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok {
		return "", nil, errors.New("not_in operator expects array")
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		*args = append(*args, v)
		placeholders[i] = c.dialect.Placeholder(len(*args))
	}
	return lhsSQL + " NOT IN (" + strings.Join(placeholders, ", ") + ")", nil, nil
}

func (c *Compiler) buildBetweenFilter(lhsSQL string, value any, args *[]any) (string, []any, error) {
	vals, ok := value.([]any)
	if !ok || len(vals) != 2 {
		return "", nil, errors.New("between operator expects 2 values")
	}
	*args = append(*args, vals[0], vals[1])
	p1 := c.dialect.Placeholder(len(*args) - 1)
	p2 := c.dialect.Placeholder(len(*args))
	return lhsSQL + " BETWEEN " + p1 + " AND " + p2, nil, nil
}
