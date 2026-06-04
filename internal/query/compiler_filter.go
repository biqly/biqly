package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

type filterHandler func(c *Compiler, f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error)

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
	}
}

func (c *Compiler) buildFilterPart(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	if handler, ok := filterHandlers[f.Operator]; ok {
		return handler(c, f, lhsSQL, model, args)
	}
	return "", nil, fmt.Errorf("unsupported operator: %s", f.Operator)
}

func (c *Compiler) buildEqFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
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
				switch c.dialect.Name() {
				case "mysql":
					part = fmt.Sprintf("%s = BINARY %s", lhsSQL, c.dialect.Placeholder(len(*args)))
				case "sqlserver":
					part = fmt.Sprintf("%s = %s COLLATE Latin1_General_CS_AS", lhsSQL, c.dialect.Placeholder(len(*args)))
				default:
					part = lhsSQL + " = " + c.dialect.Placeholder(len(*args))
				}
			} else {
				part = lhsSQL + " = " + c.dialect.Placeholder(len(*args))
			}
			parts = append(parts, part)
		}
		return "(" + strings.Join(parts, " OR ") + ")", nil, nil
	}
	*args = append(*args, f.Value)
	if f.CaseSensitive {
		switch c.dialect.Name() {
		case "mysql":
			return fmt.Sprintf("%s = BINARY %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
		case "sqlserver":
			return fmt.Sprintf("%s = %s COLLATE Latin1_General_CS_AS", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
		}
	}
	return lhsSQL + " = " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildNeqFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
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
				switch c.dialect.Name() {
				case "mysql":
					part = fmt.Sprintf("%s != BINARY %s", lhsSQL, c.dialect.Placeholder(len(*args)))
				case "sqlserver":
					part = fmt.Sprintf("%s != %s COLLATE Latin1_General_CS_AS", lhsSQL, c.dialect.Placeholder(len(*args)))
				default:
					part = lhsSQL + " != " + c.dialect.Placeholder(len(*args))
				}
			} else {
				part = lhsSQL + " != " + c.dialect.Placeholder(len(*args))
			}
			parts = append(parts, part)
		}
		return "(" + strings.Join(parts, " AND ") + ")", nil, nil
	}
	*args = append(*args, f.Value)
	if f.CaseSensitive {
		switch c.dialect.Name() {
		case "mysql":
			return fmt.Sprintf("%s != BINARY %s", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
		case "sqlserver":
			return fmt.Sprintf("%s != %s COLLATE Latin1_General_CS_AS", lhsSQL, c.dialect.Placeholder(len(*args))), nil, nil
		}
	}
	return lhsSQL + " != " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildGtFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	*args = append(*args, f.Value)
	return lhsSQL + " > " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildGteFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	*args = append(*args, f.Value)
	return lhsSQL + " >= " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildLtFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	*args = append(*args, f.Value)
	return lhsSQL + " < " + c.dialect.Placeholder(len(*args)), nil, nil
}

func (c *Compiler) buildLteFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	*args = append(*args, f.Value)
	return lhsSQL + " <= " + c.dialect.Placeholder(len(*args)), nil, nil
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

func (c *Compiler) buildContainsFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	vals, isSlice := sliceOfStrings(f.Value)
	if isSlice {
		if len(vals) == 0 {
			return "1=1", nil, nil
		}
		parts := make([]string, 0, len(vals))
		for _, valStr := range vals {
			*args = append(*args, "%"+valStr+"%")
			parts = append(parts, c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive))
		}
		return "(" + strings.Join(parts, " OR ") + ")", nil, nil
	}
	var valStr string
	if str, ok := f.Value.(string); ok {
		valStr = str
	} else {
		valStr = fmt.Sprint(f.Value)
	}
	*args = append(*args, "%"+valStr+"%")
	return c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive), nil, nil
}

func (c *Compiler) buildStartsWithFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	vals, isSlice := sliceOfStrings(f.Value)
	if isSlice {
		if len(vals) == 0 {
			return "1=1", nil, nil
		}
		parts := make([]string, 0, len(vals))
		for _, valStr := range vals {
			*args = append(*args, valStr+"%")
			parts = append(parts, c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive))
		}
		return "(" + strings.Join(parts, " OR ") + ")", nil, nil
	}
	var valStr string
	if str, ok := f.Value.(string); ok {
		valStr = str
	} else {
		valStr = fmt.Sprint(f.Value)
	}
	*args = append(*args, valStr+"%")
	return c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive), nil, nil
}

func (c *Compiler) buildEndsWithFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	vals, isSlice := sliceOfStrings(f.Value)
	if isSlice {
		if len(vals) == 0 {
			return "1=1", nil, nil
		}
		parts := make([]string, 0, len(vals))
		for _, valStr := range vals {
			*args = append(*args, "%"+valStr)
			parts = append(parts, c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive))
		}
		return "(" + strings.Join(parts, " OR ") + ")", nil, nil
	}
	var valStr string
	if str, ok := f.Value.(string); ok {
		valStr = str
	} else {
		valStr = fmt.Sprint(f.Value)
	}
	*args = append(*args, "%"+valStr)
	return c.likeExpression(lhsSQL, c.dialect.Placeholder(len(*args)), f.CaseSensitive), nil, nil
}

func (c *Compiler) buildBetweenOperatorFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return c.buildBetweenFilter(lhsSQL, f.Value, args)
}

func (c *Compiler) buildIsNullFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return lhsSQL + " IS NULL", nil, nil
}

func (c *Compiler) buildIsNotNullFilter(f Filter, lhsSQL string, model *semantic.SemanticModel, args *[]any) (string, []any, error) {
	return lhsSQL + " IS NOT NULL", nil, nil
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
