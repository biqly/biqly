package ambiguity

import (
	"reflect"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

func TestDetectGlossary(t *testing.T) {
	entries := []prompt.GlossaryEntry{
		{
			Term:       "aktif",
			Definition: "Durumu active olan müşteri",
			MapsToType: "filter",
			MapsToName: "status=active",
		},
		{
			Term:       "Aktif",
			Definition: "Son 30 günde sipariş veren müşteri",
			MapsToType: "filter",
			MapsToName: "last_order_date>=now()-30d",
		},
		{
			Term:       "ciro",
			Definition: "Toplam satış tutarı",
			MapsToType: "metric",
			MapsToName: "revenue",
		},
	}

	got := DetectGlossary("Aktif müşterileri göster", entries, nil)
	want := []Item{
		{
			Term: "aktif",
			Type: "semantic",
			Interpretations: []Interpretation{
				{
					Label:       "Durumu active olan müşteri",
					Description: "Durumu active olan müşteri",
					SemanticMapping: SemanticMapping{
						Type: "filter",
						Name: "status=active",
					},
					Confidence: 1,
				},
				{
					Label:       "Son 30 günde sipariş veren müşteri",
					Description: "Son 30 günde sipariş veren müşteri",
					SemanticMapping: SemanticMapping{
						Type: "filter",
						Name: "last_order_date>=now()-30d",
					},
					Confidence: 1,
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("DetectGlossary() = %#v, want %#v", got, want)
	}
}

func TestDetectGlossary_NoQuestionMatch(t *testing.T) {
	entries := []prompt.GlossaryEntry{
		{Term: "aktif", MapsToType: "filter", MapsToName: "status=active"},
		{Term: "aktif", MapsToType: "filter", MapsToName: "last_order_date>=now()-30d"},
	}

	got := DetectGlossary("Ülke bazında ciroyu göster", entries, nil)
	if len(got) != 0 {
		t.Errorf("DetectGlossary() = %#v, want no ambiguities", got)
	}
}

func TestDetectGlossary_EmptyGlossary(t *testing.T) {
	got := DetectGlossary("Aktif müşterileri göster", nil, nil)
	if len(got) != 0 {
		t.Errorf("DetectGlossary() = %#v, want no ambiguities", got)
	}
}

func TestDetectGlossary_AIContextSynonymCollision(t *testing.T) {
	entries := prompt.GlossaryFromExternal([]prompt.ExternalGlossaryInput{
		{
			Term:       "ciro",
			Definition: "net sales",
			MapsToType: "metric",
			MapsToName: "revenue",
			AIContext:  &pkgmetadata.GlossaryAIContext{Synonyms: []string{"gelir"}},
		},
		{
			Term:       "gelir",
			Definition: "gross sales",
			MapsToType: "metric",
			MapsToName: "gross_revenue",
		},
	})

	got := DetectGlossary("gelir ne kadar", entries, nil)
	if len(got) != 1 {
		t.Fatalf("DetectGlossary() len = %d, want 1 ambiguity", len(got))
	}
	if got[0].Term != "gelir" {
		t.Errorf("term = %q, want gelir", got[0].Term)
	}
	if len(got[0].Interpretations) != 2 {
		t.Fatalf("interpretations = %d, want 2", len(got[0].Interpretations))
	}
}

// TestDetectGlossary_ModelEntryNotAmbiguous guards against a false ambiguity
// when an entity word (e.g. "tweet") maps both to the model itself and to a
// single field synonym. The model entry must not count as an interpretation.
func TestDetectGlossary_ModelEntryNotAmbiguous(t *testing.T) {
	entries := []prompt.GlossaryEntry{
		{
			Term:       "tweet",
			Definition: "model label",
			MapsToType: "model",
			MapsToName: "zlitter_2",
		},
		{
			Term:       "tweet",
			Definition: "dimension (text) → timeline_tweets.id",
			MapsToType: "dimension",
			MapsToName: "tweet_id",
		},
	}

	got := DetectGlossary("dün toplam kaç tweet atılmıştır?", entries, nil)
	if len(got) != 0 {
		t.Errorf("DetectGlossary() = %#v, want no ambiguities", got)
	}
}
