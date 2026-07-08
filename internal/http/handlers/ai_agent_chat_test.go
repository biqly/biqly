package handlers

import (
	"context"
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/agent"
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
