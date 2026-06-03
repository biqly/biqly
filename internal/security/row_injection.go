package security

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/errmsg"
)

// PermissionInjector injects row-level security filters into compiled SQL.
type PermissionInjector struct{}

// NewPermissionInjector creates a new permission injector.
func NewPermissionInjector() *PermissionInjector {
	return &PermissionInjector{}
}

// BuildRowFilterPredicates builds SQL predicate fragments and bind values for row filters.
// initialArgCount is the number of bind parameters already present (so new placeholders are numbered after them).
// If omitUnknownFields is true, filters referencing a missing dimMap key are skipped; otherwise an error is returned.
//
// Unsupported operators are rejected — they MUST NOT fall back to equality, since
// a silent fallback (e.g. "neq" → "=") would expose restricted rows.
func BuildRowFilterPredicates(
	d dialect.Dialect,
	dimMap map[string]string,
	filters []RowFilter,
	initialArgCount int,
	omitUnknownFields bool,
) ([]string, []any, error) {
	var preds []string
	var extraArgs []any
	ph := func() string {
		return d.Placeholder(initialArgCount + len(extraArgs))
	}
	for _, rf := range filters {
		colRef, ok := dimMap[rf.Field]
		if !ok {
			if omitUnknownFields {
				continue
			}
			return nil, nil, errmsg.RowFilterUnknownField(rf.Field)
		}
		quoted := d.QuoteIdent(colRef)
		op := strings.ToLower(strings.TrimSpace(rf.Operator))
		switch op {
		case "", "eq":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s = %s", quoted, ph()))
		case "neq":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s <> %s", quoted, ph()))
		case "gt":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s > %s", quoted, ph()))
		case "gte":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s >= %s", quoted, ph()))
		case "lt":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s < %s", quoted, ph()))
		case "lte":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s <= %s", quoted, ph()))
		case "in", "not_in":
			vals, ok := rf.Value.([]any)
			if !ok {
				return nil, nil, fmt.Errorf("row filter %q expects array value for field %q", op, rf.Field)
			}
			if len(vals) == 0 {
				return nil, nil, fmt.Errorf("row filter %q requires at least one value for field %q", op, rf.Field)
			}
			placeholders := make([]string, len(vals))
			for i, v := range vals {
				extraArgs = append(extraArgs, v)
				placeholders[i] = ph()
			}
			keyword := "IN"
			if op == "not_in" {
				keyword = "NOT IN"
			}
			preds = append(preds, fmt.Sprintf("%s %s (%s)", quoted, keyword, strings.Join(placeholders, ", ")))
		case "is_null":
			preds = append(preds, fmt.Sprintf("%s IS NULL", quoted))
		case "is_not_null":
			preds = append(preds, fmt.Sprintf("%s IS NOT NULL", quoted))
		default:
			return nil, nil, fmt.Errorf("row filter operator %q is not supported for field %q", rf.Operator, rf.Field)
		}
	}
	return preds, extraArgs, nil
}

// InjectRowFilters adds mandatory row-level security filters to the compiled query.
func (pi *PermissionInjector) InjectRowFilters(
	d dialect.Dialect,
	filters []RowFilter,
	dimMap map[string]string, // field name -> column reference
	existingWhere string,
	args []any,
) (string, []any, error) {
	if len(filters) == 0 {
		return existingWhere, args, nil
	}

	filterPreds, extraArgs, err := BuildRowFilterPredicates(d, dimMap, filters, len(args), false)
	if err != nil {
		return "", nil, err
	}
	var parts []string
	if existingWhere != "" {
		parts = append(parts, existingWhere)
	}
	parts = append(parts, filterPreds...)
	return joinStr(parts, " AND "), append(args, extraArgs...), nil
}

// CheckFieldAccess validates that all selected/filter fields are allowed.
func (pi *PermissionInjector) CheckFieldAccess(
	policy *PermissionPolicy,
	modelName string,
	selectFields []string,
	filterFields []string,
) error {
	if policy == nil {
		return fmt.Errorf("no permission policy supplied for model %s", modelName)
	}
	uid := policy.UserID
	for _, field := range selectFields {
		qualified := modelName + "." + field
		if !pi.isFieldAllowed(policy, qualified, field) {
			return fmt.Errorf("field %s is not accessible for user %s", field, uid)
		}
	}

	for _, field := range filterFields {
		qualified := modelName + "." + field
		if !pi.isFieldAllowed(policy, qualified, field) {
			return fmt.Errorf("filter field %s is not accessible for user %s", field, uid)
		}
	}

	return nil
}

func (pi *PermissionInjector) isFieldAllowed(policy *PermissionPolicy, qualified, unqualified string) bool {
	return !FieldIsDenied(policy, qualified, unqualified) && !PIIFieldIsHidden(policy, qualified, unqualified)
}

func joinStr(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	var result strings.Builder
	total := len(parts[0])
	for i := 1; i < len(parts); i++ {
		total += len(sep) + len(parts[i])
	}
	result.Grow(total)
	result.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		result.WriteString(sep)
		result.WriteString(parts[i])
	}
	return result.String()
}
