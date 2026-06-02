package ambiguity

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

func TestAmbiguityResultJSON(t *testing.T) {
	result := AmbiguityResult{
		IsAmbiguous: true,
		Ambiguities: []AmbiguityItem{
			{
				Term: "aktif müşteri",
				Type: "semantic",
				Interpretations: []Interpretation{
					{
						Label:       "Son 30 günde sipariş veren müşteri",
						Description: "Müşterinin yakın zamanda sipariş vermiş olması",
						SemanticMapping: SemanticMapping{
							Type: "dimension",
							Name: "last_order_date",
						},
						Confidence: 0.92,
					},
				},
			},
		},
		ResolvedQuestion: "Son 30 günde sipariş veren müşterileri göster",
	}

	got, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(%#v) error = %v", result, err)
	}
	want := `{"is_ambiguous":true,"ambiguities":[{"term":"aktif müşteri","type":"semantic","interpretations":[{"label":"Son 30 günde sipariş veren müşteri","description":"Müşterinin yakın zamanda sipariş vermiş olması","semantic_mapping":{"type":"dimension","name":"last_order_date"},"confidence":0.92}]}],"resolved_question":"Son 30 günde sipariş veren müşterileri göster"}`
	if string(got) != want {
		t.Errorf("json.Marshal(%#v) = %s, want %s", result, got, want)
	}
}

func TestAnalyze_MergesRuleBasedAmbiguities(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"ciro"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}
	glossary := []prompt.GlossaryEntry{
		{Term: "ciro", Definition: "Brüt gelir", MapsToType: "metric", MapsToName: "revenue"},
		{Term: "ciro", Definition: "Net gelir", MapsToType: "metric", MapsToName: "net_revenue"},
	}

	got := Analyze(context.Background(), "Ciro göster", model, glossary, 0)
	want := AmbiguityResult{
		IsAmbiguous: true,
		Ambiguities: []AmbiguityItem{
			{
				Term: "ciro",
				Type: "semantic",
				Interpretations: []Interpretation{
					{
						Label:       "Brüt gelir",
						Description: "Brüt gelir",
						SemanticMapping: SemanticMapping{
							Type: "metric",
							Name: "revenue",
						},
						Confidence: 1,
					},
					{
						Label:       "Net gelir",
						Description: "Net gelir",
						SemanticMapping: SemanticMapping{
							Type: "metric",
							Name: "net_revenue",
						},
						Confidence: 1,
					},
					{
						Label:       "customer_segment",
						Description: "dimension customer_segment",
						SemanticMapping: SemanticMapping{
							Type: "dimension",
							Name: "customer_segment",
						},
						Confidence: 1,
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestFilterAmbiguities_RequiresTwoInterpretationsAboveThreshold(t *testing.T) {
	ambiguities := []AmbiguityItem{
		{
			Term: "ciro",
			Type: "semantic",
			Interpretations: []Interpretation{
				{Label: "low confidence", Confidence: 0.69},
				{Label: "high confidence", Confidence: 0.90},
			},
		},
	}

	got := filterAmbiguities(ambiguities, defaultConfidenceThreshold)
	if len(got) != 0 {
		t.Errorf("filterAmbiguities() = %#v, want no ambiguities", got)
	}
}
