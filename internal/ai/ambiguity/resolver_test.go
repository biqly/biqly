package ambiguity

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestResolveChoiceRewritesQuestion(t *testing.T) {
	result := AmbiguityResult{
		IsAmbiguous: true,
		Ambiguities: []AmbiguityItem{
			{
				Term: "aktif",
				Type: "semantic",
				Interpretations: []Interpretation{
					{Label: "son 30 günde sipariş veren"},
				},
			},
		},
	}

	got, err := ResolveChoice("Aktif müşterileri göster", "ambiguity:0:0", result)
	if err != nil {
		t.Fatalf("ResolveChoice() error = %v, want nil", err)
	}
	want := "son 30 günde sipariş veren müşterileri göster"
	if got != want {
		t.Errorf("ResolveChoice() = %q, want %q", got, want)
	}
}

func TestResolveAnalyzesAndRewritesQuestion(t *testing.T) {
	model := &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Synonyms: []string{"ciro"}},
			{Name: "net_revenue", Synonyms: []string{"ciro"}},
		},
	}

	got, err := Resolve(context.Background(), "Ciro göster", "ambiguity:0:1", model, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	want := "net_revenue göster"
	if got != want {
		t.Errorf("Resolve() = %q, want %q", got, want)
	}
}
