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
		switch rf.Operator {
		case "eq":
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s = %s", quoted, ph()))
		case "in":
			vals, ok := rf.Value.([]any)
			if !ok {
				return nil, nil, fmt.Errorf("row filter 'in' expects array")
			}
			placeholders := make([]string, len(vals))
			for i, v := range vals {
				extraArgs = append(extraArgs, v)
				placeholders[i] = ph()
			}
			preds = append(preds, fmt.Sprintf("%s IN (%s)", quoted, strings.Join(placeholders, ", ")))
		default:
			extraArgs = append(extraArgs, rf.Value)
			preds = append(preds, fmt.Sprintf("%s = %s", quoted, ph()))
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
		return nil
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
	return !FieldIsDenied(policy, qualified, unqualified)
}

func joinStr(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	var result strings.Builder
	result.WriteString(parts[0])
	for i := 1; i < len(parts); i++ {
		result.WriteString(sep + parts[i])
	}
	return result.String()
}
