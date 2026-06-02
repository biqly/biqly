package ambiguity

import (
	"reflect"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestDetectSynonyms(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"CIRO"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}

	got := DetectSynonyms("Ciro göster", model)
	want := []AmbiguityItem{
		{
			Term: "ciro",
			Type: "semantic",
			Interpretations: []Interpretation{
				{
					Label:       "customer_segment",
					Description: "dimension customer_segment",
					SemanticMapping: SemanticMapping{
						Type: "dimension",
						Name: "customer_segment",
					},
					Confidence: 1,
				},
				{
					Label:       "revenue",
					Description: "metric revenue",
					SemanticMapping: SemanticMapping{
						Type: "metric",
						Name: "revenue",
					},
					Confidence: 1,
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectSynonyms() = %#v, want %#v", got, want)
	}
}

func TestDetectSynonyms_FuzzyQuestionMatch(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"ciro"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}

	got := DetectSynonyms("Cirp göster", model)
	want := []AmbiguityItem{
		{
			Term: "ciro",
			Type: "semantic",
			Interpretations: []Interpretation{
				{
					Label:       "customer_segment",
					Description: "dimension customer_segment",
					SemanticMapping: SemanticMapping{
						Type: "dimension",
						Name: "customer_segment",
					},
					Confidence: 0.75,
				},
				{
					Label:       "revenue",
					Description: "metric revenue",
					SemanticMapping: SemanticMapping{
						Type: "metric",
						Name: "revenue",
					},
					Confidence: 0.75,
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectSynonyms() = %#v, want %#v", got, want)
	}
}
