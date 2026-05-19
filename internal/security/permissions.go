// Package security provides row-level security, permission injection, and read-only query validation.
package security

//revive:disable:exported // alias shim — canonical docs live in pkg/security

import (
	"fmt"
	"slices"

	pkgsecurity "github.com/biqly/biqly/pkg/security"
)

// PermissionPolicy and RowFilter re-export pkg/security data structures so
// existing callers continue to import the legacy "internal/security" path.
type (
	PermissionPolicy = pkgsecurity.PermissionPolicy
	RowFilter        = pkgsecurity.RowFilter
)

// PermissionManager enforces access control.
type PermissionManager struct{}

// NewPermissionManager creates a new permission manager.
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

// CheckModelAccess verifies the user can access the given model.
func (pm *PermissionManager) CheckModelAccess(policy *PermissionPolicy, modelName string) error {
	if policy == nil {
		return nil
	}
	if len(policy.AllowedModels) == 0 {
		return nil // No restrictions
	}

	if !slices.Contains(policy.AllowedModels, modelName) {
		return fmt.Errorf("user %s does not have access to model %s", policy.UserID, modelName)
	}

	return nil
}

// FieldIsDenied reports whether qualifiedField or plainField is listed in policy.DeniedFields.
func FieldIsDenied(policy *PermissionPolicy, qualifiedField, plainField string) bool {
	if policy == nil || len(policy.DeniedFields) == 0 {
		return false
	}
	return slices.Contains(policy.DeniedFields, qualifiedField) || slices.Contains(policy.DeniedFields, plainField)
}

// FilterAllowedFields removes denied fields from the semantic model.
func (pm *PermissionManager) FilterAllowedFields(modelName string, allowedFields []string, deniedFields []string) []string {
	if len(deniedFields) == 0 {
		return allowedFields
	}

	var result []string
	for _, field := range allowedFields {
		qualifiedField := modelName + "." + field
		if !slices.Contains(deniedFields, qualifiedField) && !slices.Contains(deniedFields, field) {
			result = append(result, field)
		}
	}
	return result
}

// GetRowFilters returns the mandatory row filters for a user.
func (pm *PermissionManager) GetRowFilters(policy *PermissionPolicy) []RowFilter {
	if policy == nil {
		return nil
	}
	return policy.RowFilters
}

// HasFieldAccess checks if a user can access a specific field.
func (pm *PermissionManager) HasFieldAccess(policy *PermissionPolicy, modelName, fieldName string) bool {
	if policy == nil {
		return true
	}
	return !FieldIsDenied(policy, modelName+"."+fieldName, fieldName)
}
