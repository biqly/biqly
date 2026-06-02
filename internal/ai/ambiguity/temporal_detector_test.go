package ambiguity

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestDetectTemporal_VaguePhrase(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
	}

	got := DetectTemporal("Geçen ay satışlar", model)
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
	got := DetectTemporal("Son 30 günde sipariş veren müşteriler", model)
	if len(got) != 0 {
		t.Fatalf("DetectTemporal() = %#v, want no ambiguities", got)
	}
}
