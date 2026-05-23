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
		op      string
		val     any
		want    string
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
