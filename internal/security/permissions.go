// Package security provides row-level security, permission injection, and read-only query validation.
package security

import (
	"fmt"
	"slices"
)

// PermissionPolicy defines what a user can access.
type PermissionPolicy struct {
	UserID        string      `json:"user_id"`
	DatasourceID  string      `json:"datasource_id"`
	AllowedModels []string    `json:"allowed_models,omitempty"`
	DeniedFields  []string    `json:"denied_fields,omitempty"`
	RowFilters    []RowFilter `json:"row_filters,omitempty"`
}

// RowFilter defines a mandatory filter to inject into queries.
type RowFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

// PermissionManager enforces access control.
type PermissionManager struct{}

// NewPermissionManager creates a new permission manager.
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

// CheckModelAccess verifies the user can access the given model.
func (pm *PermissionManager) CheckModelAccess(policy *PermissionPolicy, modelName string) error {
	if len(policy.AllowedModels) == 0 {
		return nil // No restrictions
	}

	if !slices.Contains(policy.AllowedModels, modelName) {
		return fmt.Errorf("user %s does not have access to model %s", policy.UserID, modelName)
	}

	return nil
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
	return policy.RowFilters
}

// HasFieldAccess checks if a user can access a specific field.
func (pm *PermissionManager) HasFieldAccess(policy *PermissionPolicy, modelName, fieldName string) bool {
	qualifiedField := modelName + "." + fieldName

	if slices.Contains(policy.DeniedFields, qualifiedField) || slices.Contains(policy.DeniedFields, fieldName) {
		return false
	}

	return true
}
