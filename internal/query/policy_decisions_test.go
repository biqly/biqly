package query

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

func policyTestModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Name:       "customers",
		BaseSchema: "public",
		BaseTable:  "customers",
		Dimensions: []semantic.Dimension{
			{Name: "tenant_id", ColumnRef: "customers.tenant_id", Type: "text"},
			{Name: "email", ColumnRef: "customers.email", Type: "text"},
			{Name: "name", ColumnRef: "customers.name", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "customers.id", Aggregation: "count"},
		},
	}
}

func TestCompileWithPermissions_RecordsRowFilterDecisions(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}},
		Limit:   10,
	}
	rowFilters := []security.RowFilter{
		{Field: "tenant_id", Operator: "", Value: "t1"},
		{Field: "not_in_model", Operator: "eq", Value: "x"},
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).
		CompileWithPermissions(context.Background(), &lq, policyTestModel(), rowFilters, nil)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if cq.Policy == nil {
		t.Fatal("expected policy decisions, got nil")
	}
	if len(cq.Policy.RowFilters) != 1 {
		t.Fatalf("expected 1 applied row filter, got %d", len(cq.Policy.RowFilters))
	}
	rf := cq.Policy.RowFilters[0]
	if rf.Field != "tenant_id" || rf.Operator != "eq" || rf.Value != "t1" {
		t.Errorf("unexpected applied row filter: %+v", rf)
	}
}

func TestCompileWithPermissions_RecordsMaskedAndHiddenColumns(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select: []SelectItem{
			{Type: "dimension", Name: "email"},
			{Type: "dimension", Name: "name"},
		},
		Limit: 10,
	}
	cfg := &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"customers.email": "masked",
			"customers.name":  "hidden",
		},
		ColumnTypes: map[string]string{"customers.email": "email"},
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).
		CompileWithPermissions(context.Background(), &lq, policyTestModel(), nil, cfg)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if cq.Policy == nil {
		t.Fatal("expected policy decisions, got nil")
	}
	if len(cq.Policy.MaskedColumns) != 1 || cq.Policy.MaskedColumns[0] != "customers.email" {
		t.Errorf("unexpected masked columns: %v", cq.Policy.MaskedColumns)
	}
	if len(cq.Policy.HiddenColumns) != 1 || cq.Policy.HiddenColumns[0] != "customers.name" {
		t.Errorf("unexpected hidden columns: %v", cq.Policy.HiddenColumns)
	}
}

func TestCompileWithPermissions_NoPolicyYieldsNilDecisions(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}},
		Limit:   10,
	}
	cq, err := NewCompiler(dialect.PostgresDialect{}).
		CompileWithPermissions(context.Background(), &lq, policyTestModel(), nil, nil)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if cq.Policy != nil {
		t.Errorf("expected nil policy decisions, got %+v", cq.Policy)
	}
}
