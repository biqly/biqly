package handlers

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/query"
)

// goldenResultColumns is a two-column (time dimension + metric) result shape
// shared by the golden tests below, chosen specifically so
// attachSuggestedFollowUps' deterministic heuristics (HasTimeField &&
// HasMetric, HasDimension && HasMetric && RowCount>1) both fire, giving a
// deterministic, non-empty SuggestedFollowUps list to assert on.
func goldenResult() *query.Result {
	return &query.Result{
		Columns: []query.ResultColumn{
			{Name: "month", Type: "string", SemanticType: query.SemanticTypeDimension, Format: query.FormatDate},
			{Name: "revenue", Type: "number", SemanticType: query.SemanticTypeMetric, Format: query.FormatCurrency},
		},
		Rows: [][]any{
			{"2026-01", 100.0},
			{"2026-02", 200.0},
		},
		Stats:            query.Stats{DurationMs: 12, RowCount: 2},
		ChartSuggestions: []string{query.ChartLine, query.ChartBar},
	}
}

func goldenLogicalQuery() *query.LogicalQuery {
	return &query.LogicalQuery{
		DatasourceID: "ds-1",
		ModelID:      "model-1",
		Select: []query.SelectItem{
			{Type: "dimension", Name: "month"},
			{Type: "metric", Name: "revenue"},
		},
		Limit: 100,
	}
}

// TestComposeWebAgentFinalResultRunQuestionGolden proves the T7 "Done when"
// criterion: a web agent run whose last data-producing tool call was
// run_question composes the exact ai.Response shape
// (result.{logical_query,sql,warnings,result,confidence,visualization_hint,
// answer,suggested_followups} + metadata.{run_steps,run_id}) that
// AssistantMessageCard's normalizeAIQueryResponse already knows how to
// render for the legacy job pipeline.
func TestComposeWebAgentFinalResultRunQuestionGolden(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	nestedResp := ai.Response{
		Result: &ai.AIResult{
			LogicalQuery: goldenLogicalQuery(),
			SQL:          "SELECT month, SUM(revenue) FROM orders GROUP BY month",
			Warnings:     []string{"sampled last 90 days"},
			Result:       goldenResult(),
			Confidence:   0.42, // must NOT leak into the composed response: the
			// web agent's own FinalResponse.Confidence is authoritative.
			Answer: "nested answer must not leak either",
		},
	}
	payload, err := sonic.Marshal(nestedResp)
	require.NoError(t, err)

	state := agent.RuntimeState{
		Steps: []agent.RuntimeStep{
			{
				Seq:         1,
				Proposal:    agent.Proposal{Tool: agent.ToolWebRunQuestion, Arguments: []byte(`{"datasource_id":"ds-1","question":"revenue by month"}`)},
				Observation: &agent.Observation{Tool: agent.ToolWebRunQuestion, Payload: payload},
			},
		},
		Terminal: &agent.TerminalResult{
			Kind:  agent.DecisionFinal,
			Final: &agent.FinalResponse{Answer: "Revenue grew from $100 to $200.", Confidence: 0.9},
		},
	}

	req := webAgentRequest{Message: "revenue by month", DatasourceID: "ds-1"}
	got := h.composeWebAgentFinalResult(context.Background(), req, "run-1", state)

	require.NotNil(t, got.Result)
	require.NotNil(t, got.Metadata)

	// The planner's own final answer/confidence are authoritative — h.service
	// is nil in this harness, so attachAINaturalLanguageAnswer is a no-op and
	// must not have overwritten them with the nested tool payload's values.
	assert.Equal(t, "Revenue grew from $100 to $200.", got.Result.Answer)
	assert.Equal(t, 0.9, got.Result.Confidence)

	assert.Equal(t, goldenLogicalQuery(), got.Result.LogicalQuery)
	assert.Equal(t, "SELECT month, SUM(revenue) FROM orders GROUP BY month", got.Result.SQL)
	assert.Contains(t, got.Result.Warnings, "sampled last 90 days")
	assert.Equal(t, goldenResult(), got.Result.Result)

	require.NotNil(t, got.Result.VisualizationHint)
	assert.Equal(t, query.ChartLine, got.Result.VisualizationHint.ChartType)

	// Deterministic follow-ups fire for a time+metric, multi-row result.
	assert.NotEmpty(t, got.Result.SuggestedFollowUps)

	assert.Equal(t, "run-1", got.Metadata.RunID)
	require.Len(t, got.Metadata.RunSteps, 1)
	assert.Equal(t, ai.RunStep{Seq: 1, Kind: "run_question", Status: ai.RunStepStatusOK}, got.Metadata.RunSteps[0])

	// Round-trip through JSON and confirm the shape normalizeAIQueryResponse
	// (frontend/src/utils/normalizeAIQueryResponse.ts) expects: result.sql,
	// result.logical_query, metadata.run_steps and metadata.run_id all
	// present at exactly those JSON paths.
	raw, err := sonic.Marshal(got)
	require.NoError(t, err)
	var asMap map[string]any
	require.NoError(t, sonic.Unmarshal(raw, &asMap))
	resultMap, ok := asMap["result"].(map[string]any)
	require.True(t, ok, "result must be a nested object")
	assert.Equal(t, "SELECT month, SUM(revenue) FROM orders GROUP BY month", resultMap["sql"])
	assert.NotNil(t, resultMap["logical_query"])
	metadataMap, ok := asMap["metadata"].(map[string]any)
	require.True(t, ok, "metadata must be a nested object")
	assert.Equal(t, "run-1", metadataMap["run_id"])
	assert.NotEmpty(t, metadataMap["run_steps"])
}

// TestComposeWebAgentFinalResultRunLogicalQueryHasNoSQLPreview proves the
// brief's SQL-preview-gating requirement: /api/query/run (what
// run_logical_query dispatches to) never echoes SQL back, so the composed
// result must not fabricate one — SQL stays empty exactly like the legacy
// pipeline's own behavior when no SQL is available. The LogicalQuery instead
// comes from the tool call's own arguments, since run_logical_query's
// response never echoes it back either.
func TestComposeWebAgentFinalResultRunLogicalQueryHasNoSQLPreview(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	result := goldenResult()
	payload, err := sonic.Marshal(result)
	require.NoError(t, err)

	lq := goldenLogicalQuery()
	lqRaw, err := sonic.Marshal(lq)
	require.NoError(t, err)
	var lqMap map[string]any
	require.NoError(t, sonic.Unmarshal(lqRaw, &lqMap))
	args, err := sonic.Marshal(map[string]any{"logical_query": lqMap})
	require.NoError(t, err)

	state := agent.RuntimeState{
		Steps: []agent.RuntimeStep{
			{
				Seq:         1,
				Proposal:    agent.Proposal{Tool: agent.ToolWebRunLogicalQuery, Arguments: args},
				Observation: &agent.Observation{Tool: agent.ToolWebRunLogicalQuery, Payload: payload},
			},
		},
		Terminal: &agent.TerminalResult{
			Kind:  agent.DecisionFinal,
			Final: &agent.FinalResponse{Answer: "done", Confidence: 0.8},
		},
	}

	got := h.composeWebAgentFinalResult(context.Background(), webAgentRequest{Message: "q"}, "run-2", state)

	assert.Equal(t, lq, got.Result.LogicalQuery)
	assert.Empty(t, got.Result.SQL, "run_logical_query's /api/query/run response never echoes SQL back")
	assert.Equal(t, result, got.Result.Result)
}

// TestComposeWebAgentFinalResultRunSkillIncludesSQL proves run_skill's
// distinct wire shape (skillRunResponse: skill_id/name/sql/result) is decoded
// correctly, including its SQL (which, unlike run_logical_query, IS echoed
// back by that endpoint).
func TestComposeWebAgentFinalResultRunSkillIncludesSQL(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	payload, err := sonic.Marshal(map[string]any{
		"skill_id": "skill-1",
		"name":     "Monthly Revenue",
		"sql":      "SELECT * FROM revenue_by_month",
		"result":   goldenResult(),
	})
	require.NoError(t, err)

	state := agent.RuntimeState{
		Steps: []agent.RuntimeStep{
			{
				Seq:         1,
				Proposal:    agent.Proposal{Tool: agent.ToolWebRunSkill, Arguments: []byte(`{"skill_id":"skill-1"}`)},
				Observation: &agent.Observation{Tool: agent.ToolWebRunSkill, Payload: payload},
			},
		},
		Terminal: &agent.TerminalResult{Kind: agent.DecisionFinal, Final: &agent.FinalResponse{Answer: "done"}},
	}

	got := h.composeWebAgentFinalResult(context.Background(), webAgentRequest{Message: "q"}, "run-3", state)

	assert.Equal(t, "SELECT * FROM revenue_by_month", got.Result.SQL)
	assert.Nil(t, got.Result.LogicalQuery, "run_skill's response never echoes a LogicalQuery back")
	assert.Equal(t, goldenResult(), got.Result.Result)
}

// TestComposeWebAgentFinalResultNoQueryToolFallsBackToPlannerAnswer proves a
// run that never executed a query (e.g. the planner only called
// list_models) still composes a valid result: the planner's own final
// answer/confidence, no LogicalQuery/SQL/Result.
func TestComposeWebAgentFinalResultNoQueryToolFallsBackToPlannerAnswer(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	state := agent.RuntimeState{
		Steps: []agent.RuntimeStep{
			{
				Seq:         1,
				Proposal:    agent.Proposal{Tool: agent.ToolWebListModels, Arguments: []byte(`{}`)},
				Observation: &agent.Observation{Tool: agent.ToolWebListModels, Payload: []byte(`{"models":[]}`)},
			},
		},
		Terminal: &agent.TerminalResult{
			Kind:  agent.DecisionFinal,
			Final: &agent.FinalResponse{Answer: "There are no models yet.", Confidence: 0.99},
		},
	}

	got := h.composeWebAgentFinalResult(context.Background(), webAgentRequest{Message: "what models exist?"}, "run-4", state)

	assert.Equal(t, "There are no models yet.", got.Result.Answer)
	assert.Equal(t, 0.99, got.Result.Confidence)
	assert.Nil(t, got.Result.LogicalQuery)
	assert.Empty(t, got.Result.SQL)
	assert.Nil(t, got.Result.Result)
	assert.Nil(t, got.Result.VisualizationHint)
	require.Len(t, got.Metadata.RunSteps, 1)
	assert.Equal(t, "list_models", got.Metadata.RunSteps[0].Kind)
}

// TestComposeWebAgentFinalResultSkipsDeniedStepInFavorOfEarlierSuccess proves
// lastWebAgentQueryData walks steps most-recent-first but skips any step
// without a usable Observation (denied or errored) — a denied retry must
// never shadow an earlier successful query result.
func TestComposeWebAgentFinalResultSkipsDeniedStepInFavorOfEarlierSuccess(t *testing.T) {
	h := newAIHandlerWithRepo(nil)

	result := goldenResult()
	payload, err := sonic.Marshal(result)
	require.NoError(t, err)

	state := agent.RuntimeState{
		Steps: []agent.RuntimeStep{
			{
				Seq:         1,
				Proposal:    agent.Proposal{Tool: agent.ToolWebRunLogicalQuery, Arguments: []byte(`{"logical_query":{"datasource_id":"ds-1","model_id":"model-1","select":[],"limit":100}}`)},
				Observation: &agent.Observation{Tool: agent.ToolWebRunLogicalQuery, Payload: payload},
			},
			{
				Seq:          2,
				Proposal:     agent.Proposal{Tool: agent.ToolWebRunLogicalQuery, Arguments: []byte(`{"logical_query":{}}`)},
				DeniedReason: "invalid_join_denied",
			},
		},
		Terminal: &agent.TerminalResult{Kind: agent.DecisionFinal, Final: &agent.FinalResponse{Answer: "done"}},
	}

	got := h.composeWebAgentFinalResult(context.Background(), webAgentRequest{Message: "q"}, "run-5", state)

	require.NotNil(t, got.Result.Result)
	assert.Equal(t, result, got.Result.Result)
	require.Len(t, got.Metadata.RunSteps, 2)
	assert.Equal(t, ai.RunStepStatusOK, got.Metadata.RunSteps[0].Status)
	assert.Equal(t, ai.RunStepStatusFailed, got.Metadata.RunSteps[1].Status)
	assert.Equal(t, "invalid_join_denied", got.Metadata.RunSteps[1].Detail)
}

// TestWebAgentRunSteps unit-tests the RuntimeStep -> ai.RunStep status/detail
// mapping in isolation from the full finalizer.
func TestWebAgentRunSteps(t *testing.T) {
	steps := []agent.RuntimeStep{
		{Seq: 1, Proposal: agent.Proposal{Tool: agent.ToolWebListDatasources}, Observation: &agent.Observation{}},
		{Seq: 2, Proposal: agent.Proposal{Tool: agent.ToolWebRunLogicalQuery}, DeniedReason: "tool_not_allowlisted"},
		{Seq: 3, Proposal: agent.Proposal{Tool: agent.ToolWebRunQuestion}, Error: "upstream timeout"},
	}

	got := webAgentRunSteps(steps)

	require.Len(t, got, 3)
	assert.Equal(t, ai.RunStep{Seq: 1, Kind: "list_datasources", Status: ai.RunStepStatusOK}, got[0])
	assert.Equal(t, ai.RunStep{Seq: 2, Kind: "run_logical_query", Status: ai.RunStepStatusFailed, Detail: "tool_not_allowlisted"}, got[1])
	assert.Equal(t, ai.RunStep{Seq: 3, Kind: "run_question", Status: ai.RunStepStatusFailed, Detail: "upstream timeout"}, got[2])
}
