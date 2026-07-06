package agent

import (
	"context"
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

func TestTruncateLongObservationPayload(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'x'
	}
	got := truncate(string(long), 500)
	assert.Contains(t, got, "100 more bytes truncated")
}
