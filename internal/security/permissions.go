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
//
// Policy semantics are FAIL-CLOSED:
//   - A nil *PermissionPolicy denies all access. Callers that legitimately
//     need unrestricted access (admin tooling, internal jobs) must construct
//     an explicit policy or use SystemPolicy() below.
//   - An empty AllowedModels list still means "no restriction" — once a
//     policy exists, it is presumed to have been intentionally constructed.
type PermissionManager struct{}

// NewPermissionManager creates a new permission manager.
func NewPermissionManager() *PermissionManager {
	return &PermissionManager{}
}

// SystemPolicy returns a permissive policy intended for trusted internal
// callers (admin endpoints, background workers). Use it explicitly rather
// than passing nil so the call site is auditable.
func SystemPolicy() *PermissionPolicy {
	return &PermissionPolicy{UserID: "system"}
}

// CheckModelAccess verifies the user can access the given model.
//
// Returns an error when policy is nil (fail-closed) or when an explicit
// AllowedModels list is set and does not contain modelName.
func (pm *PermissionManager) CheckModelAccess(policy *PermissionPolicy, modelName string) error {
	if policy == nil {
		return fmt.Errorf("no permission policy supplied for model %s", modelName)
	}
	if len(policy.AllowedModels) == 0 {
		return nil // empty list inside an explicit policy = no restriction
	}
	if !slices.Contains(policy.AllowedModels, modelName) {
		return fmt.Errorf("user %s does not have access to model %s", policy.UserID, modelName)
	}
	return nil
}

// FieldIsDenied reports whether qualifiedField or plainField is listed in policy.DeniedFields.
//
// A nil policy is treated as fail-closed: every field is considered denied.
// Callers wanting to skip the check entirely should pass SystemPolicy().
func FieldIsDenied(policy *PermissionPolicy, qualifiedField, plainField string) bool {
	if policy == nil {
		return true
	}
	if len(policy.DeniedFields) == 0 {
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
// A nil policy returns nil — the executor must not run without a policy.
func (pm *PermissionManager) GetRowFilters(policy *PermissionPolicy) []RowFilter {
	if policy == nil {
		return nil
	}
	return policy.RowFilters
}

// HasFieldAccess checks if a user can access a specific field.
// Fail-closed: nil policy denies.
func (pm *PermissionManager) HasFieldAccess(policy *PermissionPolicy, modelName, fieldName string) bool {
	if policy == nil {
		return false
	}
	return !FieldIsDenied(policy, modelName+"."+fieldName, fieldName)
}

// GetPIIPolicy returns the explicit per-column PII access overrides for a
// user, keyed by qualified column name. A nil policy returns nil; role
// defaults are resolved downstream (fail-closed).
func (pm *PermissionManager) GetPIIPolicy(policy *PermissionPolicy) map[string]string {
	if policy == nil {
		return nil
	}
	return policy.PIIPolicy
}

// PIIFieldIsHidden reports whether qualifiedField or plainField is explicitly
// hidden by the user's PII policy. Unrecognized access values fail closed to
// hidden; absent entries are not hidden here (role defaults apply later).
func PIIFieldIsHidden(policy *PermissionPolicy, qualifiedField, plainField string) bool {
	if policy == nil {
		return true
	}
	for _, key := range []string{qualifiedField, plainField} {
		if access, ok := policy.PIIPolicy[key]; ok && access != "raw" && access != "masked" {
			return true
		}
	}
	return false
}
