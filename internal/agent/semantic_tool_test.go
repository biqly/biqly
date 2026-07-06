package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSemanticGenerator struct {
	calls        int
	gotCtx       []context.Context
	gotQuestion  string
	failuresLeft int
	failWith     error
	result       SemanticPlan
}

func (f *fakeSemanticGenerator) GeneratePlan(ctx context.Context, _, _, question string) (SemanticPlan, error) {
	f.calls++
	f.gotCtx = append(f.gotCtx, ctx)
	f.gotQuestion = question
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return SemanticPlan{}, f.failWith
	}
	return f.result, nil
}

func semanticArgs(t *testing.T, run RunContext, question string) []byte {
	t.Helper()
	raw, err := sonic.Marshal(semanticResolveArgs{
		identityArgs: identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID},
		Question:     question,
	})
	require.NoError(t, err)
	return raw
}

func TestSemanticToolGeneratesPlan(t *testing.T) {
	fake := &fakeSemanticGenerator{result: SemanticPlan{Confidence: 0.9, LogicalQuery: json.RawMessage(`{"metrics":["revenue"]}`)}}
	tool := NewSemanticTool(fake)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, semanticArgs(t, run, "what is revenue"))
	require.NoError(t, err)
	assert.Equal(t, ToolSemantic, obs.Tool)
	assert.Equal(t, "what is revenue", fake.gotQuestion)

	var plan SemanticPlan
	require.NoError(t, sonic.Unmarshal(obs.Payload, &plan))
	assert.Equal(t, 0.9, plan.Confidence)
}

func TestSemanticToolRequiresQuestion(t *testing.T) {
	tool := NewSemanticTool(&fakeSemanticGenerator{})
	run := baseRunContext()
	_, err := tool.Execute(context.Background(), run, semanticArgs(t, run, ""))
	assert.Error(t, err)
}

func TestSemanticToolRejectsUnknownFields(t *testing.T) {
	tool := NewSemanticTool(&fakeSemanticGenerator{})
	_, err := tool.Execute(context.Background(), baseRunContext(),
		[]byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","question":"q","raw_sql":"SELECT 1"}`))
	assert.Error(t, err)
}

func TestSemanticToolPropagatesContextDeadline(t *testing.T) {
	fake := &fakeSemanticGenerator{result: SemanticPlan{}}
	tool := NewSemanticTool(fake)
	run := baseRunContext()

	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	_, err := tool.Execute(ctx, run, semanticArgs(t, run, "q"))
	require.NoError(t, err)
	require.Len(t, fake.gotCtx, 1)
	gotDeadline, ok := fake.gotCtx[0].Deadline()
	require.True(t, ok)
	assert.Equal(t, deadline, gotDeadline)
}

func TestSemanticToolRetriesTransientErrorAtMostOnce(t *testing.T) {
	fake := &fakeSemanticGenerator{
		failuresLeft: 1,
		failWith:     &TransientError{Err: errors.New("timeout")},
		result:       SemanticPlan{Confidence: 0.5},
	}
	tool := NewSemanticTool(fake)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, semanticArgs(t, run, "q"))
	require.NoError(t, err)
	assert.Equal(t, 2, fake.calls)
	var plan SemanticPlan
	require.NoError(t, sonic.Unmarshal(obs.Payload, &plan))
	assert.Equal(t, 0.5, plan.Confidence)
}

func TestRegistryDispatchesToSemanticTool(t *testing.T) {
	fake := &fakeSemanticGenerator{result: SemanticPlan{Confidence: 0.7}}
	registry := NewRegistry(&PolicyEngine{}, NewSemanticTool(fake))
	run := baseRunContext()

	obs, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolSemantic,
		Arguments: semanticArgs(t, run, "q"),
	})
	require.NoError(t, err)
	assert.Equal(t, ToolSemantic, obs.Tool)
	assert.Equal(t, 1, fake.calls)
}

func TestSemanticToolDoesNotRetryNonTransientError(t *testing.T) {
	fake := &fakeSemanticGenerator{failuresLeft: 1, failWith: errors.New("bad request")}
	tool := NewSemanticTool(fake)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, semanticArgs(t, run, "q"))
	assert.Error(t, err)
	assert.Equal(t, 1, fake.calls)
}
