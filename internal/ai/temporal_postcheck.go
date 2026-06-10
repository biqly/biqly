package ai

import (
	"context"
	"strings"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// maxConfidenceWithoutTemporalFilter caps confidence when the question asked
// for a time window the generated query does not contain. A silently dropped
// time condition must not surface as a high-confidence answer (and the cap
// keeps such responses out of the response cache).
const maxConfidenceWithoutTemporalFilter = 0.5

// applyTemporalFilterPostCheck warns and lowers confidence when the question
// carries a relative time phrase (per the ambiguity temporal detector) but the
// final LogicalQuery has no condition on any date dimension.
func applyTemporalFilterPostCheck(ctx context.Context, question string, model *semantic.SemanticModel, resp *AIResponse) {
	if resp == nil || resp.Result == nil || resp.Result.LogicalQuery == nil || model == nil {
		return
	}
	phrases := ambiguitypkg.MatchTemporalPhrases(question)
	if len(phrases) == 0 {
		return
	}
	if logicalQueryHasDateCondition(resp.Result.LogicalQuery, model) {
		return
	}
	locale := i18n.FromContext(ctx)
	resp.Result.Warnings = append(resp.Result.Warnings, i18n.Tf(locale, "clarification.temporal_filter_missing", map[string]any{
		"Phrase": strings.Join(phrases, ", "),
	}))
	resp.Result.Confidence = min(resp.Result.Confidence, maxConfidenceWithoutTemporalFilter)
}

// logicalQueryHasDateCondition reports whether any WHERE or HAVING filter
// references a date-typed or grain-derived dimension of the model.
func logicalQueryHasDateCondition(lq *query.LogicalQuery, model *semantic.SemanticModel) bool {
	dateDims := make(map[string]struct{}, len(model.Dimensions))
	for _, dimension := range model.Dimensions {
		if dimension.Type == string(semantic.DimensionTypeDate) || dimension.TimeGrain != "" {
			dateDims[dimension.Name] = struct{}{}
		}
	}
	for _, f := range lq.Filters {
		if _, ok := dateDims[f.Field]; ok {
			return true
		}
	}
	for _, f := range lq.Having {
		if _, ok := dateDims[f.Field]; ok {
			return true
		}
	}
	return false
}
