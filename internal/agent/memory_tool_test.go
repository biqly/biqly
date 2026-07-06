package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMemoryRecaller struct {
	calls        int
	gotCtx       []context.Context
	gotLimit     int
	failuresLeft int
	failWith     error
	result       []RecalledExample
}

func (f *fakeMemoryRecaller) Recall(ctx context.Context, _, _, _ string, limit int) ([]RecalledExample, error) {
	f.calls++
	f.gotCtx = append(f.gotCtx, ctx)
	f.gotLimit = limit
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return nil, f.failWith
	}
	return f.result, nil
}

func memoryArgs(t *testing.T, run RunContext, question string, limit int) []byte {
	t.Helper()
	raw, err := sonic.Marshal(memoryRecallArgs{
		identityArgs: identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID},
		Question:     question,
		Limit:        limit,
	})
	require.NoError(t, err)
	return raw
}

func TestMemoryToolRecallsExamples(t *testing.T) {
	fake := &fakeMemoryRecaller{result: []RecalledExample{{Question: "q1"}}}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "how much revenue", 3))
	require.NoError(t, err)
	assert.Equal(t, ToolMemoryRecall, obs.Tool)
	assert.Equal(t, 3, fake.gotLimit)

	var examples []RecalledExample
	require.NoError(t, sonic.Unmarshal(obs.Payload, &examples))
	assert.Equal(t, []RecalledExample{{Question: "q1"}}, examples)
}

func TestMemoryToolClampsLimitToMax(t *testing.T) {
	fake := &fakeMemoryRecaller{result: []RecalledExample{}}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "q", 999))
	require.NoError(t, err)
	assert.Equal(t, 5, fake.gotLimit, "requested limit above the ceiling must be clamped, never honored")
}

func TestMemoryToolDefaultsZeroLimitToMax(t *testing.T) {
	fake := &fakeMemoryRecaller{result: []RecalledExample{}}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "q", 0))
	require.NoError(t, err)
	assert.Equal(t, 5, fake.gotLimit)
}

func TestMemoryToolRequiresQuestion(t *testing.T) {
	tool := NewMemoryTool(&fakeMemoryRecaller{}, 5)
	run := baseRunContext()
	_, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "", 1))
	assert.Error(t, err)
}

func TestMemoryToolRejectsUnknownFields(t *testing.T) {
	tool := NewMemoryTool(&fakeMemoryRecaller{}, 5)
	_, err := tool.Execute(context.Background(), baseRunContext(),
		[]byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","question":"q","destination_url":"http://evil.example"}`))
	assert.Error(t, err)
}

func TestMemoryToolPropagatesContextDeadline(t *testing.T) {
	fake := &fakeMemoryRecaller{result: []RecalledExample{}}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	_, err := tool.Execute(ctx, run, memoryArgs(t, run, "q", 1))
	require.NoError(t, err)
	require.Len(t, fake.gotCtx, 1)
	gotDeadline, ok := fake.gotCtx[0].Deadline()
	require.True(t, ok)
	assert.Equal(t, deadline, gotDeadline)
}

func TestMemoryToolRetriesTransientErrorAtMostOnce(t *testing.T) {
	fake := &fakeMemoryRecaller{
		failuresLeft: 1,
		failWith:     &TransientError{Err: errors.New("timeout")},
		result:       []RecalledExample{{Question: "q"}},
	}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "q", 1))
	require.NoError(t, err)
	assert.Equal(t, 2, fake.calls)
	var examples []RecalledExample
	require.NoError(t, sonic.Unmarshal(obs.Payload, &examples))
	assert.Len(t, examples, 1)
}

func TestRegistryDispatchesToMemoryTool(t *testing.T) {
	fake := &fakeMemoryRecaller{result: []RecalledExample{{Question: "q1"}}}
	registry := NewRegistry(&PolicyEngine{}, NewMemoryTool(fake, 5))
	run := baseRunContext()

	obs, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolMemoryRecall,
		Arguments: memoryArgs(t, run, "q", 1),
	})
	require.NoError(t, err)
	assert.Equal(t, ToolMemoryRecall, obs.Tool)
	assert.Equal(t, 1, fake.calls)
}

func TestMemoryToolDoesNotRetryNonTransientError(t *testing.T) {
	fake := &fakeMemoryRecaller{failuresLeft: 1, failWith: errors.New("bad request")}
	tool := NewMemoryTool(fake, 5)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, memoryArgs(t, run, "q", 1))
	assert.Error(t, err)
	assert.Equal(t, 1, fake.calls)
}
