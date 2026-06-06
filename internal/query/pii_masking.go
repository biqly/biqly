package query

import (
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/biqly/biqly/internal/semantic"
)

// PIIMaskingConfig drives column-level PII masking during SQL compilation.
// A nil config disables masking entirely (backward compatible).
type PIIMaskingConfig struct {
	// ColumnInfo maps a column reference to its resolved PII policy metadata.
	// Prefer this over the legacy split maps below so each lookup touches one
	// map instead of three.
	ColumnInfo map[string]PIIColumnInfo
	// ColumnAccess maps a column reference to "raw" | "masked" | "hidden".
	// Keys may use the semantic ColumnRef form ("customers.email") or the
	// fully qualified physical form ("public.customers.email").
	ColumnAccess map[string]string
	// ColumnTypes maps the same keys to the column's PII type so the masking
	// expression can be selected. A masked column with an unknown type fails
	// closed to a full mask.
	ColumnTypes map[string]string
	// ColumnStrategies maps the same keys to the column's PII masking strategy override ("partial" | "full").
	ColumnStrategies map[string]string
	// Strategy generates masking SQL. Nil uses pii.DefaultMaskingStrategy.
	Strategy pii.MaskingStrategy
}

// PIIColumnInfo is the resolved PII policy metadata for one column reference.
type PIIColumnInfo struct {
	Access   string
	PIIType  string
	Strategy string
}

func (cfg *PIIMaskingConfig) strategy() pii.MaskingStrategy {
	if cfg.Strategy != nil {
		return cfg.Strategy
	}
	return pii.DefaultMaskingStrategy{}
}

// lookup resolves the access level and PII type for the first matching
// column reference.
func (cfg *PIIMaskingConfig) lookup(refs ...string) (access, piiType string, ok bool) {
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		if info, found := cfg.ColumnInfo[ref]; found {
			return pii.EffectiveColumnAccess(info.Access, info.Strategy), info.PIIType, true
		}
		if a, found := cfg.ColumnAccess[ref]; found {
			return pii.EffectiveColumnAccess(a, cfg.lookupStrategy(refs...)), cfg.ColumnTypes[ref], true
		}
	}
	return "", "", false
}

// lookupStrategy resolves the masking strategy override for the first matching
// column reference.
func (cfg *PIIMaskingConfig) lookupStrategy(refs ...string) string {
	if cfg.ColumnStrategies == nil {
		return ""
	}
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		if s, found := cfg.ColumnStrategies[ref]; found {
			return s
		}
	}
	return ""
}

// piiAccessForDim returns the PII access level and type for a dimension's
// underlying column, matching both the semantic and physical references.
func (c *Compiler) piiAccessForDim(dim *semantic.Dimension, resolver *SchemaResolver) (access, piiType string, ok bool) {
	if c.pii == nil || dim == nil || dim.ColumnRef == "" {
		return "", "", false
	}
	return c.pii.lookup(dim.ColumnRef, resolver.PhysicalColumnRef(dim.ColumnRef))
}

// dimensionOutputSQL renders a dimension for SELECT/GROUP BY/ORDER BY with
// the PII policy applied: hidden columns become a constant mask, masked
// columns are wrapped with the masking expression, raw columns pass through.
// Unrecognized access values fail closed to hidden.
func (c *Compiler) dimensionOutputSQL(dim *semantic.Dimension, resolver *SchemaResolver) string {
	access, piiType, found := c.piiAccessForDim(dim, resolver)
	if !found || access == pii.AccessRaw {
		sql, err := c.dimensionSQL(dim, resolver)
		if err != nil {
			c.err = err
			return ""
		}
		return sql
	}
	if access == pii.AccessMasked {
		colRef := c.dialect.QuoteIdent(resolver.PhysicalColumnRef(dim.ColumnRef))
		return c.pii.strategy().MaskExpression(colRef, piiType, c.dialect)
	}
	return pii.HiddenLiteral
}

// dimensionFullyHidden reports whether dim is hidden for this user after
// access policy and masking strategy are resolved.
func (c *Compiler) dimensionFullyHidden(dim *semantic.Dimension, resolver *SchemaResolver) bool {
	access, _, found := c.piiAccessForDim(dim, resolver)
	return found && access != pii.AccessRaw && access != pii.AccessMasked
}

// filterFieldHidden reports whether a filter on dim must be rejected because
// the column is hidden for this user. Masked columns may still be filtered
// (the predicate runs server-side; values are never projected raw).
func (c *Compiler) filterFieldHidden(dim *semantic.Dimension, resolver *SchemaResolver) bool {
	return c.dimensionFullyHidden(dim, resolver)
}
