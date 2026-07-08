package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

type fakeWebAgentLimiter struct {
	err       error
	acquired  int
	released  int
	workspace string
	user      string
	ttl       time.Duration
}

func (l *fakeWebAgentLimiter) Acquire(_ context.Context, workspaceID, userID string, ttl time.Duration) (func(context.Context), error) {
	l.acquired++
	l.workspace = workspaceID
	l.user = userID
	l.ttl = ttl
	if l.err != nil {
		return nil, l.err
	}
	return func(context.Context) {
		l.released++
	}, nil
}

func TestWebAgentChatDisabled(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: false}

	rec := httptest.NewRecorder()
	h.WebAgentChat(rec, httptest.NewRequestWithContext(
		context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{}`)))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWebAgentChatMissingRuntimeReturnsSSEError(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"runtime_unavailable"`)
	assert.Contains(t, body, "data: [DONE]")
}

func TestWebAgentChatMissingConcurrencyGuardReturnsSSEError(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		t.Fatal("runner should not be called when concurrency guard is unavailable")
		return agent.RuntimeState{}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"concurrency_unavailable"`)
	assert.Contains(t, body, "data: [DONE]")
}

func TestWebAgentChatConcurrencyLimitReturnsSSEError(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{err: errWebAgentConcurrencyLimit}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		t.Fatal("runner should not be called when concurrency limit is reached")
		return agent.RuntimeState{}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"concurrency_limit"`)
	assert.Contains(t, body, "data: [DONE]")
}

func TestWebAgentChatStreamsRunStartedStepAndResult(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	limiter := &fakeWebAgentLimiter{}
	h.webAgentLimiter = limiter
	h.webAgentRunner = func(_ context.Context, req webAgentRequest, runID string) (agent.RuntimeState, error) {
		require.Equal(t, "run-1", runID)
		require.Equal(t, "Bearer jwt", req.Credential.Authorization)
		return agent.RuntimeState{
			Steps: []agent.RuntimeStep{{
				Seq: 1,
				Proposal: agent.Proposal{
					Tool:      agent.ToolWebListModels,
					Arguments: []byte(`{"datasource_id":"ds-1"}`),
				},
				Observation: &agent.Observation{Tool: agent.ToolWebListModels, Payload: []byte(`{"models":[]}`)},
			}},
			Terminal: &agent.TerminalResult{
				Kind:  agent.DecisionFinal,
				Final: &agent.FinalResponse{Answer: "done", Confidence: 0.9},
			},
		}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1",
		"conversation_id":"conv-1"
	}`))
	req.Header.Set("Authorization", "Bearer jwt")
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"run_started"`)
	assert.Contains(t, body, `"run_id":"run-1"`)
	assert.Contains(t, body, `"type":"step"`)
	assert.Contains(t, body, `"tool":"list_models"`)
	assert.Contains(t, body, `"type":"result"`)
	assert.Contains(t, body, `"answer":"done"`)
	assert.Contains(t, body, "data: [DONE]")

	insert := findCall(state.calls, "INSERT INTO agent_runs")
	require.NotNil(t, insert)
	require.Len(t, insert.Args, 10)
	assert.Equal(t, "conv-1", insert.Args[0])
	assert.Equal(t, "ds-1", insert.Args[1])
	assert.Equal(t, "user-1", insert.Args[3])
	assert.Equal(t, webAgentMode, insert.Args[6])
	assert.Equal(t, 1, limiter.acquired)
	assert.Equal(t, 1, limiter.released)
	assert.Equal(t, "workspace-1", limiter.workspace)
	assert.Equal(t, "user-1", limiter.user)
	assert.Equal(t, 150*time.Second, limiter.ttl)

	var event map[string]any
	line := strings.TrimPrefix(strings.Split(body, "\n")[0], "data: ")
	require.NoError(t, sonic.Unmarshal([]byte(line), &event))
	assert.Equal(t, "run_started", event["type"])
}

// TestWebAgentChatPersistsStepsOnCompletedRun proves the design doc's
// "Full fidelity persists in agent_steps as today" commitment holds for the
// web agent path too: a completed run's step trace must land in agent_steps
// via ReplaceAgentSteps (matching the legacy job pipeline's persistAgentRun),
// not just the JSON state blob webAgentStateStore.Save writes into
// agent_runs — otherwise a page reload / GET run-by-id after the SSE stream
// ends would show no steps for a web agent run.
func TestWebAgentChatPersistsStepsOnCompletedRun(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	state.execs = []execMock{
		{Pattern: "DELETE FROM agent_steps", RowsAffected: 1},
		{Pattern: "INSERT INTO agent_steps", RowsAffected: 1},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(_ context.Context, _ webAgentRequest, _ string) (agent.RuntimeState, error) {
		return agent.RuntimeState{
			Steps: []agent.RuntimeStep{{
				Seq: 1,
				Proposal: agent.Proposal{
					Tool:      agent.ToolWebListModels,
					Arguments: []byte(`{"datasource_id":"ds-1"}`),
				},
				Observation: &agent.Observation{Tool: agent.ToolWebListModels, Payload: []byte(`{"models":[]}`)},
			}},
			Terminal: &agent.TerminalResult{
				Kind:  agent.DecisionFinal,
				Final: &agent.FinalResponse{Answer: "done", Confidence: 0.9},
			},
		}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	del := findCall(state.calls, "DELETE FROM agent_steps")
	require.NotNil(t, del, "expected the completed run's steps to be replaced")
	require.Len(t, del.Args, 1)
	assert.Equal(t, "run-1", del.Args[0])

	ins := findCall(state.calls, "INSERT INTO agent_steps")
	require.NotNil(t, ins, "expected the single recorded step to be persisted")
	require.Len(t, ins.Args, 7)
	assert.Equal(t, "run-1", ins.Args[0])
	assert.EqualValues(t, 1, ins.Args[1], "seq")
	assert.Equal(t, string(agent.ToolWebListModels), ins.Args[2], "kind")
	assert.Equal(t, "ok", ins.Args[3], "status")
}

// TestWebAgentChatPersistsStepsOnFailedRun proves the failure-path
// counterpart to TestWebAgentChatPersistsStepsOnCompletedRun: a run that
// reaches a graceful Terminal.Failure (max_steps_exceeded, tool_error,
// timeout, max_clarification_rounds_exceeded, ...) after executing at least
// one real tool step must still persist that partial trace to agent_steps —
// otherwise GetAgentRun/RunTracePanel show an empty trace for a failed run
// that made real progress, even though the legacy job pipeline's
// persistAgentRun persists steps from its own structurally-equivalent
// failure branch.
func TestWebAgentChatPersistsStepsOnFailedRun(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	state.execs = []execMock{
		{Pattern: "DELETE FROM agent_steps", RowsAffected: 1},
		{Pattern: "INSERT INTO agent_steps", RowsAffected: 1},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(_ context.Context, _ webAgentRequest, _ string) (agent.RuntimeState, error) {
		return agent.RuntimeState{
			Steps: []agent.RuntimeStep{{
				Seq: 1,
				Proposal: agent.Proposal{
					Tool:      agent.ToolWebListModels,
					Arguments: []byte(`{"datasource_id":"ds-1"}`),
				},
				Observation: &agent.Observation{Tool: agent.ToolWebListModels, Payload: []byte(`{"models":[]}`)},
			}},
			Terminal: &agent.TerminalResult{
				Kind: agent.DecisionFail,
				Failure: &agent.Failure{
					ReasonCode: "max_steps_exceeded",
					Message:    "exceeded max steps",
				},
			},
		}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"max_steps_exceeded"`)
	assert.NotContains(t, body, `"type":"result"`, "a failed run must never report a final result")

	del := findCall(state.calls, "DELETE FROM agent_steps")
	require.NotNil(t, del, "expected the failed run's partial steps to be replaced")
	require.Len(t, del.Args, 1)
	assert.Equal(t, "run-1", del.Args[0])

	ins := findCall(state.calls, "INSERT INTO agent_steps")
	require.NotNil(t, ins, "expected the single recorded step to be persisted despite the run failing")
	require.Len(t, ins.Args, 7)
	assert.Equal(t, "run-1", ins.Args[0])
	assert.EqualValues(t, 1, ins.Args[1], "seq")
	assert.Equal(t, string(agent.ToolWebListModels), ins.Args[2], "kind")
	assert.Equal(t, "ok", ins.Args[3], "status")
}

// TestWebAgentChatClientCancelAbandonsRunCleanly proves the client-abort path
// (T6 item 2): canceling the request context mid-run must unblock the
// in-flight runner via the runtime's existing cancellation semantics, mark
// the run failed, surface a clean SSE error frame (never a hang or a panic),
// and leave no goroutine still running once the handler returns.
func TestWebAgentChatClientCancelAbandonsRunCleanly(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}

	runnerStarted := make(chan struct{})
	runnerDone := make(chan struct{})
	h.webAgentRunner = func(ctx context.Context, _ webAgentRequest, _ string) (agent.RuntimeState, error) {
		defer close(runnerDone)
		close(runnerStarted)
		<-ctx.Done() // simulate an in-flight tool/LLM call honoring cancellation
		return agent.RuntimeState{}, ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = bimw.WithUserID(ctx, "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	rec := httptest.NewRecorder()

	// Cancel from a separate goroutine once the runner is actually in flight
	// — the test goroutine itself is blocked inside h.WebAgentChat below.
	go func() {
		<-runnerStarted
		cancel()
	}()

	h.WebAgentChat(rec, req)

	select {
	case <-runnerDone:
	default:
		t.Fatal("runner goroutine leaked: it did not observe cancellation and return before the handler returned")
	}

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"runtime_error"`)
	assert.Contains(t, body, "context canceled")
	assert.Contains(t, body, "data: [DONE]")
	assert.NotContains(t, body, `"type":"result"`, "a canceled run must never report a final result")

	// The run was abandoned cleanly: no dangling "still running" state — it
	// is durably marked failed, not left running or half-written.
	update := findCall(state.calls, "UPDATE agent_runs")
	require.NotNil(t, update, "cancellation must mark the run failed, not leave it stuck running")
}

// poisonProvider fails the test if either LLM call method is ever invoked —
// used to prove the spend cap short-circuits before any planner/finalizer
// LLM call (T6 item 3).
type poisonProvider struct{ t *testing.T }

func (p poisonProvider) Generate(context.Context, string) (providerpkg.GenerationResult, error) {
	p.t.Fatal("provider.Generate must not be called once the spend cap has rejected the request")
	return providerpkg.GenerationResult{}, nil
}

func (p poisonProvider) GenerateAt(context.Context, string, float64) (providerpkg.GenerationResult, error) {
	p.t.Fatal("provider.GenerateAt must not be called once the spend cap has rejected the request")
	return providerpkg.GenerationResult{}, nil
}

// fakeSpendChecker forces a deterministic Check outcome without a real
// Redis-backed *ai.SpendLimiter, via the spendChecker seam.
type fakeSpendChecker struct {
	checkErr error
	checked  int
	recorded int
}

func (f *fakeSpendChecker) Check(context.Context, string) error {
	f.checked++
	return f.checkErr
}

func (f *fakeSpendChecker) Record(context.Context, string, int) {
	f.recorded++
}

// TestSpendLimitedProviderRejectsBeforeAnyLLMCall proves (T6 item 3) that
// spendLimitedProvider.Generate/GenerateAt consult SpendLimiter.Check first
// and return its error immediately — the wrapped provider's Generate/
// GenerateAt (and thus any actual LLM call) are never reached, and a
// rejected call never records spend.
func TestSpendLimitedProviderRejectsBeforeAnyLLMCall(t *testing.T) {
	limiter := &fakeSpendChecker{checkErr: ai.ErrSpendLimitExceeded}
	provider := spendLimitedProvider{next: poisonProvider{t: t}, limiter: limiter, workspace: "ws-1"}

	_, err := provider.Generate(context.Background(), "prompt")
	require.ErrorIs(t, err, ai.ErrSpendLimitExceeded)
	assert.Equal(t, 1, limiter.checked)

	_, err = provider.GenerateAt(context.Background(), "prompt", 0.2)
	require.ErrorIs(t, err, ai.ErrSpendLimitExceeded)
	assert.Equal(t, 2, limiter.checked)
	assert.Zero(t, limiter.recorded, "a rejected call must never record spend")
}

// TestWebAgentChatSpendCapRejectionSurfacesAsCleanSSEError proves (T6 item 3)
// that a spend-cap rejection propagating out of the run reaches the client
// as a clean SSE error frame — never a 500 or a panic — and never a "result"
// event.
func TestWebAgentChatSpendCapRejectionSurfacesAsCleanSSEError(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		return agent.RuntimeState{}, ai.ErrSpendLimitExceeded
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		h.WebAgentChat(rec, req.WithContext(ctx))
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"runtime_error"`)
	assert.Contains(t, body, ai.ErrSpendLimitExceeded.Error())
	assert.Contains(t, body, "data: [DONE]")
	assert.NotContains(t, body, `"type":"result"`)

	update := findCall(state.calls, "UPDATE agent_runs")
	require.NotNil(t, update, "a spend-cap rejection must still mark the run failed")
}

// TestWebAgentAllowedToolsNarrowsByRole proves T6/T4's role-based web-tool
// allowlist: viewers (and unrecognized/empty roles, fail-closed) get every
// tool except run_logical_query; analysts and admins additionally get it.
func TestWebAgentAllowedToolsNarrowsByRole(t *testing.T) {
	viewerTools := webAgentAllowedTools("viewer")
	assert.NotContains(t, viewerTools, agent.ToolWebRunLogicalQuery)
	assert.Contains(t, viewerTools, agent.ToolWebRunQuestion)
	assert.Contains(t, viewerTools, agent.ToolWebRunSkill)
	assert.Contains(t, viewerTools, agent.ToolWebListDatasources)
	assert.Contains(t, viewerTools, agent.ToolWebListModels)
	assert.Contains(t, viewerTools, agent.ToolWebListSkills)

	analystTools := webAgentAllowedTools("analyst")
	assert.Contains(t, analystTools, agent.ToolWebRunLogicalQuery)
	assert.Contains(t, analystTools, agent.ToolWebRunQuestion)

	adminTools := webAgentAllowedTools("admin")
	assert.Contains(t, adminTools, agent.ToolWebRunLogicalQuery)

	// Fail closed: an empty/unrecognized role gets the viewer-level set.
	unknownTools := webAgentAllowedTools("")
	assert.NotContains(t, unknownTools, agent.ToolWebRunLogicalQuery)
}

// TestWebAgentRunContextAppliesRoleFromAuthContext proves the narrowing is
// actually wired into request handling (T4's "confirm this is wired into
// the /api/agent/chat handler's tool set construction, not just present in
// the policy engine unused"): the run context built from an auth context
// carrying "viewer" excludes run_logical_query, while "analyst" includes it.
func TestWebAgentRunContextAppliesRoleFromAuthContext(t *testing.T) {
	h := newAIHandlerWithRepo(nil)
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	req := webAgentRequest{Message: "show revenue", DatasourceID: "ds-1"}

	viewerCtx := bimw.WithUserRoles(context.Background(), []string{"viewer"})
	viewerRun := h.webAgentRunContext(viewerCtx, req)
	assert.NotContains(t, viewerRun.AllowedTools, agent.ToolWebRunLogicalQuery)

	analystCtx := bimw.WithUserRoles(context.Background(), []string{"analyst"})
	analystRun := h.webAgentRunContext(analystCtx, req)
	assert.Contains(t, analystRun.AllowedTools, agent.ToolWebRunLogicalQuery)

	// No roles at all (e.g. a claim-less identity) fails closed to viewer.
	noRoleRun := h.webAgentRunContext(context.Background(), req)
	assert.NotContains(t, noRoleRun.AllowedTools, agent.ToolWebRunLogicalQuery)
}

// TestStreamAgentStepsDeliversStepsBeforeRunFinishes proves the live event
// sink itself (T6 item 1, isolated from the HTTP handler): a step emitted
// mid-run must reach `send` while the run is still in progress, not only
// once runFn returns. A regression to "buffer steps and emit them all at the
// end" would leave `events` empty until the run completes, and this test's
// require.Eventually would time out.
func TestStreamAgentStepsDeliversStepsBeforeRunFinishes(t *testing.T) {
	var mu sync.Mutex
	var events []string
	send := agentSSESender(func(eventType string, _ any) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, eventType)
	})

	proceed := make(chan struct{})
	//nolint:unparam // signature fixed by streamAgentSteps' runFn parameter type
	runFn := func(_ context.Context, emit func(agent.RuntimeStep)) (agent.RuntimeState, error) {
		emit(agent.RuntimeStep{Seq: 1, Proposal: agent.Proposal{Tool: agent.ToolWebListModels}})
		<-proceed // held open until the test confirms live delivery
		return agent.RuntimeState{Terminal: &agent.TerminalResult{
			Kind: agent.DecisionFinal, Final: &agent.FinalResponse{Answer: "done"},
		}}, nil
	}

	type result struct {
		state agent.RuntimeState
		err   error
	}
	done := make(chan result, 1)
	go func() {
		state, err := streamAgentSteps(context.Background(), send, nil, time.Hour, runFn)
		done <- result{state, err}
	}()

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(events) == 1
	}, time.Second, time.Millisecond, "the step must be delivered to send while the run is still blocked, i.e. live")

	close(proceed)
	res := <-done

	require.NoError(t, res.err)
	require.NotNil(t, res.state.Terminal)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"step"}, events)
}

// TestStreamAgentStepsFiresHeartbeatWhileRunIsSlow proves the 15s heartbeat
// requirement (T6 item 1): while runFn has not yet returned, heartbeat fires
// repeatedly on the configured interval so a slow planner/tool call never
// leaves the SSE connection looking idle.
func TestStreamAgentStepsFiresHeartbeatWhileRunIsSlow(t *testing.T) {
	var heartbeats atomic.Int32
	heartbeat := func() { heartbeats.Add(1) }
	send := agentSSESender(func(string, any) {})

	proceed := make(chan struct{})
	//nolint:unparam // signature fixed by streamAgentSteps' runFn parameter type
	runFn := func(_ context.Context, _ func(agent.RuntimeStep)) (agent.RuntimeState, error) {
		<-proceed
		return agent.RuntimeState{}, nil
	}

	done := make(chan struct{})
	go func() {
		_, _ = streamAgentSteps(context.Background(), send, heartbeat, 5*time.Millisecond, runFn)
		close(done)
	}()

	require.Eventually(t, func() bool {
		return heartbeats.Load() >= 2
	}, time.Second, time.Millisecond)

	close(proceed)
	<-done
}
