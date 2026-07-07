package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubFollowUpRewriteProvider is a minimal providerpkg.Provider stub for
// exercising the AI follow-up rewrite phase without a real LLM call.
type stubFollowUpRewriteProvider struct {
	content string
	err     error
}

func (p *stubFollowUpRewriteProvider) Generate(context.Context, string) (providerpkg.GenerationResult, error) {
	if p.err != nil {
		return providerpkg.GenerationResult{}, p.err
	}
	return providerpkg.GenerationResult{Content: p.content}, nil
}

func (p *stubFollowUpRewriteProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, prompt)
}

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

func TestFollowUpSignalsFromColumnsMetricNameDoesNotFalsePositiveTimeField(t *testing.T) {
	columns := []query.ResultColumn{
		{Name: "region", SemanticType: query.SemanticTypeDimension, Format: query.FormatText},
		{Name: "monthly_recurring_revenue", SemanticType: query.SemanticTypeMetric, Format: query.FormatNumber},
	}

	resultColumns, hasMetric, hasDimension, hasTimeField := followUpSignalsFromColumns(columns)

	assert.Equal(t, []string{"region", "monthly_recurring_revenue"}, resultColumns)
	assert.True(t, hasMetric)
	assert.True(t, hasDimension)
	assert.False(t, hasTimeField, "a metric column whose name contains a time-shaped word must not be treated as a time field")
}

func TestFollowUpSignalsFromColumnsNameFallbackStillDetectsGenuineTimeField(t *testing.T) {
	columns := []query.ResultColumn{
		{Name: "created_at_ts", SemanticType: query.SemanticTypeDimension, Format: query.FormatText},
		{Name: "revenue", SemanticType: query.SemanticTypeMetric, Format: query.FormatNumber},
	}

	_, hasMetric, hasDimension, hasTimeField := followUpSignalsFromColumns(columns)

	assert.True(t, hasMetric)
	assert.True(t, hasDimension)
	assert.True(t, hasTimeField, "a non-metric column whose name matches the time pattern should still be detected via the name fallback")
}

func TestFollowUpSignalsFromColumnsDateFormatDetectsTimeFieldRegardlessOfName(t *testing.T) {
	columns := []query.ResultColumn{
		{Name: "order_date", SemanticType: query.SemanticTypeDimension, Format: query.FormatDate},
	}

	_, _, _, hasTimeField := followUpSignalsFromColumns(columns)

	assert.True(t, hasTimeField)
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

// aiFollowUpsResponseWithSingleMetricRow builds a minimal *ai.Response/
// aiQueryRequest pair whose executed result shape is guaranteed to produce at
// least one deterministic follow-up suggestion (a single-row metric result),
// so attachSuggestedFollowUps has something to hand to the AI rewrite phase.
func aiFollowUpsResponseWithSingleMetricRow() (*ai.Response, aiQueryRequest) {
	resp := &ai.Response{
		Result: &ai.AIResult{
			Result: &query.Result{
				Columns: []query.ResultColumn{{Name: "revenue", SemanticType: query.SemanticTypeMetric}},
				Rows:    [][]any{{100}},
			},
		},
	}
	req := aiQueryRequest{Question: "what is total revenue?"}
	return resp, req
}

// TestAttachSuggestedFollowUpsAIRewriteInvalidJSONFallsBackToDeterministic
// covers the brief's Step 4: a stub provider that returns invalid JSON must
// leave the already-attached deterministic suggestions in place, never an
// empty or partial result.
func TestAttachSuggestedFollowUpsAIRewriteInvalidJSONFallsBackToDeterministic(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.service = ai.NewServiceWithProvider(&config.AIConfig{}, query.NewValidator(1000), &stubFollowUpRewriteProvider{content: "not json at all"})

	resp, req := aiFollowUpsResponseWithSingleMetricRow()
	fc := FollowUpContext{
		UserQuestion:    req.Question,
		AvailableFields: []string{"revenue"},
		ResultColumns:   []string{"revenue"},
		ResultRowCount:  1,
		HasMetric:       true,
	}
	want := BuildDeterministicFollowUps(fc)
	require.NotEmpty(t, want, "test fixture must produce at least one deterministic suggestion")

	h.attachSuggestedFollowUps(context.Background(), resp, req)

	assert.Equal(t, want, resp.Result.SuggestedFollowUps)
}

// TestAttachSuggestedFollowUpsAIRewriteInvalidFieldsFallsBackToDeterministic
// covers the brief's Step 4 for the "invalid fields" half: the AI returns
// well-formed JSON, but every suggestion fails re-validation (references a
// field outside AVAILABLE_FIELDS), so the deterministic fallback must be kept.
func TestAttachSuggestedFollowUpsAIRewriteInvalidFieldsFallsBackToDeterministic(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.service = ai.NewServiceWithProvider(&config.AIConfig{}, query.NewValidator(1000), &stubFollowUpRewriteProvider{
		content: `{"suggestions": [{"id": "bad", "label": "Bad", "question": "Bad question?", "kind": "comparison", "requires": ["not_a_real_field"]}]}`,
	})

	resp, req := aiFollowUpsResponseWithSingleMetricRow()
	fc := FollowUpContext{
		UserQuestion:    req.Question,
		AvailableFields: []string{"revenue"},
		ResultColumns:   []string{"revenue"},
		ResultRowCount:  1,
		HasMetric:       true,
	}
	want := BuildDeterministicFollowUps(fc)
	require.NotEmpty(t, want, "test fixture must produce at least one deterministic suggestion")

	h.attachSuggestedFollowUps(context.Background(), resp, req)

	assert.Equal(t, want, resp.Result.SuggestedFollowUps)
}

// TestAttachSuggestedFollowUpsAIRewriteProviderErrorFallsBackToDeterministic
// covers the LLM-call-failure half of the fallback contract: a provider error
// must not empty out or otherwise disturb the deterministic suggestions.
func TestAttachSuggestedFollowUpsAIRewriteProviderErrorFallsBackToDeterministic(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.service = ai.NewServiceWithProvider(&config.AIConfig{}, query.NewValidator(1000), &stubFollowUpRewriteProvider{err: errors.New("boom")})

	resp, req := aiFollowUpsResponseWithSingleMetricRow()
	fc := FollowUpContext{
		UserQuestion:    req.Question,
		AvailableFields: []string{"revenue"},
		ResultColumns:   []string{"revenue"},
		ResultRowCount:  1,
		HasMetric:       true,
	}
	want := BuildDeterministicFollowUps(fc)
	require.NotEmpty(t, want, "test fixture must produce at least one deterministic suggestion")

	h.attachSuggestedFollowUps(context.Background(), resp, req)

	assert.Equal(t, want, resp.Result.SuggestedFollowUps)
}

// TestAttachSuggestedFollowUpsAIRewriteValidResponseReplacesDeterministic
// confirms the success path: a well-formed, valid AI rewrite does replace the
// deterministic suggestions, so the fallback tests above are meaningfully
// exercising the failure path and not just a no-op AI phase.
func TestAttachSuggestedFollowUpsAIRewriteValidResponseReplacesDeterministic(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.service = ai.NewServiceWithProvider(&config.AIConfig{}, query.NewValidator(1000), &stubFollowUpRewriteProvider{
		content: `{"suggestions": [{"id": "rewritten", "label": "Rewritten", "question": "Rewritten question?", "kind": "comparison", "requires": ["revenue"]}]}`,
	})

	resp, req := aiFollowUpsResponseWithSingleMetricRow()

	h.attachSuggestedFollowUps(context.Background(), resp, req)

	require.Len(t, resp.Result.SuggestedFollowUps, 1)
	assert.Equal(t, "rewritten", resp.Result.SuggestedFollowUps[0].ID)
}
