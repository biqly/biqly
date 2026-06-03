package pii

import (
	"testing"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/stretchr/testify/assert"
)

func TestDefaultPIIPolicy(t *testing.T) {
	// Admin: raw everything.
	for _, typ := range AllTypes {
		assert.Equal(t, AccessRaw, DefaultPIIPolicy(RoleAdmin, typ))
		assert.Equal(t, AccessRaw, DefaultPIIPolicy("super_admin", typ))
	}
	// Analyst: masked everything.
	for _, typ := range AllTypes {
		assert.Equal(t, AccessMasked, DefaultPIIPolicy(RoleAnalyst, typ))
	}
	// Viewer: hidden for sensitive types, masked otherwise.
	assert.Equal(t, AccessHidden, DefaultPIIPolicy(RoleViewer, TypeTCKimlikNo))
	assert.Equal(t, AccessHidden, DefaultPIIPolicy(RoleViewer, TypeCreditCardLike))
	assert.Equal(t, AccessHidden, DefaultPIIPolicy(RoleViewer, TypeIBAN))
	assert.Equal(t, AccessMasked, DefaultPIIPolicy(RoleViewer, TypeEmail))
	assert.Equal(t, AccessMasked, DefaultPIIPolicy(RoleViewer, TypePhone))
	// Unknown role fails closed to viewer behavior.
	assert.Equal(t, AccessHidden, DefaultPIIPolicy("intern", TypeIBAN))
	assert.Equal(t, AccessMasked, DefaultPIIPolicy("", TypeEmail))
	// Case-insensitive.
	assert.Equal(t, AccessRaw, DefaultPIIPolicy("Admin", TypeEmail))
}

func TestResolveColumnAccess(t *testing.T) {
	// Explicit override wins over role default.
	assert.Equal(t, AccessRaw, ResolveColumnAccess(RoleViewer, TypeEmail, AccessRaw))
	assert.Equal(t, AccessHidden, ResolveColumnAccess(RoleAdmin, TypeEmail, AccessHidden))
	// Empty override falls back to role default.
	assert.Equal(t, AccessMasked, ResolveColumnAccess(RoleAnalyst, TypeEmail, ""))
	// Invalid override fails closed.
	assert.Equal(t, AccessHidden, ResolveColumnAccess(RoleAdmin, TypeEmail, "visible"))
}

func TestPrimaryRole(t *testing.T) {
	assert.Equal(t, RoleAdmin, PrimaryRole([]string{"viewer", "admin"}))
	assert.Equal(t, RoleAnalyst, PrimaryRole([]string{"analyst", "viewer"}))
	assert.Equal(t, RoleViewer, PrimaryRole([]string{"viewer"}))
	assert.Equal(t, "", PrimaryRole([]string{"custom_role"}))
	assert.Equal(t, "", PrimaryRole(nil))
	assert.Equal(t, RoleAdmin, PrimaryRole([]string{"Admin"}))
}

func TestBuildColumnAccessMaps(t *testing.T) {
	email := TypeEmail
	tckn := TypeTCKimlikNo
	cols := []pkgmetadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &email},
		{SchemaName: "public", TableName: "customers", ColumnName: "tckn", PIIType: &tckn},
		{SchemaName: "public", TableName: "customers", ColumnName: "name"}, // not PII
	}

	access, types := BuildColumnAccessMaps(RoleViewer, cols, map[string]string{
		"public.customers.email": AccessRaw, // explicit override
	})

	// Both key forms registered.
	assert.Equal(t, AccessRaw, access["public.customers.email"])
	assert.Equal(t, AccessRaw, access["customers.email"])
	assert.Equal(t, TypeEmail, types["customers.email"])

	// Role default applies without override: viewer hides TCKN.
	assert.Equal(t, AccessHidden, access["customers.tckn"])
	assert.Equal(t, TypeTCKimlikNo, types["public.customers.tckn"])

	// Non-PII columns excluded.
	assert.NotContains(t, access, "customers.name")
	assert.Len(t, access, 4) // 2 PII columns x 2 key forms
}

func TestBuildColumnAccessMaps_ShortKeyOverride(t *testing.T) {
	email := TypeEmail
	cols := []pkgmetadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &email},
	}
	access, _ := BuildColumnAccessMaps(RoleAdmin, cols, map[string]string{
		"customers.email": AccessHidden, // short-form override
	})
	assert.Equal(t, AccessHidden, access["public.customers.email"])
	assert.Equal(t, AccessHidden, access["customers.email"])
}

func TestBuildColumnMaskingStrategyMaps(t *testing.T) {
	email := TypeEmail
	full := MaskingStrategyFull
	invalid := "surprise"
	cols := []pkgmetadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &email, PIIMaskingStrategy: &full},
		{SchemaName: "public", TableName: "customers", ColumnName: "name", PIIMaskingStrategy: &full},
		{SchemaName: "public", TableName: "customers", ColumnName: "notes", PIIType: &email, PIIMaskingStrategy: &invalid},
	}

	strategies := BuildColumnMaskingStrategyMaps(cols)

	assert.Equal(t, MaskingStrategyFull, strategies["public.customers.email"])
	assert.Equal(t, MaskingStrategyFull, strategies["customers.email"])
	assert.NotContains(t, strategies, "customers.name")
	assert.Equal(t, MaskingStrategyFull, strategies["customers.notes"])
}
