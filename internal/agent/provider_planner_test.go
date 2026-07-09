package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct {
	gotPrompt string
	result    providerpkg.GenerationResult
	err       error
}

func (f *fakeProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	f.gotPrompt = prompt
	return f.result, f.err
}

func (f *fakeProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return f.Generate(ctx, prompt)
}

type queuedProvider struct {
	prompts []string
	results []providerpkg.GenerationResult
}

func (q *queuedProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	q.prompts = append(q.prompts, prompt)
	if len(q.prompts) > len(q.results) {
		return providerpkg.GenerationResult{}, errors.New("provider queue exhausted")
	}
	return q.results[len(q.prompts)-1], nil
}

func (q *queuedProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return q.Generate(ctx, prompt)
}

type staticTool struct {
	name    ToolName
	payload string
	calls   int
}

func (t *staticTool) Name() ToolName { return t.name }

func (t *staticTool) Execute(_ context.Context, _ RunContext, _ json.RawMessage) (Observation, error) {
	t.calls++
	return Observation{Tool: t.name, Payload: []byte(t.payload)}, nil
}

func TestProviderPlannerDecodesToolDecision(t *testing.T) {
	provider := &fakeProvider{result: providerpkg.GenerationResult{
		Content: `{"tool":{"name":"catalog.resolve","arguments":{"tenant_id":"t","user_id":"u","datasource_id":"d"}}}`,
	}}
	planner := NewProviderPlanner(provider)
	run := baseRunContext()
	run.Question = "what is revenue?"

	decision, err := planner.Decide(context.Background(), run, nil)
	require.NoError(t, err)
	assert.Equal(t, DecisionTool, decision.Kind)
	assert.Equal(t, ToolCatalog, decision.Proposal.Tool)
	assert.Contains(t, provider.gotPrompt, "what is revenue?")
	assert.Contains(t, provider.gotPrompt, "No steps taken yet.")
}

func TestProviderPlannerExtractsJSONFromSurroundingProse(t *testing.T) {
	provider := &fakeProvider{result: providerpkg.GenerationResult{
		Content: "Here is my decision:\n" + `{"final":{"answer":"42","confidence":0.9}}` + "\nDone.",
	}}
	planner := NewProviderPlanner(provider)

	decision, err := planner.Decide(context.Background(), baseRunContext(), nil)
	require.NoError(t, err)
	assert.Equal(t, DecisionFinal, decision.Kind)
	assert.Equal(t, "42", decision.Final.Answer)
}

func TestProviderPlannerRejectsResponseWithNoJSON(t *testing.T) {
	provider := &fakeProvider{result: providerpkg.GenerationResult{Content: "I'm not sure what to do."}}
	planner := NewProviderPlanner(provider)

	_, err := planner.Decide(context.Background(), baseRunContext(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPlannerResponseHasNoJSON))
}

func TestProviderPlannerPropagatesGenerateError(t *testing.T) {
	provider := &fakeProvider{err: errors.New("provider unavailable")}
	planner := NewProviderPlanner(provider)

	_, err := planner.Decide(context.Background(), baseRunContext(), nil)
	assert.Error(t, err)
}

func TestProviderPlannerRejectsMalformedEnvelope(t *testing.T) {
	provider := &fakeProvider{result: providerpkg.GenerationResult{Content: `{"tool":{},"final":{"answer":"x","confidence":1}}`}}
	planner := NewProviderPlanner(provider)

	_, err := planner.Decide(context.Background(), baseRunContext(), nil)
	assert.Error(t, err, "mixed variants must be rejected even when they come from the model itself")
}

func TestBuildPlannerPromptDescribesDeniedAndErroredSteps(t *testing.T) {
	run := baseRunContext()
	run.Question = "q"
	history := []RuntimeStep{
		{Seq: 1, Proposal: Proposal{Tool: ToolQueryExecute}, DeniedReason: ReasonToolNotAllowlisted},
		{Seq: 2, Proposal: Proposal{Tool: ToolCatalog}, Error: "upstream timeout"},
		{Seq: 3, Proposal: Proposal{Tool: ToolSemantic}, Observation: &Observation{Payload: []byte(`{"ok":true}`)}},
	}
	prompt := buildPlannerPrompt(run, history)
	assert.Contains(t, prompt, "DENIED reason=tool_not_allowlisted")
	assert.Contains(t, prompt, "ERROR=upstream timeout")
	assert.Contains(t, prompt, `observation={"ok":true}`)
}

func TestBuildPlannerPromptDescribesWebToolsAndPriorTurns(t *testing.T) {
	run := baseRunContext()
	run.Question = "what about this month?"
	run.AllowedTools = []ToolName{ToolWebListSkills, ToolWebRunSkill, ToolWebRunQuestion}
	run.PriorTurns = []PriorTurn{
		{User: "show last month revenue by region", Assistant: "Revenue was split by region.", ResultSummary: "filters: last month; group_by: region"},
	}

	prompt := buildPlannerPrompt(run, nil)

	assert.Contains(t, prompt, "Never write raw SQL")
	assert.Contains(t, prompt, "Prefer list_skills then run_skill")
	assert.Contains(t, prompt, "Prior turns")
	assert.Contains(t, prompt, "last month")
	assert.Contains(t, prompt, "inherit")
	assert.Contains(t, prompt, "list_skills:")
	assert.Contains(t, prompt, "run_question:")
	assert.NotContains(t, prompt, "list_datasources:")
}

func TestProviderPlannerWebHappyPathListModelsRunQuestionFinal(t *testing.T) {
	provider := &queuedProvider{results: []providerpkg.GenerationResult{
		{Content: `{"tool":{"name":"list_models","arguments":{"datasource_id":"ds-1"}}}`},
		{Content: `{"tool":{"name":"run_question","arguments":{"datasource_id":"ds-1","question":"revenue by region","model_id":"model-1"}}}`},
		{Content: `{"final":{"answer":"Regional revenue is ready.","confidence":0.92}}`},
	}}
	planner := NewProviderPlanner(provider)
	listModels := &staticTool{name: ToolWebListModels, payload: `{"models":[{"id":"model-1","name":"Revenue"}]}`}
	runQuestion := &staticTool{name: ToolWebRunQuestion, payload: `{"rows":[{"region":"TR","revenue":42}],"logical_query":{"select":[]}}`}
	registry := NewRegistry(&PolicyEngine{}, listModels, runQuestion)
	run := runtimeTestRun()
	run.Question = "revenue by region"
	run.AllowedTools = []ToolName{ToolWebListModels, ToolWebRunQuestion}
	run.MaxSteps = 6
	run.MaxClarificationRounds = 2

	state, err := NewRuntime(planner, registry, newFakeStateStore()).Run(context.Background(), run, "web-run-1")

	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, DecisionFinal, state.Terminal.Kind)
	assert.Equal(t, "Regional revenue is ready.", state.Terminal.Final.Answer)
	require.Len(t, state.Steps, 2)
	assert.Equal(t, 1, listModels.calls)
	assert.Equal(t, 1, runQuestion.calls)
	require.Len(t, provider.prompts, 3)
	assert.Contains(t, provider.prompts[0], "list_models:")
	assert.Contains(t, provider.prompts[1], `"models"`)
	assert.Contains(t, provider.prompts[2], `"rows"`)
}

// TestBuildPlannerPromptDescribesPriorClarification proves a resumed run's
// answered clarification round-trip reaches the planner prompt (T8: "the
// resumed run's next planner call sees the Q&A"), and that a fresh run
// (ClarificationHistory empty) renders no such section.
func TestBuildPlannerPromptDescribesPriorClarification(t *testing.T) {
	run := baseRunContext()
	run.Question = "revenue by region"
	run.ClarificationHistory = []ClarificationExchange{{Question: "which metric?", Answer: "net_revenue"}}

	prompt := buildPlannerPrompt(run, nil)
	assert.Contains(t, prompt, "which metric?")
	assert.Contains(t, prompt, "net_revenue")
	assert.Contains(t, prompt, "do not ask the same question again")

	fresh := buildPlannerPrompt(baseRunContext(), nil)
	assert.NotContains(t, fresh, "previously asked these clarifications")
}

// TestBuildPlannerPromptDescribesFullClarificationHistory proves a SECOND
// resume round's planner prompt sees BOTH round 1's Q1/A1 and round 2's
// Q2/A2 — not just the latest round — closing the gap where
// RuntimeState.PendingClarification (a single field) used to be
// unconditionally overwritten each round, silently dropping round 1's
// resolution by the time round 2's resume happened.
func TestBuildPlannerPromptDescribesFullClarificationHistory(t *testing.T) {
	run := baseRunContext()
	run.Question = "revenue by region"
	run.ClarificationHistory = []ClarificationExchange{
		{Question: "which metric?", Answer: "net_revenue"},
		{Question: "which quarter?", Answer: "Q2"},
	}

	prompt := buildPlannerPrompt(run, nil)
	assert.Contains(t, prompt, "which metric?")
	assert.Contains(t, prompt, "net_revenue")
	assert.Contains(t, prompt, "which quarter?")
	assert.Contains(t, prompt, "Q2")
}

func TestTruncateLongObservationPayload(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'x'
	}
	got := truncate(string(long), 500)
	assert.Contains(t, got, "100 more bytes truncated")
}
