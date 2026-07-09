package handlers

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// agentRunRowQueryMock returns the queryMock for GetAgentRun's row query
// (internal/metadata/agent_runs.go's agentRunColumns SELECT), matched by the
// COALESCE(conversation_id) column expression unique to that query
// (LoadAgentRuntimeState's "SELECT runtime_state ..." query targets the same
// table but a different, non-overlapping pattern).
func agentRunRowQueryMock(userID string) queryMock {
	now := time.Now()
	return queryMock{
		Pattern: "coalesce(conversation_id, '')",
		Cols: []string{
			"id", "conversation_id", "datasource_id", "model_id", "user_id",
			"question", "question_hash", "mode", "status", "confidence", "answer",
			"created_at", "updated_at",
		},
		Rows: [][]driver.Value{{
			"run-1", "", "ds-1", "", userID,
			"show revenue", "hash", webAgentMode, metadata.AgentRunStatusWaitingClarification, 0.0, "",
			now, now,
		}},
	}
}

// agentStepsQueryMock returns an empty-steps queryMock for GetAgentRun's
// listAgentSteps call.
func agentStepsQueryMock() queryMock {
	return queryMock{
		Pattern: "from agent_steps",
		Cols:    []string{"seq", "kind", "status", "attempt", "duration_ms", "detail"},
		Rows:    nil,
	}
}

// agentRuntimeStateQueryMock returns the queryMock for
// webAgentStateStore.Load's LoadAgentRuntimeState call.
func agentRuntimeStateQueryMock(raw string) queryMock {
	return queryMock{
		Pattern: "select runtime_state from agent_runs",
		Cols:    []string{"runtime_state"},
		Rows:    [][]driver.Value{{[]byte(raw)}},
	}
}

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
				DurationMs:  412,
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
	assert.Contains(t, body, `"duration_ms":412`,
		"terminal step events must carry the measured tool duration")
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
	viewerRun := h.webAgentRunContext(viewerCtx, req, webAgentResumeInfo{})
	assert.NotContains(t, viewerRun.AllowedTools, agent.ToolWebRunLogicalQuery)

	analystCtx := bimw.WithUserRoles(context.Background(), []string{"analyst"})
	analystRun := h.webAgentRunContext(analystCtx, req, webAgentResumeInfo{})
	assert.Contains(t, analystRun.AllowedTools, agent.ToolWebRunLogicalQuery)

	// No roles at all (e.g. a claim-less identity) fails closed to viewer.
	noRoleRun := h.webAgentRunContext(context.Background(), req, webAgentResumeInfo{})
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

// TestWebAgentChatClarificationRequiredEventCarriesQuestionAndChoices proves
// T8's clarification_required SSE event surfaces the planner's actual
// question and options (design doc: {"question":..., "choices":[{"id":...,
// "label":...}], "allow_free_text": true}), not just a bare run_id.
func TestWebAgentChatClarificationRequiredEventCarriesQuestionAndChoices(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "INSERT INTO agent_runs", Cols: []string{"id"}, Rows: [][]driver.Value{{"run-1"}}},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		return agent.RuntimeState{
			ClarificationRounds: 1,
			PendingClarification: &agent.Clarification{
				Question: "which revenue metric?",
				Options:  []string{"net_revenue", "gross_revenue"},
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
	assert.Contains(t, body, `"type":"clarification_required"`)
	assert.Contains(t, body, `"run_id":"run-1"`)
	assert.Contains(t, body, `"allow_free_text":true`)
	assert.Contains(t, body, `"question":"which revenue metric?"`)
	assert.Contains(t, body, `"id":"net_revenue"`)
	assert.Contains(t, body, `"label":"net_revenue"`)
	assert.Contains(t, body, `"id":"gross_revenue"`)
}

// TestWebAgentChatResumeContinuesRunAfterClarification is T8's pause/resume
// integration test: a first call pauses on a clarification, and a second
// call carrying resume_run_id + clarification_answer continues the *same*
// run (no new INSERT) through to a final result.
func TestWebAgentChatResumeContinuesRunAfterClarification(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{Pattern: "INSERT INTO agent_runs", Cols: []string{"id"}, Rows: [][]driver.Value{{"run-1"}}},
		agentRunRowQueryMock("user-1"),
		agentStepsQueryMock(),
		agentRuntimeStateQueryMock(`{"clarification_rounds":1,"pending_clarification":{"question":"which metric?","options":["net_revenue","gross_revenue"]}}`),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		return agent.RuntimeState{
			ClarificationRounds: 1,
			PendingClarification: &agent.Clarification{
				Question: "which metric?",
				Options:  []string{"net_revenue", "gross_revenue"},
			},
		}, nil
	}

	firstReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"message":"show revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(firstReq.Context(), "user-1")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	firstRec := httptest.NewRecorder()
	h.WebAgentChat(firstRec, firstReq.WithContext(ctx))
	require.Contains(t, firstRec.Body.String(), `"type":"clarification_required"`)

	h.webAgentRunner = func(_ context.Context, req webAgentRequest, runID string) (agent.RuntimeState, error) {
		require.Equal(t, "run-1", runID, "resume must continue the same run, not create a new one")
		require.Equal(t, "net_revenue", req.ClarificationAnswer)
		return agent.RuntimeState{
			Terminal: &agent.TerminalResult{
				Kind:  agent.DecisionFinal,
				Final: &agent.FinalResponse{Answer: "net revenue is 100", Confidence: 0.9},
			},
		}, nil
	}

	resumeReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"resume_run_id":"run-1",
		"clarification_answer":"net_revenue",
		"datasource_id":"ds-1"
	}`))
	resumeCtx := bimw.WithUserID(resumeReq.Context(), "user-1")
	resumeCtx = bimw.WithWorkspaceID(resumeCtx, "workspace-1")
	resumeRec := httptest.NewRecorder()

	h.WebAgentChat(resumeRec, resumeReq.WithContext(resumeCtx))

	require.Equal(t, http.StatusOK, resumeRec.Code, resumeRec.Body.String())
	body := resumeRec.Body.String()
	assert.Contains(t, body, `"type":"run_started"`)
	assert.Contains(t, body, `"run_id":"run-1"`)
	assert.Contains(t, body, `"type":"result"`)
	assert.Contains(t, body, `"answer":"net revenue is 100"`)
	assert.Contains(t, body, "data: [DONE]")

	// Resuming must not have inserted a second agent_runs row.
	var inserts int
	for _, call := range state.calls {
		if strings.Contains(call.Op, "INSERT INTO agent_runs") {
			inserts++
		}
	}
	assert.Equal(t, 1, inserts)
}

// TestWebAgentChatResumeRejectsDifferentUser is T8's "Done when" security
// case: a caller resuming someone else's run gets a generic not-found error,
// not a distinguishable "forbidden" that would leak the run's existence.
func TestWebAgentChatResumeRejectsDifferentUser(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		agentRunRowQueryMock("owner-user"),
		agentStepsQueryMock(),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))
	h.deps.Config.WebAgent = config.WebAgentConfig{Enabled: true, MaxSteps: 6, MaxClarificationRounds: 2}
	h.webAgentLimiter = &fakeWebAgentLimiter{}
	h.webAgentRunner = func(context.Context, webAgentRequest, string) (agent.RuntimeState, error) {
		t.Fatal("runner must not be invoked for a rejected resume")
		return agent.RuntimeState{}, nil
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/agent/chat", strings.NewReader(`{
		"resume_run_id":"run-1",
		"clarification_answer":"net_revenue",
		"datasource_id":"ds-1"
	}`))
	ctx := bimw.WithUserID(req.Context(), "attacker-user")
	ctx = bimw.WithWorkspaceID(ctx, "workspace-1")
	rec := httptest.NewRecorder()

	h.WebAgentChat(rec, req.WithContext(ctx))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, `"type":"error"`)
	assert.Contains(t, body, `"code":"not_found"`)
	assert.NotContains(t, body, "run_started")
}

// TestResumeWebAgentRunLoadsPendingClarificationForPlanner is a focused unit
// test on resumeWebAgentRun itself: it must resolve the persisted
// PendingClarification's question plus the request's answer into a
// ClarificationExchange the planner prompt can render, AND (T8 review
// finding 1) return the run's ORIGINAL question — not the clarification
// answer — for the caller to thread into RunContext.Question.
func TestResumeWebAgentRunLoadsPendingClarificationForPlanner(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		agentRunRowQueryMock("user-1"),
		agentStepsQueryMock(),
		agentRuntimeStateQueryMock(`{"pending_clarification":{"question":"which metric?","options":["net_revenue","gross_revenue"]}}`),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	runID, resume, err := h.resumeWebAgentRun(ctx, webAgentRequest{
		ResumeRunID:         "run-1",
		DatasourceID:        "ds-1",
		ClarificationAnswer: "net_revenue",
	})

	require.NoError(t, err)
	assert.Equal(t, "run-1", runID)
	// agentRunRowQueryMock persists the original question as "show revenue" —
	// distinct from the clarification answer "net_revenue" below, so this
	// assertion fails if OriginalQuestion were ever recomputed from the
	// current request instead of the persisted run row.
	assert.Equal(t, "show revenue", resume.OriginalQuestion)
	require.Len(t, resume.ClarificationHistory, 1)
	assert.Equal(t, "which metric?", resume.ClarificationHistory[0].Question)
	assert.Equal(t, "net_revenue", resume.ClarificationHistory[0].Answer)
}

// TestResumeWebAgentRunAccumulatesClarificationHistoryAcrossTwoRounds is the
// genuine 2-round data-flow test T8's review demanded: round 1's persisted
// history (already containing Q1/A1, as if this were the second resume in a
// real flow) plus round 2's own pending question/answer must BOTH surface in
// the ClarificationHistory resumeWebAgentRun returns — round 1's resolution
// must not be lost by the time round 2 resumes.
func TestResumeWebAgentRunAccumulatesClarificationHistoryAcrossTwoRounds(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		agentRunRowQueryMock("user-1"),
		agentStepsQueryMock(),
		agentRuntimeStateQueryMock(`{
			"clarification_history":[{"Question":"which metric?","Answer":"net_revenue"}],
			"pending_clarification":{"question":"which quarter?","options":["Q1","Q2"]}
		}`),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	runID, resume, err := h.resumeWebAgentRun(ctx, webAgentRequest{
		ResumeRunID:         "run-1",
		DatasourceID:        "ds-1",
		ClarificationAnswer: "Q2",
	})

	require.NoError(t, err)
	assert.Equal(t, "run-1", runID)
	assert.Equal(t, "show revenue", resume.OriginalQuestion)
	require.Len(t, resume.ClarificationHistory, 2, "round 1's Q1/A1 must survive alongside round 2's Q2/A2")
	assert.Equal(t, "which metric?", resume.ClarificationHistory[0].Question)
	assert.Equal(t, "net_revenue", resume.ClarificationHistory[0].Answer)
	assert.Equal(t, "which quarter?", resume.ClarificationHistory[1].Question)
	assert.Equal(t, "Q2", resume.ClarificationHistory[1].Answer)

	// Chaining resumeWebAgentRun's output into webAgentRunContext (exactly as
	// WebAgentChat does in production) must produce a RunContext whose
	// Question is the ORIGINAL question, and whose ClarificationHistory
	// carries both resolved rounds — not just the latest one.
	runCtx := h.webAgentRunContext(ctx, webAgentRequest{
		DatasourceID:        "ds-1",
		ClarificationAnswer: "Q2",
	}, resume)
	assert.Equal(t, "show revenue", runCtx.Question)
	require.Len(t, runCtx.ClarificationHistory, 2)
	assert.Equal(t, "Q2", runCtx.ClarificationHistory[1].Answer)
}

// TestResumeWebAgentRunRejectsMismatchedDatasource proves the extra
// defense-in-depth check: even with the correct owner, a resume request
// naming a different datasource than the run's original one is rejected the
// same generic way as a different user.
func TestResumeWebAgentRunRejectsMismatchedDatasource(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		agentRunRowQueryMock("user-1"),
		agentStepsQueryMock(),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	_, _, err := h.resumeWebAgentRun(ctx, webAgentRequest{
		ResumeRunID:  "run-1",
		DatasourceID: "ds-other",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, errWebAgentResumeForbidden))
}

// TestResumeWebAgentRunRejectsAlreadyTerminalRun proves resuming a run that
// already reached a terminal result surfaces a distinct, non-security error
// rather than silently re-running (or masquerading as "not found").
func TestResumeWebAgentRunRejectsAlreadyTerminalRun(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		agentRunRowQueryMock("user-1"),
		agentStepsQueryMock(),
		agentRuntimeStateQueryMock(`{"terminal":{"kind":"final","final":{"answer":"done","confidence":1}}}`),
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	_, _, err := h.resumeWebAgentRun(ctx, webAgentRequest{ResumeRunID: "run-1", DatasourceID: "ds-1"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, agent.ErrRunAlreadyTerminal))
}

// TestCreateWebAgentRunFallsBackWithoutConversationLinkage reproduces the
// production incident where every agent run failed with "could not create
// agent run": the client is the single conversation writer (snapshot +
// idempotency flow), so the client-supplied conversation id can reference an
// ai_conversations row that doesn't exist yet server-side, and the
// agent_runs.conversation_id FK insert then fails with SQLSTATE 23503. The
// linkage is best-effort — the handler must retry without it, not fail the
// whole run.
func TestCreateWebAgentRunFallsBackWithoutConversationLinkage(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "agent_runs_conversation_id_fkey",
			},
			Once: true,
		},
		{
			Pattern: "INSERT INTO agent_runs",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"run-1"}},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	id, err := h.createWebAgentRun(ctx, webAgentRequest{
		Message:        "show revenue",
		DatasourceID:   "ds-1",
		ConversationID: "conv_client_local_id",
	})

	require.NoError(t, err)
	assert.Equal(t, "run-1", id)

	var inserts []mockCall
	for _, call := range state.calls {
		if strings.Contains(call.Op, "INSERT INTO agent_runs") {
			inserts = append(inserts, call)
		}
	}
	require.Len(t, inserts, 2, "expected the failed insert plus one linkage-free retry")
	assert.Equal(t, "conv_client_local_id", inserts[0].Args[0])
	assert.Nil(t, inserts[1].Args[0], "retry must drop the conversation linkage (NULL)")
}

// TestCreateWebAgentRunDoesNotRetryOtherErrors proves the FK fallback is
// scoped to exactly the conversation-linkage constraint: any other insert
// failure (here a different FK) still fails the run.
func TestCreateWebAgentRunDoesNotRetryOtherErrors(t *testing.T) {
	db, state := setupMockDB(t)
	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO agent_runs",
			Err: &pgconn.PgError{
				Code:           "23503",
				ConstraintName: "agent_runs_datasource_id_fkey",
			},
		},
	}
	h := newAIHandlerWithRepo(metadata.NewRepository(db))

	ctx := bimw.WithUserID(context.Background(), "user-1")
	_, err := h.createWebAgentRun(ctx, webAgentRequest{
		Message:        "show revenue",
		DatasourceID:   "ds-missing",
		ConversationID: "conv-1",
	})

	require.Error(t, err)
	var inserts int
	for _, call := range state.calls {
		if strings.Contains(call.Op, "INSERT INTO agent_runs") {
			inserts++
		}
	}
	assert.Equal(t, 1, inserts, "non-conversation FK errors must not trigger a retry")
}
