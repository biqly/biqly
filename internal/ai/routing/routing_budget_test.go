package routing

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestPruneAutoSemanticModel_CountQuestionDropsNumericSums(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: autoModelPrefix + "tweets",
		Dimensions: []semantic.Dimension{
			{Name: "id", ColumnRef: "timeline_tweets.id", Type: "number"},
			{Name: "created_at", ColumnRef: "timeline_tweets.created_at", Type: "date"},
			{Name: "created_at_year", ColumnRef: "timeline_tweets.created_at", Type: "date"},
			{Name: "text", ColumnRef: "timeline_tweets.text", Type: "text"},
			{Name: "noise_1", ColumnRef: "timeline_tweets.noise_1", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: "count"},
			{Name: "sum_retweets", Expression: "timeline_tweets.retweets", Aggregation: "sum"},
			{Name: "avg_retweets", Expression: "timeline_tweets.retweets", Aggregation: "avg"},
			{Name: "min_created_at", Expression: "timeline_tweets.created_at", Aggregation: "min"},
			{Name: "max_created_at", Expression: "timeline_tweets.created_at", Aggregation: "max"},
		},
	}
	limits := Limits{MaxDimensions: 3, MaxMetrics: 4}
	pruneAutoSemanticModel(model, "dün kaç adet tweet atılmıştır?", limits, nil)

	if len(model.Metrics) > 4 {
		t.Fatalf("metrics len = %d, want <= 4", len(model.Metrics))
	}
	if !metricPresent(model.Metrics, "row_count") {
		t.Fatal("expected row_count metric to remain")
	}
	for _, m := range model.Metrics {
		if m.Name == "sum_retweets" || m.Name == "avg_retweets" {
			t.Fatalf("count question should drop sum/avg metrics, got %s", m.Name)
		}
	}
	if len(model.Dimensions) > 3 {
		t.Fatalf("dimensions len = %d, want <= 3", len(model.Dimensions))
	}
}

func TestRoutingLimitsFromConfig_ZeroUsesDefaults(t *testing.T) {
	limits := LimitsFromConfig(0, 0, 0, 0, true)
	def := DefaultRoutingLimits()
	if limits.MaxDimensions != def.MaxDimensions {
		t.Fatalf("MaxDimensions = %d, want %d", limits.MaxDimensions, def.MaxDimensions)
	}
	if !limits.SlimNumericMetrics {
		t.Fatal("expected SlimNumericMetrics true")
	}
}

func TestScoreDimensionForPrune_EnumLabelMatch(t *testing.T) {
	tokens := map[string]struct{}{"shipped": {}}
	plain := semantic.Dimension{Name: "status", ColumnRef: "orders.status", Type: "number"}
	coded := semantic.Dimension{
		Name: "status", ColumnRef: "orders.status", Type: "number",
		EnumValues: []semantic.EnumMapping{{RawValue: "3", Label: "shipped"}},
	}
	plainScore := scoreDimensionForPrune(plain, tokens, nil)
	codedScore := scoreDimensionForPrune(coded, tokens, nil)
	if codedScore <= plainScore {
		t.Fatalf("expected enum label match to raise score: plain=%v coded=%v", plainScore, codedScore)
	}
}

func metricPresent(metrics []semantic.Metric, name string) bool {
	for _, m := range metrics {
		if m.Name == name {
			return true
		}
	}
	return false
}
