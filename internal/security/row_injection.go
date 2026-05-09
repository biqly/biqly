package security

import (
	"fmt"

	"github.com/biqly/biqly/internal/dialect"
)

// PermissionInjector injects row-level security filters into compiled SQL.
type PermissionInjector struct{}

// NewPermissionInjector creates a new permission injector.
func NewPermissionInjector() *PermissionInjector {
	return &PermissionInjector{}
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

	var parts []string
	if existingWhere != "" {
		parts = append(parts, existingWhere)
	}

	for _, rf := range filters {
		colRef, ok := dimMap[rf.Field]
		if !ok {
			return "", nil, fmt.Errorf("row filter references unknown field: %s", rf.Field)
		}

		quoted := d.QuoteIdent(colRef)

		switch rf.Operator {
		case "eq":
			args = append(args, rf.Value)
			parts = append(parts, fmt.Sprintf("%s = %s", quoted, d.Placeholder(len(args))))
		case "in":
			vals, ok := rf.Value.([]any)
			if !ok {
				return "", nil, fmt.Errorf("row filter 'in' expects array")
			}
			placeholders := make([]string, len(vals))
			for i, v := range vals {
				args = append(args, v)
				placeholders[i] = d.Placeholder(len(args))
			}
			parts = append(parts, fmt.Sprintf("%s IN (%s)", quoted, joinStr(placeholders, ", ")))
		default:
			args = append(args, rf.Value)
			parts = append(parts, fmt.Sprintf("%s = %s", quoted, d.Placeholder(len(args))))
		}
	}

	return joinStr(parts, " AND "), args, nil
}

// CheckFieldAccess validates that all selected/filter fields are allowed.
func (pi *PermissionInjector) CheckFieldAccess(
	policy *PermissionPolicy,
	modelName string,
	selectFields []string,
	filterFields []string,
) error {
	for _, field := range selectFields {
		qualified := modelName + "." + field
		if !pi.isFieldAllowed(policy, qualified, field) {
			return fmt.Errorf("field %s is not accessible for user %s", field, policy.UserID)
		}
	}

	for _, field := range filterFields {
		qualified := modelName + "." + field
		if !pi.isFieldAllowed(policy, qualified, field) {
			return fmt.Errorf("filter field %s is not accessible for user %s", field, policy.UserID)
		}
	}

	return nil
}

func (pi *PermissionInjector) isFieldAllowed(policy *PermissionPolicy, qualified, unqualified string) bool {
	for _, denied := range policy.DeniedFields {
		if denied == qualified || denied == unqualified {
			return false
		}
	}
	return true
}

func joinStr(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
