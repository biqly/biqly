package security

import "testing"

func TestPermissionManager_CheckCompositeAccess_NilFailsClosed(t *testing.T) {
	pm := NewPermissionManager()
	if err := pm.CheckCompositeAccess(nil, []string{"sales"}); err == nil {
		t.Fatal("CheckCompositeAccess with nil policy must deny")
	}
}

func TestPermissionManager_CheckCompositeAccess_RequiresAllComponents(t *testing.T) {
	pm := NewPermissionManager()
	policy := &PermissionPolicy{
		UserID:        "u1",
		AllowedModels: []string{"sales", "customer"},
	}

	if err := pm.CheckCompositeAccess(policy, []string{"sales", "customer"}); err != nil {
		t.Fatalf("expected access granted for allowed components: %v", err)
	}

	if err := pm.CheckCompositeAccess(policy, []string{"sales", "campaign"}); err == nil {
		t.Fatal("expected denial when a component is not allowed")
	}
}

func TestPermissionManager_CheckCompositeAccess_UnrestrictedPolicy(t *testing.T) {
	pm := NewPermissionManager()
	// Empty AllowedModels inside an explicit policy means no restriction.
	policy := &PermissionPolicy{UserID: "u1"}
	if err := pm.CheckCompositeAccess(policy, []string{"sales", "customer", "campaign"}); err != nil {
		t.Fatalf("unrestricted policy should grant composite access: %v", err)
	}
}

func TestMergeComponentPolicies_NilFailsClosed(t *testing.T) {
	policies := []*PermissionPolicy{
		{UserID: "u1", DeniedFields: []string{"salary"}},
		nil,
	}
	if _, err := MergeComponentPolicies("u1", "ds1", policies); err == nil {
		t.Fatal("merge with a nil component policy must deny")
	}
}

func TestMergeComponentPolicies_UnionsDeniedAndFilters(t *testing.T) {
	policies := []*PermissionPolicy{
		{
			UserID:        "u1",
			AllowedModels: []string{"sales"},
			DeniedFields:  []string{"sales.cost"},
			RowFilters:    []RowFilter{{Field: "region", Operator: "eq", Value: "EU"}},
		},
		{
			UserID:        "u1",
			AllowedModels: []string{"customer", "sales"},
			DeniedFields:  []string{"customer.ssn", "sales.cost"},
			RowFilters: []RowFilter{
				{Field: "region", Operator: "eq", Value: "EU"},
				{Field: "tier", Operator: "eq", Value: "gold"},
			},
		},
	}

	merged, err := MergeComponentPolicies("u1", "ds1", policies)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(merged.DeniedFields); got != 2 {
		t.Fatalf("expected 2 unioned denied fields, got %d: %v", got, merged.DeniedFields)
	}
	if got := len(merged.AllowedModels); got != 2 {
		t.Fatalf("expected 2 unioned allowed models, got %d: %v", got, merged.AllowedModels)
	}
	if got := len(merged.RowFilters); got != 2 {
		t.Fatalf("expected 2 deduplicated row filters, got %d: %v", got, merged.RowFilters)
	}
	if merged.DatasourceID != "ds1" {
		t.Fatalf("expected datasource ds1, got %s", merged.DatasourceID)
	}
}
