package query

import (
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

func (c *Compiler) buildCaseExpr(
	item SelectItem,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
	args *[]any,
) (string, error) {
	if item.Case == nil || len(item.Case.Branches) == 0 {
		return "", fmt.Errorf("case select item %q missing case branches", item.Name)
	}
	parts := make([]string, 0, len(item.Case.Branches)+3)
	parts = append(parts, "CASE")
	for i, br := range item.Case.Branches {
		if len(br.When) == 0 {
			return "", fmt.Errorf("case branch %d missing when filters", i)
		}
		pred, err := c.buildPredicate(br.When, dimMap, metricMap, model, resolver, args)
		if err != nil {
			return "", fmt.Errorf("case branch %d: %w", i, err)
		}
		thenSQL, err := c.buildCaseThen(br.Then, dimMap, resolver, args)
		if err != nil {
			return "", fmt.Errorf("case branch %d then: %w", i, err)
		}
		parts = append(parts, "WHEN "+pred+" THEN "+thenSQL)
	}
	if item.Case.Else != nil {
		elseSQL, err := c.buildCaseThen(*item.Case.Else, dimMap, resolver, args)
		if err != nil {
			return "", fmt.Errorf("case else: %w", err)
		}
		parts = append(parts, "ELSE "+elseSQL)
	}
	parts = append(parts, "END")
	return strings.Join(parts, " "), nil
}

func (c *Compiler) buildCaseThen(
	then CaseThen,
	dimMap map[string]*semantic.Dimension,
	resolver *SchemaResolver,
	args *[]any,
) (string, error) {
	switch strings.ToLower(strings.TrimSpace(then.Type)) {
	case CaseThenTypeDimension, "":
		if then.Dimension == "" {
			return "", errors.New("case then dimension name required")
		}
		dim, ok := dimMap[then.Dimension]
		if !ok {
			return "", fmt.Errorf("unknown case then dimension: %s", then.Dimension)
		}
		dimSQL, err := c.dimensionSQL(dim, resolver)
		if err != nil {
			return "", err
		}
		return dimSQL, nil
	case CaseThenTypeLiteral:
		return c.formatLiteral(then.Literal, args)
	default:
		return "", fmt.Errorf("invalid case then type: %s", then.Type)
	}
}

func (c *Compiler) buildPredicate(
	filters []Filter,
	dimMap map[string]*semantic.Dimension,
	metricMap map[string]*semantic.Metric,
	model *semantic.SemanticModel,
	resolver *SchemaResolver,
	args *[]any,
) (string, error) {
	if len(filters) == 0 {
		return "", errors.New("empty predicate")
	}
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
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

func (c *Compiler) formatLiteral(value any, args *[]any) (string, error) {
	if value == nil {
		return "NULL", nil
	}
	switch v := value.(type) {
	case bool:
		if v {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int, int32, int64, float32, float64:
		*args = append(*args, v)
		return c.dialect.Placeholder(len(*args)), nil
	case string:
		*args = append(*args, v)
		return c.dialect.Placeholder(len(*args)), nil
	default:
		*args = append(*args, value)
		return c.dialect.Placeholder(len(*args)), nil
	}
}
