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

func assertDetectSynonyms(t *testing.T, model *semantic.SemanticModel, question string, want []Item) {
	t.Helper()
	got := DetectSynonyms(i18n.LocaleEN, question, model)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectSynonyms() = %#v, want %#v", got, want)
	}
}

func TestDetectSynonyms(t *testing.T) {
	assertDetectSynonyms(t, ciroAmbiguityModel(), "Ciro göster", []Item{
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
	assertDetectSynonyms(t, fuzzyCiroAmbiguityModel(), "Cirp göster", []Item{
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

func genericTokenModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "created_month", Synonyms: []string{"ay"}},
			{Name: "created_day", Synonyms: []string{"day", "days"}},
		},
		Metrics: []semantic.Metric{
			{Name: "deleted_month", Synonyms: []string{"ay"}},
			{Name: "fetched_day", Synonyms: []string{"day", "days"}},
		},
	}
}

// Short, generic synonym tokens that only appear as substrings of unrelated
// words (e.g. "ay" inside "kayıt", "day" inside "today") must not be flagged as
// ambiguous — that was the source of the repeated low-value clarification loop.
func TestDetectSynonyms_GenericSubstringTokensNotFlagged(t *testing.T) {
	assertDetectSynonyms(t, genericTokenModel(), "geçen hafta toplam kaç adet tweet atılmıştır", nil)
	assertDetectSynonyms(t, genericTokenModel(), "kayıt sayısı today göster", nil)
}

func multiWordSynonymModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Synonyms: []string{"sipariş tarihi"}},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Synonyms: []string{"sipariş tarihi"}},
		},
	}
}

// Multi-word synonyms remain matchable as a contiguous phrase.
func TestDetectSynonyms_MultiWordPhraseMatches(t *testing.T) {
	got := DetectSynonyms(i18n.LocaleEN, "sipariş tarihi nedir", multiWordSynonymModel())
	if len(got) != 1 || got[0].Term != "sipariş tarihi" {
		t.Fatalf("expected one ambiguity for multi-word phrase, got %#v", got)
	}
}
