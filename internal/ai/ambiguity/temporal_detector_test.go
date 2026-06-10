package ambiguity

import (
	"slices"
	"testing"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

type stubLexiconStore struct {
	phrases []lexicon.TemporalPhrase
}

func (s stubLexiconStore) TemporalPhrases() []lexicon.TemporalPhrase { return s.phrases }
func (stubLexiconStore) Terms(string, string) []string               { return nil }
func (stubLexiconStore) DomainTerms(string) map[string][]string      { return nil }
func (stubLexiconStore) Invalidate()                                 {}

// A new language's temporal phrases are pure lexicon data: swapping the store
// (what DB rows do in production) makes the detector recognize them without
// any code change (ADR-0001 acceptance).
func TestTemporalPhrasesComeFromLexiconStore(t *testing.T) {
	prev := lexicon.SetActive(stubLexiconStore{phrases: []lexicon.TemporalPhrase{
		{Phrase: "letzten monat", InterpretationKeys: []string{"prev_calendar_month", "rolling_30d"}},
	}})
	defer lexicon.SetActive(prev)

	got := MatchTemporalPhrases("letzten monat wie viele tweets")
	if !slices.Equal(got, []string{"letzten monat"}) {
		t.Fatalf("MatchTemporalPhrases() = %v, want German phrase from store", got)
	}
	if leak := MatchTemporalPhrases("geçen ay kaç tweet"); leak != nil {
		t.Fatalf("default phrases must not leak when the store overrides them, got %v", leak)
	}
}

func TestDetectTemporal_VaguePhrase(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
	}

	got := DetectTemporal(i18n.LocaleTR, "Geçen ay satışlar", model)
	if len(got) != 1 {
		t.Fatalf("DetectTemporal() len = %d, want 1", len(got))
	}
	if got[0].Term != "geçen ay" || got[0].Type != "temporal" {
		t.Fatalf("DetectTemporal()[0] = %#v, want temporal geçen ay", got[0])
	}
	if len(got[0].Interpretations) < 2 {
		t.Fatalf("DetectTemporal() interpretations = %d, want >= 2", len(got[0].Interpretations))
	}
	if got[0].Interpretations[0].SemanticMapping.Name != "order_date" {
		t.Fatalf("SemanticMapping.Name = %q, want order_date", got[0].Interpretations[0].SemanticMapping.Name)
	}
}

func TestDetectTemporal_SpecificPhraseSkipped(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
	}
	got := DetectTemporal(i18n.LocaleTR, "Son 30 günde sipariş veren müşteriler", model)
	if len(got) != 0 {
		t.Fatalf("DetectTemporal() = %#v, want no ambiguities", got)
	}
}

func TestDetectTemporal_EnglishVaguePhrase(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
	}

	got := DetectTemporal(i18n.LocaleEN, "Show customers who ordered recently", model)
	if len(got) != 1 {
		t.Fatalf("DetectTemporal() len = %d, want 1", len(got))
	}
	if got[0].Term != "recently" || got[0].Type != "temporal" {
		t.Fatalf("DetectTemporal()[0] = %#v, want temporal recently", got[0])
	}
	if len(got[0].Interpretations) < 2 {
		t.Fatalf("DetectTemporal() interpretations = %d, want >= 2", len(got[0].Interpretations))
	}
}
