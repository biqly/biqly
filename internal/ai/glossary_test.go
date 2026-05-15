package ai

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestGlossaryFromSemanticModel(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:     "orders",
		Synonyms: []string{"sipariş"},
		Dimensions: []semantic.Dimension{
			{Name: "country", Synonyms: []string{"ülke"}, Type: "text", ColumnRef: "customers.country"},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Aggregation: "sum", Expression: "orders.amount", Synonyms: []string{"ciro"}},
		},
	}
	entries := GlossaryFromSemanticModel(model)
	if len(entries) < 4 {
		t.Fatalf("expected several glossary entries, got %d", len(entries))
	}
	found := false
	for _, e := range entries {
		if e.Term == "ülke" && e.MapsToName == "country" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ülke → country mapping in glossary")
	}
}

func TestSelectGlossaryForQuestion(t *testing.T) {
	entries := []GlossaryEntry{
		{Term: "ciro", MapsToName: "revenue", MapsToType: "metric", Source: "catalog"},
		{Term: "ülke", MapsToName: "country", MapsToType: "dimension", Source: "catalog"},
		{Term: "müşteri adı", MapsToName: "name", MapsToType: "dimension", Source: "glossary", Definition: "curated"},
	}
	got := SelectGlossaryForQuestion("ülke bazında ciro", entries, nil)
	if len(got) == 0 {
		t.Fatal("expected matches")
	}
	terms := make(map[string]bool)
	for _, e := range got {
		terms[e.Term] = true
	}
	if !terms["ciro"] || !terms["ülke"] {
		t.Errorf("expected ciro and ülke in selection, got %v", got)
	}
}

func TestPromptBuildIncludesBusinessGlossary(t *testing.T) {
	pb := &PromptBuilder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "orders"}
	glossary := []GlossaryEntry{
		{Term: "ciro", MapsToName: "revenue", MapsToType: "metric", Definition: "toplam satış", Source: "glossary"},
	}
	got := pb.Build("q", model, 0, "", nil, nil, nil, nil, glossary)
	if !strings.Contains(got, "## Business Glossary") {
		t.Errorf("expected business glossary section")
	}
	if !strings.Contains(got, "**ciro**") {
		t.Errorf("expected glossary term in prompt")
	}
	if !strings.Contains(got, "[curated]") {
		t.Errorf("expected curated marker for external glossary row")
	}
}

func TestMergeGlossaryEntriesExternalWins(t *testing.T) {
	catalog := []GlossaryEntry{{Term: "ciro", MapsToName: "old_metric", MapsToType: "metric", Source: "catalog"}}
	external := []GlossaryEntry{{Term: "ciro", MapsToName: "revenue", MapsToType: "metric", Source: "glossary"}}
	merged := MergeGlossaryEntries(catalog, external)
	if len(merged) != 1 {
		t.Fatalf("expected one term, got %d", len(merged))
	}
	if merged[0].MapsToName != "revenue" {
		t.Errorf("external should override catalog, got %s", merged[0].MapsToName)
	}
}
