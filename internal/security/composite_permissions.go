package security

import (
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"reflect"
	"slices"
)

// CheckCompositeAccess verifies the policy grants access to every component
// model that makes up a composite model.
//
// Fail-closed: a nil policy denies, and a composite is only queryable when the
// user can reach all of its parts. If the user lacks access to any single
// component model, the entire composite is denied — there is no partial access.
func (pm *PermissionManager) CheckCompositeAccess(policy *PermissionPolicy, componentModelNames []string) error {
	if policy == nil {
		return errors.New("no permission policy supplied for composite model")
	}
	for _, name := range componentModelNames {
		if err := pm.CheckModelAccess(policy, name); err != nil {
			return fmt.Errorf("composite access denied: %w", err)
		}
	}
	return nil
}

// MergeComponentPolicies merges per-component permission policies into a single
// effective policy for a composite query. Denied fields and row filters are
// unioned so that a restriction declared in any component carries through into
// the composite. Allowed models are likewise unioned.
//
// Fail-closed: a nil entry denies the whole composite, because a missing policy
// means the user's access to that component is unknown.
func MergeComponentPolicies(userID, datasourceID string, policies []*PermissionPolicy) (*PermissionPolicy, error) {
	merged := &PermissionPolicy{UserID: userID, DatasourceID: datasourceID}
	deniedSeen := make(map[string]struct{})
	allowedSeen := make(map[string]struct{})

	for _, p := range policies {
		if p == nil {
			return nil, errors.New("missing permission policy for a composite component")
		}
		for _, f := range p.DeniedFields {
			if _, ok := deniedSeen[f]; ok {
				continue
			}
			deniedSeen[f] = struct{}{}
			merged.DeniedFields = append(merged.DeniedFields, f)
		}
		for _, m := range p.AllowedModels {
			if _, ok := allowedSeen[m]; ok {
				continue
			}
			allowedSeen[m] = struct{}{}
			merged.AllowedModels = append(merged.AllowedModels, m)
		}
		for _, rf := range p.RowFilters {
			if !containsRowFilter(merged.RowFilters, rf) {
				merged.RowFilters = append(merged.RowFilters, rf)
			}
		}
	}
	return merged, nil
}

// containsRowFilter reports whether filters already holds an equivalent filter,
// deduplicating identical row filters declared across multiple components.
func containsRowFilter(filters []RowFilter, candidate RowFilter) bool {
	return slices.ContainsFunc(filters, func(f RowFilter) bool {
		return f.Field == candidate.Field &&
			f.Operator == candidate.Operator &&
			rowFilterValueKey(f.Value) == rowFilterValueKey(candidate.Value)
	})
}

func rowFilterValueKey(value any) string {
	valueType := reflect.TypeOf(value)
	typeName := "<nil>"
	if valueType != nil {
		typeName = valueType.String()
	}
	data, err := sonic.ConfigStd.Marshal(value)
	if err != nil {
		return typeName + ":" + fmt.Sprintf("%#v", value)
	}
	return typeName + ":" + string(data)
}
