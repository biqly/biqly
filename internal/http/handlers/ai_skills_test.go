package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/query"
)

func skillTemplateLQ() query.LogicalQuery {
	return query.LogicalQuery{
		Select: []query.SelectItem{{Type: "metric", Name: "revenue"}},
		Filters: []query.Filter{
			{Field: "country", Operator: "eq", Value: "{{country}}"},
			{Field: "status", Operator: "in", Value: []any{"{{status}}", "active"}},
		},
		Having: []query.Filter{
			{Field: "revenue", Operator: "gt", Value: "{{min_revenue}}"},
		},
		Limit: 100,
	}
}

func TestApplySkillParameters_SubstitutesProvidedAndDefaults(t *testing.T) {
	lq := skillTemplateLQ()
	defs := []SkillParameter{
		{Name: "country", Required: true},
		{Name: "status", Default: "pending"},
		{Name: "min_revenue", Default: float64(1000)},
	}
	err := applySkillParameters(&lq, defs, map[string]any{"country": "TR"})
	if err != nil {
		t.Fatalf("applySkillParameters: %v", err)
	}
	if lq.Filters[0].Value != "TR" {
		t.Errorf("country = %v, want TR", lq.Filters[0].Value)
	}
	arr, ok := lq.Filters[1].Value.([]any)
	if !ok || arr[0] != "pending" || arr[1] != "active" {
		t.Errorf("status = %v, want [pending active]", lq.Filters[1].Value)
	}
	if lq.Having[0].Value != float64(1000) {
		t.Errorf("min_revenue = %v, want 1000", lq.Having[0].Value)
	}
}

func TestApplySkillParameters_MissingRequired(t *testing.T) {
	lq := skillTemplateLQ()
	defs := []SkillParameter{{Name: "country", Required: true}}
	if err := applySkillParameters(&lq, defs, nil); err == nil {
		t.Fatal("expected error for missing required parameter")
	}
}

func TestApplySkillParameters_UnresolvedPlaceholder(t *testing.T) {
	lq := skillTemplateLQ()
	defs := []SkillParameter{
		{Name: "country", Default: "TR"},
		{Name: "status", Default: "active"},
	}
	if err := applySkillParameters(&lq, defs, nil); err == nil {
		t.Fatal("expected error for unresolved {{min_revenue}} placeholder")
	}
}

func TestApplySkillParameters_NonPlaceholderValuesUntouched(t *testing.T) {
	lq := query.LogicalQuery{
		Filters: []query.Filter{
			{Field: "amount", Operator: "gt", Value: float64(50)},
			{Field: "note", Operator: "contains", Value: "{{"},
		},
	}
	if err := applySkillParameters(&lq, nil, nil); err != nil {
		t.Fatalf("applySkillParameters: %v", err)
	}
	if lq.Filters[0].Value != float64(50) || lq.Filters[1].Value != "{{" {
		t.Errorf("values changed: %v, %v", lq.Filters[0].Value, lq.Filters[1].Value)
	}
}

func TestPlaceholderName(t *testing.T) {
	cases := []struct {
		in     string
		name   string
		isName bool
	}{
		{"{{country}}", "country", true},
		{"  {{ country }}  ", "country", true},
		{"{{}}", "", false},
		{"plain", "", false},
		{"{{open", "", false},
	}
	for _, tc := range cases {
		name, ok := placeholderName(tc.in)
		if name != tc.name || ok != tc.isName {
			t.Errorf("placeholderName(%q) = (%q, %v), want (%q, %v)", tc.in, name, ok, tc.name, tc.isName)
		}
	}
}
