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

type fakeQueryCompiler struct {
	calls        int
	gotCtx       []context.Context
	gotLQ        []string
	failuresLeft int
	failWith     error
	result       CompileResult
}

func (f *fakeQueryCompiler) Compile(ctx context.Context, _ string, logicalQuery json.RawMessage) (CompileResult, error) {
	f.calls++
	f.gotCtx = append(f.gotCtx, ctx)
	f.gotLQ = append(f.gotLQ, string(logicalQuery))
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return CompileResult{}, f.failWith
	}
	return f.result, nil
}

type fakeQueryExecutor struct {
	calls          int
	gotCtx         []context.Context
	gotRowLimit    int
	gotTimeoutSecs int
	failuresLeft   int
	failWith       error
	result         QueryResult
}

func (f *fakeQueryExecutor) Execute(ctx context.Context, _ string, _ json.RawMessage, rowLimit, timeoutSeconds int) (QueryResult, error) {
	f.calls++
	f.gotCtx = append(f.gotCtx, ctx)
	f.gotRowLimit = rowLimit
	f.gotTimeoutSecs = timeoutSeconds
	if f.failuresLeft > 0 {
		f.failuresLeft--
		return QueryResult{}, f.failWith
	}
	return f.result, nil
}

func compileArgs(t *testing.T, run RunContext, lq string) []byte {
	t.Helper()
	raw, err := sonic.Marshal(queryProposalArgs{
		identityArgs: identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID},
		LogicalQuery: json.RawMessage(lq),
	})
	require.NoError(t, err)
	return raw
}

func executeArgs(t *testing.T, run RunContext, lq, fingerprint string, rowLimit, timeoutSeconds int) []byte {
	t.Helper()
	raw, err := sonic.Marshal(queryProposalArgs{
		identityArgs:   identityArgs{TenantID: run.TenantID, UserID: run.UserID, DatasourceID: run.DatasourceID},
		LogicalQuery:   json.RawMessage(lq),
		Fingerprint:    fingerprint,
		RowLimit:       rowLimit,
		TimeoutSeconds: timeoutSeconds,
	})
	require.NoError(t, err)
	return raw
}

func TestQueryCompileToolCompiles(t *testing.T) {
	fake := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1", SQL: "SELECT 1"}}
	tool := NewQueryCompileTool(fake)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, compileArgs(t, run, `{"metrics":["revenue"]}`))
	require.NoError(t, err)
	assert.Equal(t, ToolQueryCompile, obs.Tool)

	var result CompileResult
	require.NoError(t, sonic.Unmarshal(obs.Payload, &result))
	assert.Equal(t, "fp-1", result.Fingerprint)
}

func TestQueryCompileToolRequiresLogicalQuery(t *testing.T) {
	tool := NewQueryCompileTool(&fakeQueryCompiler{})
	run := baseRunContext()
	_, err := tool.Execute(context.Background(), run, compileArgs(t, run, ""))
	assert.Error(t, err)
}

func TestQueryCompileToolRejectsUnknownFields(t *testing.T) {
	tool := NewQueryCompileTool(&fakeQueryCompiler{})
	_, err := tool.Execute(context.Background(), baseRunContext(),
		[]byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","logical_query":{},"raw_sql":"SELECT 1"}`))
	assert.Error(t, err)
}

func TestQueryCompileToolPropagatesContextDeadline(t *testing.T) {
	fake := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp"}}
	tool := NewQueryCompileTool(fake)
	run := baseRunContext()

	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	_, err := tool.Execute(ctx, run, compileArgs(t, run, `{}`))
	require.NoError(t, err)
	require.Len(t, fake.gotCtx, 1)
	gotDeadline, ok := fake.gotCtx[0].Deadline()
	require.True(t, ok)
	assert.Equal(t, deadline, gotDeadline)
}

func TestQueryCompileToolRetriesTransientErrorAtMostOnce(t *testing.T) {
	fake := &fakeQueryCompiler{
		failuresLeft: 1,
		failWith:     &TransientError{Err: errors.New("timeout")},
		result:       CompileResult{Fingerprint: "fp"},
	}
	tool := NewQueryCompileTool(fake)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, compileArgs(t, run, `{}`))
	require.NoError(t, err)
	assert.Equal(t, 2, fake.calls)
}

func TestRegistryDispatchesToQueryCompileTool(t *testing.T) {
	fake := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	registry := NewRegistry(&PolicyEngine{}, NewQueryCompileTool(fake))
	run := baseRunContext()

	obs, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolQueryCompile,
		Arguments: compileArgs(t, run, `{}`),
	})
	require.NoError(t, err)
	assert.Equal(t, ToolQueryCompile, obs.Tool)
	assert.Equal(t, 1, fake.calls)
}

func TestQueryExecuteToolExecutesWhenFingerprintMatches(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1", SQL: "SELECT 1"}}
	executor := &fakeQueryExecutor{result: QueryResult{RowCount: 3}}
	tool := NewQueryExecuteTool(compiler, executor)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, executeArgs(t, run, `{"metrics":["revenue"]}`, "fp-1", 100, 20))
	require.NoError(t, err)
	assert.Equal(t, ToolQueryExecute, obs.Tool)
	assert.Equal(t, 1, compiler.calls, "execute must revalidate via compile before running")
	assert.Equal(t, 100, executor.gotRowLimit)
	assert.Equal(t, 20, executor.gotTimeoutSecs)

	var result QueryResult
	require.NoError(t, sonic.Unmarshal(obs.Payload, &result))
	assert.Equal(t, 3, result.RowCount)
}

func TestQueryExecuteToolDeniesFingerprintMismatch(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-real"}}
	executor := &fakeQueryExecutor{result: QueryResult{RowCount: 99}}
	tool := NewQueryExecuteTool(compiler, executor)
	run := baseRunContext()

	_, err := tool.Execute(context.Background(), run, executeArgs(t, run, `{"metrics":["revenue"]}`, "fp-claimed-but-wrong", 100, 20))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCompileFingerprintMismatch))
	assert.Zero(t, executor.calls, "must never execute when the fingerprint claim doesn't match a fresh compile")
}

func TestQueryExecuteToolRequiresFingerprint(t *testing.T) {
	tool := NewQueryExecuteTool(&fakeQueryCompiler{}, &fakeQueryExecutor{})
	run := baseRunContext()
	_, err := tool.Execute(context.Background(), run, executeArgs(t, run, `{}`, "", 10, 10))
	assert.Error(t, err)
}

func TestQueryExecuteToolRejectsUnknownFields(t *testing.T) {
	tool := NewQueryExecuteTool(&fakeQueryCompiler{}, &fakeQueryExecutor{})
	_, err := tool.Execute(context.Background(), baseRunContext(),
		[]byte(`{"tenant_id":"t","user_id":"u","datasource_id":"d","logical_query":{},"fingerprint":"fp","credentials":"secret"}`))
	assert.Error(t, err)
}

func TestQueryExecuteToolPropagatesContextDeadlineToExecutor(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	executor := &fakeQueryExecutor{result: QueryResult{}}
	tool := NewQueryExecuteTool(compiler, executor)
	run := baseRunContext()

	deadline := time.Now().Add(time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	_, err := tool.Execute(ctx, run, executeArgs(t, run, `{}`, "fp-1", 10, 10))
	require.NoError(t, err)
	require.Len(t, executor.gotCtx, 1)
	gotDeadline, ok := executor.gotCtx[0].Deadline()
	require.True(t, ok)
	assert.Equal(t, deadline, gotDeadline)
}

func TestQueryExecuteToolRetriesTransientExecutorErrorAtMostOnce(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	executor := &fakeQueryExecutor{
		failuresLeft: 1,
		failWith:     &TransientError{Err: errors.New("upstream flaked")},
		result:       QueryResult{RowCount: 5},
	}
	tool := NewQueryExecuteTool(compiler, executor)
	run := baseRunContext()

	obs, err := tool.Execute(context.Background(), run, executeArgs(t, run, `{}`, "fp-1", 10, 10))
	require.NoError(t, err)
	assert.Equal(t, 2, executor.calls)
	var result QueryResult
	require.NoError(t, sonic.Unmarshal(obs.Payload, &result))
	assert.Equal(t, 5, result.RowCount)
}

// End-to-end through Registry+PolicyEngine: the row limit and timeout the
// executor receives are the policy-narrowed values, never the planner's
// originally requested ones.
func TestQueryExecuteToolReceivesPolicyNarrowedLimitsThroughRegistry(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	executor := &fakeQueryExecutor{result: QueryResult{RowCount: 1}}
	registry := NewRegistry(&PolicyEngine{}, NewQueryExecuteTool(compiler, executor))

	run := baseRunContext()
	run.MaxRows = 50
	run.Timeout = 10 * time.Second

	_, err := registry.Execute(context.Background(), run, Proposal{
		Tool:      ToolQueryExecute,
		Arguments: executeArgs(t, run, `{}`, "fp-1", 10000, 999),
	})
	require.NoError(t, err)
	assert.Equal(t, 50, executor.gotRowLimit, "policy must clamp the row limit before it reaches the executor")
	assert.Equal(t, 10, executor.gotTimeoutSecs, "policy must clamp the timeout before it reaches the executor")
}
