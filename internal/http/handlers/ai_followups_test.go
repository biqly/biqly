package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministicFollowUpsTimeAndMetricProducesTrendAndChart(t *testing.T) {
	fc := FollowUpContext{
		AvailableFields: []string{"order_date", "revenue"},
		ResultColumns:   []string{"order_date", "revenue"},
		ResultRowCount:  5,
		HasMetric:       true,
		HasTimeField:    true,
	}

	got := BuildDeterministicFollowUps(fc)

	require.NotEmpty(t, got)
	kinds := make([]ai.SuggestedFollowUpKind, 0, len(got))
	for _, s := range got {
		kinds = append(kinds, s.Kind)
	}
	assert.Contains(t, kinds, ai.SuggestedFollowUpTrend)
	assert.Contains(t, kinds, ai.SuggestedFollowUpChart)
}

func TestBuildDeterministicFollowUpsDimensionMetricMultiRowProducesComparison(t *testing.T) {
	fc := FollowUpContext{
		AvailableFields: []string{"region", "revenue"},
		ResultColumns:   []string{"region", "revenue"},
		ResultRowCount:  3,
		HasMetric:       true,
		HasDimension:    true,
	}

	got := BuildDeterministicFollowUps(fc)

	require.Len(t, got, 1)
	assert.Equal(t, ai.SuggestedFollowUpComparison, got[0].Kind)
}

func TestBuildDeterministicFollowUpsSingleRowMetricProducesDrilldownAndBreakdown(t *testing.T) {
	fc := FollowUpContext{
		AvailableFields: []string{"revenue"},
		ResultColumns:   []string{"revenue"},
		ResultRowCount:  1,
		HasMetric:       true,
	}

	got := BuildDeterministicFollowUps(fc)

	require.NotEmpty(t, got)
	kinds := make([]ai.SuggestedFollowUpKind, 0, len(got))
	for _, s := range got {
		kinds = append(kinds, s.Kind)
	}
	assert.Contains(t, kinds, ai.SuggestedFollowUpDrilldown)
	assert.Contains(t, kinds, ai.SuggestedFollowUpBreakdown)
}

func TestBuildDeterministicFollowUpsNoSignalsProducesNoCandidates(t *testing.T) {
	fc := FollowUpContext{
		AvailableFields: []string{"note"},
		ResultColumns:   []string{"note"},
		ResultRowCount:  4,
	}

	got := BuildDeterministicFollowUps(fc)

	assert.Empty(t, got)
}

func TestBuildDeterministicFollowUpsDropsQuestionsMatchingPriorTurns(t *testing.T) {
	fc := FollowUpContext{
		AvailableFields: []string{"revenue"},
		ResultColumns:   []string{"revenue"},
		ResultRowCount:  1,
		HasMetric:       true,
		PriorQuestions:  []string{"Break this result down into more detail"},
	}

	got := BuildDeterministicFollowUps(fc)

	for _, s := range got {
		assert.NotEqual(t, ai.SuggestedFollowUpDrilldown, s.Kind)
	}
}

// TestSuggestedFollowUpsSurviveAIJobResultRoundTrip verifies that
// suggested_followups attached to an ai.Response survive the same
// marshal/unmarshal path used to persist and reload AI job results
// (encodeAIJobResult, internal/http/handlers/ai_job_exec.go), so the chips
// are not silently dropped between a job completing and the client fetching
// its stored result.
func TestSuggestedFollowUpsSurviveAIJobResultRoundTrip(t *testing.T) {
	resp := &ai.Response{
		Result: &ai.AIResult{
			Confidence: 0.9,
			Result: &query.Result{
				Columns: []query.ResultColumn{{Name: "revenue", SemanticType: query.SemanticTypeMetric}},
				Rows:    [][]any{{100}},
			},
			SuggestedFollowUps: []ai.SuggestedFollowUp{
				{
					ID:       "drilldown-detail",
					Label:    "See more detail",
					Question: "Break this result down into more detail",
					Reason:   "This result is a single summary value; a drilldown can reveal what makes it up.",
					Kind:     ai.SuggestedFollowUpDrilldown,
				},
			},
		},
	}

	encoded, err := encodeAIJobResult(resp)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "suggested_followups")

	var reloaded ai.Response
	require.NoError(t, sonic.ConfigStd.Unmarshal(encoded, &reloaded))

	require.NotNil(t, reloaded.Result)
	require.Len(t, reloaded.Result.SuggestedFollowUps, 1)
	assert.Equal(t, resp.Result.SuggestedFollowUps[0], reloaded.Result.SuggestedFollowUps[0])
}
