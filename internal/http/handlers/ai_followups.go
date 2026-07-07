package handlers

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
)

// timeFieldNamePattern is a fallback heuristic for recognizing a time-shaped
// result column by name alone, for parity with the frontend's deterministic
// fallback (frontend/src/components/aiQuery/followUpSuggestions.ts) when a
// column's Format metadata does not already mark it as a date/datetime.
var timeFieldNamePattern = regexp.MustCompile(`(?i)date|time|hour|day|month|year|_ts$`)

// FollowUpContext summarizes the shape of an executed AI query result that
// BuildDeterministicFollowUps uses to derive candidate follow-up questions.
// It deliberately carries only aggregate signals (booleans, counts, field
// names) rather than a full query.Result, keeping the builder a pure
// function that is trivial to unit test.
type FollowUpContext struct {
	UserQuestion    string
	PriorQuestions  []string
	AvailableFields []string
	ResultColumns   []string
	ResultRowCount  int
	HasMetric       bool
	HasDimension    bool
	HasTimeField    bool
}

// BuildDeterministicFollowUps derives up to a few candidate next-question
// suggestions from the shape of an executed result — no AI call is involved.
// Candidates are always passed through ai.ValidateSuggestedFollowUps before
// being returned, so callers never need to validate the result again.
func BuildDeterministicFollowUps(fc FollowUpContext) []ai.SuggestedFollowUp {
	var candidates []ai.SuggestedFollowUp

	if fc.HasTimeField && fc.HasMetric {
		candidates = append(candidates,
			ai.SuggestedFollowUp{
				ID:       "trend-over-time",
				Label:    "Show trend over time",
				Question: "Show this as a trend over time",
				Reason:   "This result has a time field and a metric, so it can be plotted over time.",
				Kind:     ai.SuggestedFollowUpTrend,
			},
			ai.SuggestedFollowUp{
				ID:       "visualize-as-chart",
				Label:    "Visualize as a chart",
				Question: "Visualize this result as a chart",
				Reason:   "This result has a time field and a metric, so a chart can highlight the pattern.",
				Kind:     ai.SuggestedFollowUpChart,
			},
		)
	}

	if fc.HasDimension && fc.HasMetric && fc.ResultRowCount > 1 {
		candidates = append(candidates, ai.SuggestedFollowUp{
			ID:       "compare-top-values",
			Label:    "Compare top values",
			Question: "Compare the top values in this result",
			Reason:   "This result has multiple rows across a dimension and a metric to compare.",
			Kind:     ai.SuggestedFollowUpComparison,
		})
	}

	if fc.ResultRowCount == 1 && fc.HasMetric {
		candidates = append(candidates,
			ai.SuggestedFollowUp{
				ID:       "drilldown-detail",
				Label:    "See more detail",
				Question: "Break this result down into more detail",
				Reason:   "This result is a single summary value; a drilldown can reveal what makes it up.",
				Kind:     ai.SuggestedFollowUpDrilldown,
			},
			ai.SuggestedFollowUp{
				ID:       "breakdown-by-dimension",
				Label:    "Break down by category",
				Question: "Break this result down by a category",
				Reason:   "This result is a single summary value that could be split by a dimension.",
				Kind:     ai.SuggestedFollowUpBreakdown,
			},
		)
	}

	return ai.ValidateSuggestedFollowUps(candidates, fc.AvailableFields, fc.PriorQuestions)
}

// attachSuggestedFollowUps derives deterministic next-question chips from the
// shape of the executed result and attaches them to
// resp.Result.SuggestedFollowUps. Best-effort and gated: it is a no-op when
// there is no executed result. Mirrors the guard style of
// attachAINaturalLanguageAnswer; call it only after the executed result
// (resp.Result.Result) has been populated.
//
// It is a method on *AIHandler (rather than a free function) for call-site
// and interface parity with attachAINaturalLanguageAnswer, and so a later AI
// rewrite/selection phase (see ai.RewriteFollowUpsWithAI in the plan) can use
// handler-level dependencies without changing every call site again. It does
// not use handler state today, since Task 2 is deterministic-only.
func (*AIHandler) attachSuggestedFollowUps(ctx context.Context, resp *ai.Response, req aiQueryRequest) {
	if resp == nil || resp.Result == nil || resp.Result.Result == nil {
		return
	}
	result := resp.Result.Result

	fc := FollowUpContext{
		UserQuestion:   req.Question,
		PriorQuestions: priorQuestionsFromTurns(req.PriorTurns),
		ResultRowCount: len(result.Rows),
	}
	fc.ResultColumns = make([]string, 0, len(result.Columns))
	for _, col := range result.Columns {
		fc.ResultColumns = append(fc.ResultColumns, col.Name)
		switch col.SemanticType {
		case query.SemanticTypeMetric:
			fc.HasMetric = true
		case query.SemanticTypeDimension:
			fc.HasDimension = true
		}
		if col.Format == query.FormatDate || col.Format == query.FormatDateTime || timeFieldNamePattern.MatchString(col.Name) {
			fc.HasTimeField = true
		}
	}
	// The fields actually present on this result are the safest set to
	// validate Requires against — anything else is not guaranteed usable by
	// the follow-up the chip would trigger.
	fc.AvailableFields = fc.ResultColumns

	resp.Result.SuggestedFollowUps = BuildDeterministicFollowUps(fc)
	slog.DebugContext(ctx, "attached deterministic follow-up suggestions", "count", len(resp.Result.SuggestedFollowUps))
}

// priorQuestionsFromTurns extracts the question text from each prior
// conversation turn, in order, for follow-up duplicate detection.
func priorQuestionsFromTurns(turns []priorTurnPayload) []string {
	if len(turns) == 0 {
		return nil
	}
	out := make([]string, 0, len(turns))
	for _, t := range turns {
		out = append(out, t.Question)
	}
	return out
}
