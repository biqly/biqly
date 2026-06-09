package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
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
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "orders"}
	glossary := []GlossaryEntry{
		{Term: "ciro", MapsToName: "revenue", MapsToType: "metric", Definition: "toplam satış", Source: "glossary"},
	}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:   i18n.DefaultLocale,
		Glossary: glossary,
	})
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

func TestGlossaryFromExternalAIContextSynonyms(t *testing.T) {
	rows := []ExternalGlossaryInput{{
		Term:       "ciro",
		Definition: "net sales",
		MapsToType: "metric",
		MapsToName: "revenue",
		AIContext: &pkgmetadata.GlossaryAIContext{
			Synonyms:      []string{"gelir", "satış"},
			Unit:          "TRY",
			NullMeaning:   "not invoiced",
			BusinessRules: []string{"exclude cancelled orders"},
		},
	}}
	entries := GlossaryFromExternal(rows)
	terms := make(map[string]GlossaryEntry)
	for _, e := range entries {
		terms[e.Term] = e
	}
	for _, want := range []string{"ciro", "gelir", "satış"} {
		if _, ok := terms[want]; !ok {
			t.Fatalf("expected term %q in glossary entries, got %v", want, entries)
		}
	}
	if terms["ciro"].AIContext == nil || terms["ciro"].AIContext.Unit != "TRY" {
		t.Errorf("primary term should carry ai_context, got %#v", terms["ciro"].AIContext)
	}
	if terms["gelir"].AIContext != nil {
		t.Errorf("synonym-expanded entry should not duplicate ai_context")
	}
}

func TestPromptBuildIncludesGlossaryAIContextHints(t *testing.T) {
	pb := &Builder{}
	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "orders"}
	glossary := []GlossaryEntry{{
		Term:       "ciro",
		MapsToName: "revenue",
		MapsToType: "metric",
		Definition: "net sales",
		Source:     "glossary",
		AIContext: &pkgmetadata.GlossaryAIContext{
			Unit:          "TRY",
			NullMeaning:   "not invoiced",
			BusinessRules: []string{"exclude cancelled orders"},
		},
	}}
	got := pb.Build(context.Background(), "q", model, Config{
		Locale:   i18n.DefaultLocale,
		Glossary: glossary,
	})
	for _, want := range []string{"unit: TRY", "null: not invoiced", "rule: exclude cancelled orders"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected prompt to include %q", want)
		}
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
