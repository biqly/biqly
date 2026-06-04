package semantic

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateExprStrict recursively checks the validity of an expression AST.
func ValidateExprStrict(
	node ExprNode,
	allowedColumns map[string]bool,
	allowedMetrics map[string]bool,
	allowedDimensions map[string]bool,
	allowMetrics bool,
	depth int,
) error {
	if node == nil {
		return nil
	}

	if depth > 10 {
		return errors.New("expression nesting depth exceeds limit of 10")
	}

	switch e := node.(type) {
	case LiteralExpr:
		return nil

	case ColumnRefExpr:
		return validateColumnRefExpr(e, allowedColumns)

	case MetricRefExpr:
		if !allowMetrics {
			return fmt.Errorf("metric reference not allowed in this context: %s", e.Name)
		}
		nameLower := strings.ToLower(e.Name)
		if !allowedMetrics[nameLower] {
			return fmt.Errorf("unknown metric reference: %s", e.Name)
		}
		return nil

	case DimensionRefExpr:
		nameLower := strings.ToLower(e.Name)
		if !allowedDimensions[nameLower] {
			return fmt.Errorf("unknown dimension reference: %s", e.Name)
		}
		return nil

	case BinaryExpr:
		if err := ValidateExprStrict(e.Left, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1); err != nil {
			return err
		}
		return ValidateExprStrict(e.Right, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1)

	case UnaryExpr:
		return ValidateExprStrict(e.Expr, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1)

	case FunctionCallExpr:
		return validateFunctionCallExpr(e, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth)

	case CaseExpr:
		return validateCaseExpr(e, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth)

	default:
		return fmt.Errorf("unknown expression node type: %T", node)
	}
}

func validateColumnRefExpr(e ColumnRefExpr, allowedColumns map[string]bool) error {
	key := strings.ToLower(e.Column)
	if e.Table != "" {
		key = strings.ToLower(e.Table + "." + e.Column)
	}
	if !allowedColumns[key] {
		// Fallback: check unqualified column if allowedColumns has table.column format
		if e.Table == "" {
			found := false
			for col := range allowedColumns {
				parts := strings.Split(col, ".")
				if len(parts) == 2 && parts[1] == key {
					found = true
					break
				}
			}
			if found {
				return nil
			}
		}
		return fmt.Errorf("unknown column reference: %s", key)
	}
	return nil
}

func validateFunctionCallExpr(
	e FunctionCallExpr,
	allowedColumns map[string]bool,
	allowedMetrics map[string]bool,
	allowedDimensions map[string]bool,
	allowMetrics bool,
	depth int,
) error {
	funcName := strings.ToUpper(e.Name)
	arity, ok := AllowedFunctions[funcName]
	if !ok {
		return fmt.Errorf("disallowed function call: %s", e.Name)
	}

	// Arity check using switch
	switch {
	case funcName == "ROUND":
		if len(e.Args) != 1 && len(e.Args) != 2 {
			return fmt.Errorf("function %s requires 1 or 2 arguments, got %d", e.Name, len(e.Args))
		}
	case arity == -1:
		if len(e.Args) < 1 {
			return fmt.Errorf("function %s requires at least 1 argument, got %d", e.Name, len(e.Args))
		}
	default:
		if len(e.Args) != arity {
			return fmt.Errorf("function %s requires exactly %d arguments, got %d", e.Name, arity, len(e.Args))
		}
	}

	// Validate arguments
	for _, arg := range e.Args {
		if err := ValidateExprStrict(arg, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func validateCaseExpr(
	e CaseExpr,
	allowedColumns map[string]bool,
	allowedMetrics map[string]bool,
	allowedDimensions map[string]bool,
	allowMetrics bool,
	depth int,
) error {
	for i, cond := range e.Conditions {
		if err := ValidateExprStrict(cond.When, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1); err != nil {
			return fmt.Errorf("invalid when condition %d: %w", i, err)
		}
		if err := ValidateExprStrict(cond.Then, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1); err != nil {
			return fmt.Errorf("invalid then result %d: %w", i, err)
		}
	}
	if e.ElseExpr != nil {
		if err := ValidateExprStrict(e.ElseExpr, allowedColumns, allowedMetrics, allowedDimensions, allowMetrics, depth+1); err != nil {
			return fmt.Errorf("invalid else result: %w", err)
		}
	}
	return nil
}
