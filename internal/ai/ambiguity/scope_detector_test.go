package ambiguity

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestDetectScope_QualifierWithoutMetric(t *testing.T) {
	model := &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "revenue", Label: strPtr("Gelir")},
			{Name: "order_count", Label: strPtr("Sipariş sayısı")},
		},
	}

	got := DetectScope("Büyük müşterileri göster", model)
	if len(got) != 1 {
		t.Fatalf("DetectScope() len = %d, want 1", len(got))
	}
	if got[0].Type != "scope" || len(got[0].Interpretations) < 2 {
		t.Fatalf("DetectScope()[0] = %#v, want scope with >= 2 interpretations", got[0])
	}
}

func TestDetectScope_SingleMetricMentioned(t *testing.T) {
	model := &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
			{Name: "order_count"},
		},
	}
	got := DetectScope("Yüksek ciro", model)
	if len(got) != 0 {
		t.Fatalf("DetectScope() = %#v, want no ambiguities when one metric is explicit", got)
	}
}

func strPtr(s string) *string { return &s }
