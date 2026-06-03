package pii

import (
	"strings"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

// Role names recognized by the default PII policy.
const (
	RoleAdmin   = "admin"
	RoleAnalyst = "analyst"
	RoleViewer  = "viewer"
)

// sensitiveTypes are hidden outright (not just masked) for viewers and
// unknown roles.
var sensitiveTypes = map[string]bool{
	TypeTCKimlikNo:     true,
	TypeCreditCardLike: true,
	TypeIBAN:           true,
}

// DefaultPIIPolicy returns the access level a role gets for a PII type when
// no explicit per-column override exists. Unknown roles fail closed to the
// viewer behavior (hidden for sensitive types, masked otherwise).
func DefaultPIIPolicy(role, piiType string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleAdmin, "super_admin":
		return AccessRaw
	case RoleAnalyst:
		return AccessMasked
	default: // viewer and unknown roles
		if sensitiveTypes[piiType] {
			return AccessHidden
		}
		return AccessMasked
	}
}

// ResolveColumnAccess merges the role default with an explicit per-column
// override. Empty override falls back to the role default; invalid override
// values fail closed to hidden.
func ResolveColumnAccess(role, piiType, override string) string {
	switch override {
	case AccessRaw, AccessMasked, AccessHidden:
		return override
	case "":
		return DefaultPIIPolicy(role, piiType)
	default:
		return AccessHidden
	}
}

// PrimaryRole picks the most privileged recognized role from a user's role
// list. Returns "" when none match, which downstream resolves to the
// fail-closed viewer defaults.
func PrimaryRole(roles []string) string {
	best := ""
	rank := func(r string) int {
		switch strings.ToLower(strings.TrimSpace(r)) {
		case RoleAdmin, "super_admin":
			return 3
		case RoleAnalyst:
			return 2
		case RoleViewer:
			return 1
		default:
			return 0
		}
	}
	for _, r := range roles {
		if rank(r) > rank(best) {
			best = strings.ToLower(strings.TrimSpace(r))
		}
	}
	return best
}

// BuildColumnAccessMaps computes the per-column access and PII type maps the
// compiler consumes, merging role defaults with explicit overrides keyed by
// qualified column name. Each column is registered under both its
// "schema.table.column" and "table.column" forms so semantic ColumnRefs
// ("customers.email") and physical refs both resolve.
func BuildColumnAccessMaps(role string, columns []pkgmetadata.Column, overrides map[string]string) (access, types map[string]string) {
	access = make(map[string]string, len(columns)*2)
	types = make(map[string]string, len(columns)*2)
	for _, col := range columns {
		if col.PIIType == nil || *col.PIIType == "" {
			continue
		}
		piiType := *col.PIIType
		qualified := col.SchemaName + "." + col.TableName + "." + col.ColumnName
		short := col.TableName + "." + col.ColumnName

		override := overrides[qualified]
		if override == "" {
			override = overrides[short]
		}
		level := ResolveColumnAccess(role, piiType, override)

		for _, key := range []string{qualified, short} {
			access[key] = level
			types[key] = piiType
		}
	}
	return access, types
}

// BuildColumnMaskingStrategyMaps computes per-column masking strategies for
// PII-annotated columns only. Each column is registered under both physical
// and semantic key forms.
func BuildColumnMaskingStrategyMaps(columns []pkgmetadata.Column) map[string]string {
	strategies := make(map[string]string, len(columns)*2)
	for _, col := range columns {
		if col.PIIType == nil || *col.PIIType == "" || col.PIIMaskingStrategy == nil || *col.PIIMaskingStrategy == "" {
			continue
		}
		strategy := NormalizeMaskingStrategy(*col.PIIMaskingStrategy)
		qualified := col.SchemaName + "." + col.TableName + "." + col.ColumnName
		short := col.TableName + "." + col.ColumnName
		strategies[qualified] = strategy
		strategies[short] = strategy
	}
	return strategies
}
