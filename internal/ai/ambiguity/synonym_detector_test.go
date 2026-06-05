package ambiguity

import (
	"reflect"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

func ciroAmbiguityModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"CIRO"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}
}

func assertDetectSynonyms(t *testing.T, model *semantic.SemanticModel, question string, want []AmbiguityItem) {
	t.Helper()
	got := DetectSynonyms(i18n.LocaleEN, question, model)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectSynonyms() = %#v, want %#v", got, want)
	}
}

func TestDetectSynonyms(t *testing.T) {
	assertDetectSynonyms(t, ciroAmbiguityModel(), "Ciro göster", []AmbiguityItem{
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
	})
}

func fuzzyCiroAmbiguityModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"ciro"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}
}

func TestDetectSynonyms_FuzzyQuestionMatch(t *testing.T) {
	assertDetectSynonyms(t, fuzzyCiroAmbiguityModel(), "Cirp göster", []AmbiguityItem{
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
	})
}
