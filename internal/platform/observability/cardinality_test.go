package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCardinalityCollectorExposesSeriesAndLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordAmbiguityAnalysis(5, "rule_based", true)
	m.RecordAmbiguityAnalysis(5, "llm", true)
	m.RecordAIRepair(true, 1, []string{"UNKNOWN_DIMENSION", "SOME_RANDOM_CODE"})

	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}

	series, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	seriesByMetric, _ := countMetricCardinality(series)
	if seriesByMetric["biqly_ambiguity_by_source"] != 2 {
		t.Fatalf("ambiguity series = %d, want 2", seriesByMetric["biqly_ambiguity_by_source"])
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var promSeries float64
	for _, mf := range mfs {
		if mf.GetName() != "bi_prom_metric_series_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if lp.GetName() == "metric" && lp.GetValue() == "biqly_ambiguity_by_source" {
					promSeries = metric.GetGauge().GetValue()
				}
			}
		}
	}
	if promSeries != 2 {
		t.Fatalf("bi_prom_metric_series_total{metric=biqly_ambiguity_by_source} = %v, want 2", promSeries)
	}
}

func TestCheckGatheredCardinalityRejectsForbiddenLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "bi_test_forbidden"}, []string{"user_id"})
	reg.MustRegister(vec)
	vec.WithLabelValues("u-1").Inc()

	if err := CheckGatheredCardinality(reg); err == nil {
		t.Fatal("expected forbidden label error")
	}
}

func TestBoundLabel(t *testing.T) {
	t.Parallel()
	if got := BoundLabel("llm", []string{"rule_based", "llm"}, "other"); got != "llm" {
		t.Fatalf("got %q", got)
	}
	if got := BoundLabel("unknown", []string{"rule_based", "llm"}, "other"); got != "other" {
		t.Fatalf("got %q", got)
	}
}
