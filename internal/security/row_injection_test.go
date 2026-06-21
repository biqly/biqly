package security

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
)

func TestBuildRowFilterPredicatesUnknownOperatorFails(t *testing.T) {
	dimMap := map[string]string{"tenant_id": "orders.tenant_id"}
	filters := []RowFilter{{Field: "tenant_id", Operator: "bogus", Value: 1}}

	_, _, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, false)
	if err == nil {
		t.Fatal("expected error for unsupported operator")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("expected error to name the operator, got %v", err)
	}
}

func TestBuildRowFilterPredicatesNeqIsNotEq(t *testing.T) {
	dimMap := map[string]string{"tenant_id": "orders.tenant_id"}
	filters := []RowFilter{{Field: "tenant_id", Operator: "neq", Value: 42}}

	preds, _, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(preds))
	}
	if !strings.Contains(preds[0], "<>") {
		t.Fatalf("expected `<>` operator in predicate, got %q", preds[0])
	}
	if strings.Contains(preds[0], "= $") {
		t.Fatalf("neq must not compile to equality, got %q", preds[0])
	}
}

func TestBuildRowFilterPredicatesAllowedOperators(t *testing.T) {
	dimMap := map[string]string{"x": "t.x"}
	cases := []struct {
		op   string
		val  any
		want string
	}{
		{"eq", 1, "="},
		{"neq", 1, "<>"},
		{"gt", 1, ">"},
		{"gte", 1, ">="},
		{"lt", 1, "<"},
		{"lte", 1, "<="},
		{"in", []any{1, 2}, "IN ("},
		{"not_in", []any{1, 2}, "NOT IN ("},
		{"is_null", nil, "IS NULL"},
		{"is_not_null", nil, "IS NOT NULL"},
	}
	for _, c := range cases {
		filters := []RowFilter{{Field: "x", Operator: c.op, Value: c.val}}
		preds, _, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, false)
		if err != nil {
			t.Errorf("op %q: unexpected error: %v", c.op, err)
			continue
		}
		if len(preds) != 1 || !strings.Contains(preds[0], c.want) {
			t.Errorf("op %q: expected fragment containing %q, got %v", c.op, c.want, preds)
		}
	}
}

func TestBuildRowFilterPredicatesInRequiresArray(t *testing.T) {
	dimMap := map[string]string{"x": "t.x"}
	filters := []RowFilter{{Field: "x", Operator: "in", Value: 42}}

	_, _, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, false)
	if err == nil {
		t.Fatal("expected error when 'in' value is not an array")
	}
}

func TestBuildRowFilterPredicatesUnknownFieldReturnsError(t *testing.T) {
	dimMap := map[string]string{"x": "t.x"}
	filters := []RowFilter{{Field: "not_there", Operator: "eq", Value: 1}}

	_, _, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, false)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestBuildRowFilterPredicatesUnknownFieldOmitted(t *testing.T) {
	dimMap := map[string]string{"x": "t.x"}
	filters := []RowFilter{{Field: "not_there", Operator: "eq", Value: 1}}

	preds, args, err := BuildRowFilterPredicates(dialect.PostgresDialect{}, dimMap, filters, 0, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(preds) != 0 || len(args) != 0 {
		t.Fatalf("expected unknown field to be omitted, got preds=%v args=%v", preds, args)
	}
}

// CheckFieldAccess tests

func TestCheckFieldAccess_AllowsAccess(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID: "u1",
	}
	if err := pi.CheckFieldAccess(pol, "orders", []string{"amount", "date"}, []string{"region"}); err != nil {
		t.Fatalf("expected all fields to be accessible, got %v", err)
	}
}

func TestCheckFieldAccess_DeniedSelectField(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID:       "u1",
		DeniedFields: []string{"orders.salary"},
	}
	err := pi.CheckFieldAccess(pol, "orders", []string{"amount", "salary"}, nil)
	if err == nil {
		t.Fatal("expected error for denied select field")
	}
}

func TestCheckFieldAccess_DeniedFilterField(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID:       "u1",
		DeniedFields: []string{"orders.ssn"},
	}
	err := pi.CheckFieldAccess(pol, "orders", []string{"amount"}, []string{"ssn"})
	if err == nil {
		t.Fatal("expected error for denied filter field")
	}
}

func TestCheckFieldAccess_SelectFieldPIIHidden(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID: "u1",
		PIIPolicy: map[string]string{
			"orders.email": "hidden",
		},
	}
	err := pi.CheckFieldAccess(pol, "orders", []string{"email"}, nil)
	if err == nil {
		t.Fatal("expected error for PII-hidden select field")
	}
}

func TestCheckFieldAccess_FilterFieldPIIHidden(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID: "u1",
		PIIPolicy: map[string]string{
			"orders.email": "hidden",
		},
	}
	err := pi.CheckFieldAccess(pol, "orders", []string{"amount"}, []string{"email"})
	if err == nil {
		t.Fatal("expected error for PII-hidden filter field")
	}
}

func TestCheckFieldAccess_NoFieldsAllows(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID: "u1",
	}
	if err := pi.CheckFieldAccess(pol, "orders", nil, nil); err != nil {
		t.Fatalf("nil/empty fields should not cause error, got %v", err)
	}
}

func TestCheckFieldAccess_SelectFieldsCase(t *testing.T) {
	pi := NewPermissionInjector()
	pol := &PermissionPolicy{
		UserID:       "u1",
		DeniedFields: []string{"orders.salary"},
	}
	// Should fail because "salary" is denied
	err := pi.CheckFieldAccess(pol, "orders", []string{"amount", "salary"}, nil)
	if err == nil {
		t.Fatal("expected error for denied field")
	}
	// Should pass for non-denied fields
	err = pi.CheckFieldAccess(pol, "orders", []string{"amount"}, nil)
	if err != nil {
		t.Fatalf("expected non-denied field to pass, got %v", err)
	}
}
