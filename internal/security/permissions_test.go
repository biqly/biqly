package security

import "testing"

func TestPermissionManager_NilPolicyFailsClosed(t *testing.T) {
	pm := NewPermissionManager()

	if err := pm.CheckModelAccess(nil, "orders"); err == nil {
		t.Fatal("CheckModelAccess with nil policy must return error")
	}
	if pm.HasFieldAccess(nil, "orders", "amount") {
		t.Fatal("HasFieldAccess with nil policy must return false")
	}
	if FieldIsDenied(nil, "orders.amount", "amount") != true {
		t.Fatal("FieldIsDenied with nil policy must return true")
	}
	if got := pm.RowFilters(nil); got != nil {
		t.Fatalf("RowFilters with nil policy must return nil, got %v", got)
	}
}

func TestPermissionManager_SystemPolicyGrantsAccess(t *testing.T) {
	pm := NewPermissionManager()
	sys := SystemPolicy()
	if err := pm.CheckModelAccess(sys, "orders"); err != nil {
		t.Fatalf("SystemPolicy should permit any model, got %v", err)
	}
	if !pm.HasFieldAccess(sys, "orders", "amount") {
		t.Fatal("SystemPolicy should grant field access")
	}
}

func TestPermissionManager_ExplicitPolicyBehavior(t *testing.T) {
	pm := NewPermissionManager()
	pol := &PermissionPolicy{
		UserID:        "u1",
		AllowedModels: []string{"orders"},
		DeniedFields:  []string{"orders.customer_email"},
	}
	if err := pm.CheckModelAccess(pol, "orders"); err != nil {
		t.Fatalf("expected access to orders, got %v", err)
	}
	if err := pm.CheckModelAccess(pol, "payroll"); err == nil {
		t.Fatal("expected payroll access to be denied")
	}
	if pm.HasFieldAccess(pol, "orders", "customer_email") {
		t.Fatal("denied field must report no access")
	}
	if !pm.HasFieldAccess(pol, "orders", "amount") {
		t.Fatal("non-denied field must be accessible")
	}
}

func TestPermissionInjector_CheckFieldAccess_NilPolicyFailsClosed(t *testing.T) {
	pi := NewPermissionInjector()
	if err := pi.CheckFieldAccess(nil, "orders", []string{"amount"}, nil); err == nil {
		t.Fatal("expected nil policy to be rejected")
	}
}
