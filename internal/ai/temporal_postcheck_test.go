package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func temporalPostCheckModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "created_at", Type: string(semantic.DimensionTypeDate)},
			{Name: "created_at_month", Type: string(semantic.DimensionTypeDate), TimeGrain: "month"},
			{Name: "username"},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: string(semantic.AggCount)},
		},
	}
}

func temporalPostCheckResponse(filters []query.Filter) *AIResponse {
	return &AIResponse{
		Result: &AIResult{
			LogicalQuery: &query.LogicalQuery{
				Select:  []query.SelectItem{{Type: query.SelectTypeMetric, Name: "row_count"}},
				Filters: filters,
				Limit:   100,
			},
			Confidence: 0.9,
		},
	}
}

func TestApplyTemporalFilterPostCheck_WarnsAndCapsConfidence(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleTR)
	resp := temporalPostCheckResponse(nil)

	applyTemporalFilterPostCheck(ctx, "geçen ay kaç adet tweet atılmıştır", temporalPostCheckModel(), resp)

	if resp.Result.Confidence != maxConfidenceWithoutTemporalFilter {
		t.Fatalf("Confidence = %v, want %v", resp.Result.Confidence, maxConfidenceWithoutTemporalFilter)
	}
	if len(resp.Result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want exactly one temporal warning", resp.Result.Warnings)
	}
	warning := resp.Result.Warnings[0]
	if !strings.Contains(warning, "geçen ay") || !strings.Contains(warning, "zaman koşulu") {
		t.Fatalf("warning = %q, want localized TR message mentioning the phrase", warning)
	}
}

func TestApplyTemporalFilterPostCheck_DateFilterPresent(t *testing.T) {
	for _, field := range []string{"created_at", "created_at_month"} {
		resp := temporalPostCheckResponse([]query.Filter{
			{Field: field, Operator: query.OpBetween, Value: []any{"2026-05-01", "2026-05-31"}},
		})
		applyTemporalFilterPostCheck(context.Background(), "geçen ay kaç adet tweet atılmıştır", temporalPostCheckModel(), resp)
		if len(resp.Result.Warnings) != 0 || resp.Result.Confidence != 0.9 {
			t.Fatalf("filter on %s: response modified: warnings=%#v confidence=%v", field, resp.Result.Warnings, resp.Result.Confidence)
		}
	}
}

func TestApplyTemporalFilterPostCheck_NoTemporalPhrase(t *testing.T) {
	resp := temporalPostCheckResponse(nil)
	applyTemporalFilterPostCheck(context.Background(), "kaç adet tweet atılmıştır", temporalPostCheckModel(), resp)
	if len(resp.Result.Warnings) != 0 || resp.Result.Confidence != 0.9 {
		t.Fatalf("response modified without temporal phrase: warnings=%#v confidence=%v", resp.Result.Warnings, resp.Result.Confidence)
	}
}

func TestApplyTemporalFilterPostCheck_NonDateFilterDoesNotCount(t *testing.T) {
	resp := temporalPostCheckResponse([]query.Filter{
		{Field: "username", Operator: query.OpEq, Value: "zlitter"},
	})
	applyTemporalFilterPostCheck(context.Background(), "last month how many tweets", temporalPostCheckModel(), resp)
	if len(resp.Result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want temporal warning despite non-date filter", resp.Result.Warnings)
	}
	if resp.Result.Confidence != maxConfidenceWithoutTemporalFilter {
		t.Fatalf("Confidence = %v, want %v", resp.Result.Confidence, maxConfidenceWithoutTemporalFilter)
	}
}
