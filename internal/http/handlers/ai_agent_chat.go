package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/agent"
	"github.com/biqly/biqly/internal/ai"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/biqly/biqly/internal/toolcontract"
)

const webAgentMode = "web"

var (
	errWebAgentConcurrencyLimit       = errors.New("web agent concurrency limit reached")
	errWebAgentConcurrencyUnavailable = errors.New("web agent concurrency guard unavailable")
)

type webAgentRequest struct {
	Message             string                  `json:"message"`
	ConversationID      string                  `json:"conversation_id,omitempty"`
	DatasourceID        string                  `json:"datasource_id,omitempty"`
	PriorTurns          []priorTurnPayload      `json:"prior_turns,omitempty"`
	ResumeRunID         string                  `json:"resume_run_id,omitempty"`
	ClarificationAnswer string                  `json:"clarification_answer,omitempty"`
	Credential          toolcontract.Credential `json:"-"`
}

type webAgentRunFunc func(context.Context, webAgentRequest, string) (agent.RuntimeState, error)

type webAgentConcurrencyLimiter interface {
	Acquire(ctx context.Context, workspaceID, userID string, ttl time.Duration) (func(context.Context), error)
}

type redisWebAgentLimiter struct {
	client *redis.Client
	max    int
}

// SetWebAgentDispatcher wires the governed /api/* dispatch target used by
// POST /api/agent/chat. Tests may leave it nil and inject webAgentRunner.
func (h *AIHandler) SetWebAgentDispatcher(d toolcontract.Dispatcher) {
	h.webAgentDispatcher = d
}

// WebAgentChat streams a bounded web-agent run over SSE.
func (h *AIHandler) WebAgentChat(w http.ResponseWriter, r *http.Request) {
	cfg := h.webAgentConfig()
	if !cfg.Enabled {
		writeError(w, http.StatusNotFound, "web agent is disabled")
		return
	}
	workspaceID := bimw.WorkspaceID(r.Context())
	if !workspaceAllowed(workspaceID, cfg.WorkspaceAllowlist) {
		writeError(w, http.StatusForbidden, "web agent is not enabled for this workspace")
		return
	}

	send, heartbeat, ok := newAgentSSESender(r.Context(), w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	var req webAgentRequest
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		sendAgentError(send, "bad_request", "invalid JSON request")
		sendAgentDone(send)
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	req.DatasourceID = strings.TrimSpace(req.DatasourceID)
	req.ResumeRunID = strings.TrimSpace(req.ResumeRunID)
	req.ClarificationAnswer = strings.TrimSpace(req.ClarificationAnswer)
	req.Credential = requestCredential(r)
	if req.Message == "" && req.ClarificationAnswer == "" {
		sendAgentError(send, "bad_request", "message is required")
		sendAgentDone(send)
		return
	}
	if req.DatasourceID == "" {
		sendAgentError(send, "bad_request", "datasource_id is required")
		sendAgentDone(send)
		return
	}
	if h.webAgentRunner == nil && h.webAgentDispatcher == nil {
		sendAgentError(send, "runtime_unavailable", "web agent runtime is not configured")
		sendAgentDone(send)
		return
	}
	release, err := h.acquireWebAgentSlot(r.Context(), cfg)
	if err != nil {
		if errors.Is(err, errWebAgentConcurrencyLimit) {
			sendAgentError(send, "concurrency_limit", "too many active web agent runs")
		} else {
			sendAgentError(send, "concurrency_unavailable", "web agent concurrency guard is unavailable")
		}
		sendAgentDone(send)
		return
	}
	defer release(context.WithoutCancel(r.Context()))

	runID, err := h.createWebAgentRun(r.Context(), req)
	if err != nil {
		slog.WarnContext(r.Context(), "create web agent run failed", "error", err)
		sendAgentError(send, "run_create_failed", "could not create agent run")
		sendAgentDone(send)
		return
	}
	sendAgentEvent(send, "run_started", map[string]any{"run_id": runID})

	state, err := h.runWebAgent(r.Context(), req, runID, send, heartbeat)
	if err != nil {
		// context.WithoutCancel: a client abort (T6 item 2) cancels
		// r.Context(), but the run must still be durably marked failed
		// rather than left stuck "running" forever — same reasoning as the
		// concurrency-slot release above.
		h.failWebAgentRun(context.WithoutCancel(r.Context()), runID, err)
		sendAgentError(send, "runtime_error", err.Error())
		sendAgentDone(send)
		return
	}
	switch {
	case state.Terminal != nil && state.Terminal.Final != nil:
		sendAgentEvent(send, "result", map[string]any{
			"run_id":     runID,
			"answer":     state.Terminal.Final.Answer,
			"confidence": state.Terminal.Final.Confidence,
			"steps":      webAgentStepEvents(state.Steps),
		})
	case state.Terminal != nil && state.Terminal.Failure != nil:
		sendAgentError(send, state.Terminal.Failure.ReasonCode, state.Terminal.Failure.Message)
	default:
		sendAgentEvent(send, "clarification_required", map[string]any{
			"run_id":          runID,
			"allow_free_text": true,
		})
	}
	sendAgentDone(send)
}

func (h *AIHandler) webAgentConfig() config.WebAgentConfig {
	if h == nil || h.deps == nil || h.deps.Config == nil {
		return config.WebAgentConfig{}
	}
	return normalizeWebAgentConfig(h.deps.Config.WebAgent)
}

func (h *AIHandler) acquireWebAgentSlot(ctx context.Context, cfg config.WebAgentConfig) (func(context.Context), error) {
	if h == nil || h.webAgentLimiter == nil {
		return nil, errWebAgentConcurrencyUnavailable
	}
	ttl := max(cfg.Timeout+30*time.Second, time.Minute)
	return h.webAgentLimiter.Acquire(ctx, bimw.WorkspaceID(ctx), bimw.UserID(ctx), ttl)
}

func normalizeWebAgentConfig(cfg config.WebAgentConfig) config.WebAgentConfig {
	if cfg.MaxSteps == 0 {
		cfg.MaxSteps = 6
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	return cfg
}

func workspaceAllowed(workspaceID string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	return slices.Contains(allowlist, workspaceID)
}

func (h *AIHandler) createWebAgentRun(ctx context.Context, req webAgentRequest) (string, error) {
	if h == nil || h.deps == nil || h.deps.MetaRepo == nil {
		return "", errors.New("metadata repository is not configured")
	}
	question := firstNonEmpty(req.Message, req.ClarificationAnswer)
	return h.deps.MetaRepo.CreateAgentRun(ctx, metadata.AgentRunInsert{
		ConversationID: req.ConversationID,
		DatasourceID:   req.DatasourceID,
		UserID:         bimw.UserID(ctx),
		Question:       question,
		QuestionHash:   metadata.QuestionHash(question),
		Mode:           webAgentMode,
		Status:         metadata.AgentRunStatusRunning,
	})
}

func (h *AIHandler) failWebAgentRun(ctx context.Context, runID string, cause error) {
	if h == nil || h.deps == nil || h.deps.MetaRepo == nil || runID == "" || cause == nil {
		return
	}
	if err := h.deps.MetaRepo.UpdateAgentRunStatus(ctx, runID, metadata.AgentRunStatusFailed, 0, cause.Error()); err != nil {
		slog.WarnContext(ctx, "mark web agent run failed", "run_id", runID, "error", err)
	}
}

// webAgentHeartbeatInterval matches the design doc's SSE heartbeat cadence:
// a comment frame every 15s keeps the gateway (and any other HTTP
// intermediary) from treating a long planner/tool call as an idle
// connection and closing it early — the same 1800s HTTPRoute timeout only
// covers total request duration, not idle gaps.
const webAgentHeartbeatInterval = 15 * time.Second

// runWebAgent executes one run — the h.webAgentRunner test seam or, in
// production, the real agent.Runtime — and streams its steps live over send
// as they happen (T6 item 1) via streamAgentSteps, instead of buffering them
// until the run finishes.
func (h *AIHandler) runWebAgent(ctx context.Context, req webAgentRequest, runID string, send agentSSESender, heartbeat func()) (agent.RuntimeState, error) {
	if h.webAgentRunner != nil {
		runner := h.webAgentRunner
		return streamAgentSteps(ctx, send, heartbeat, webAgentHeartbeatInterval,
			func(ctx context.Context, emit func(agent.RuntimeStep)) (agent.RuntimeState, error) {
				state, err := runner(ctx, req, runID)
				for _, step := range state.Steps {
					emit(step)
				}
				return state, err
			})
	}
	if h.webAgentDispatcher == nil {
		return agent.RuntimeState{}, errors.New("web agent dispatcher is not configured")
	}
	if h.deps == nil || h.deps.AIProviderStore == nil || h.deps.MetaRepo == nil {
		return agent.RuntimeState{}, errors.New("web agent dependencies are not configured")
	}

	cfg := normalizeWebAgentConfig(h.deps.Config.WebAgent)
	provider := spendLimitedProvider{
		next:      ai.NewPurposeProvider(h.deps.AIProviderStore, ai.PurposeAgent, nil, nil),
		limiter:   h.deps.SpendLimiter,
		workspace: bimw.WorkspaceID(ctx),
	}
	planner := agent.NewProviderPlanner(provider)
	webTools := agent.NewWebTools(h.webAgentDispatcher, req.Credential)
	registry := agent.NewRegistry(&agent.PolicyEngine{}, webTools.All()...)
	return streamAgentSteps(ctx, send, heartbeat, webAgentHeartbeatInterval,
		func(ctx context.Context, emit func(agent.RuntimeStep)) (agent.RuntimeState, error) {
			rt := agent.NewRuntime(planner, registry, &webAgentStateStore{repo: h.deps.MetaRepo})
			rt.SetStepHook(emit)
			ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()
			return rt.Run(ctx, h.webAgentRunContext(ctx, req), runID)
		})
}

// streamAgentSteps runs runFn on a background goroutine and relays every
// step it emits to send as a live "step" SSE event, in order, from the
// calling goroutine (the only goroutine that ever writes to send — runFn's
// goroutine only ever pushes to the internal channel). While runFn is still
// running, heartbeat fires every heartbeatEvery so a slow planner/tool call
// never leaves the connection looking idle. runFn's ctx is exactly the ctx
// passed in here: an already-canceled or later-canceled context propagates
// straight through, relying on the runtime's own cancellation semantics to
// make runFn return promptly — streamAgentSteps itself does not time out.
func streamAgentSteps(
	ctx context.Context,
	send agentSSESender,
	heartbeat func(),
	heartbeatEvery time.Duration,
	runFn func(ctx context.Context, emit func(agent.RuntimeStep)) (agent.RuntimeState, error),
) (agent.RuntimeState, error) {
	type runResult struct {
		state agent.RuntimeState
		err   error
	}
	stepCh := make(chan agent.RuntimeStep, 32)
	resultCh := make(chan runResult, 1)
	go func() {
		state, err := runFn(ctx, func(step agent.RuntimeStep) {
			select {
			case stepCh <- step:
			case <-ctx.Done():
			}
		})
		close(stepCh)
		resultCh <- runResult{state: state, err: err}
	}()

	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case step, ok := <-stepCh:
			if !ok {
				// runFn returned and closed stepCh: the result is already
				// waiting (send happens strictly after close, in program
				// order, on the same goroutine).
				res := <-resultCh
				return res.state, res.err
			}
			sendAgentEvent(send, "step", webAgentStepEvent(step))
		case <-ticker.C:
			if heartbeat != nil {
				heartbeat()
			}
		}
	}
}

func (h *AIHandler) webAgentRunContext(ctx context.Context, req webAgentRequest) agent.RunContext {
	cfg := normalizeWebAgentConfig(h.deps.Config.WebAgent)
	return agent.RunContext{
		TenantID:               bimw.WorkspaceID(ctx),
		UserID:                 bimw.UserID(ctx),
		DatasourceID:           req.DatasourceID,
		Question:               firstNonEmpty(req.Message, req.ClarificationAnswer),
		PriorTurns:             agentPriorTurns(req.PriorTurns),
		AllowedTools:           webAgentAllowedTools(pii.PrimaryRole(bimw.UserRoles(ctx))),
		RetryBudget:            webAgentRetryBudget(),
		DeploymentMode:         h.deps.Config.DeploymentMode,
		Timeout:                cfg.Timeout,
		MaxSteps:               cfg.MaxSteps,
		MaxClarificationRounds: cfg.MaxClarificationRounds,
		ExternalEgressTools:    nil,
		MaxRows:                100,
		Credential:             req.Credential,
	}
}

func requestCredential(r *http.Request) toolcontract.Credential {
	return toolcontract.Credential{
		Authorization: strings.TrimSpace(r.Header.Get("Authorization")),
		APIKey:        strings.TrimSpace(r.Header.Get("X-API-Key")),
	}
}

func agentPriorTurns(in []priorTurnPayload) []agent.PriorTurn {
	if len(in) == 0 {
		return nil
	}
	out := make([]agent.PriorTurn, 0, len(in))
	for _, turn := range in {
		out = append(out, agent.PriorTurn{
			User:          turn.Question,
			ResultSummary: turn.ResultSummary,
		})
	}
	return out
}

// webAgentAllowedTools returns the role-narrowed tool allowlist (T6/T4:
// "role-based allowlist (viewer vs analyst) from auth context"). This is a
// cheap first gate mirroring /api/* RBAC — the HTTP middleware chain the
// tools loop back through remains the authority — so an unrecognized or
// empty role fails closed to the viewer set, same as pii.PrimaryRole's
// documented default.
func webAgentAllowedTools(role string) []agent.ToolName {
	tools := []agent.ToolName{
		agent.ToolWebListDatasources,
		agent.ToolWebListModels,
		agent.ToolWebRunQuestion,
		agent.ToolWebListSkills,
		agent.ToolWebRunSkill,
	}
	if role == pii.RoleAnalyst || role == pii.RoleAdmin {
		tools = append(tools, agent.ToolWebRunLogicalQuery)
	}
	return tools
}

func webAgentRetryBudget() map[agent.ToolName]int {
	return map[agent.ToolName]int{
		agent.ToolWebListDatasources: 2,
		agent.ToolWebListModels:      2,
		agent.ToolWebRunQuestion:     3,
		agent.ToolWebRunLogicalQuery: 2,
		agent.ToolWebListSkills:      2,
		agent.ToolWebRunSkill:        2,
	}
}

type agentSSESender func(string, any)

// newAgentSSESender returns the SSE event writer plus a heartbeat writer
// that emits a bare SSE comment frame (a line starting with ':' — ignored by
// every conforming SSE/EventSource parser, so it never becomes a spurious
// "step"/"result" the client has to filter out) to keep the connection alive
// during a long planner/tool call.
func newAgentSSESender(ctx context.Context, w http.ResponseWriter) (send agentSSESender, heartbeat func(), ok bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, nil, false
	}
	send = func(eventType string, payload any) {
		if eventType == "done" {
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		body := map[string]any{"type": eventType}
		if payloadMap, ok := payload.(map[string]any); ok {
			for k, v := range payloadMap {
				body[k] = v
			}
		} else if payload != nil {
			body["payload"] = payload
		}
		raw, err := sonic.Marshal(body)
		if err != nil {
			slog.ErrorContext(ctx, "agent stream marshal", "error", err)
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		flusher.Flush()
	}
	heartbeat = func() {
		_, _ = fmt.Fprint(w, ": heartbeat\n\n")
		flusher.Flush()
	}
	return send, heartbeat, true
}

func sendAgentEvent(send agentSSESender, eventType string, payload map[string]any) {
	send(eventType, payload)
}

func sendAgentError(send agentSSESender, code, message string) {
	send("error", map[string]any{"code": code, "message": message})
}

func sendAgentDone(send agentSSESender) {
	send("done", nil)
}

func webAgentStepEvents(steps []agent.RuntimeStep) []map[string]any {
	out := make([]map[string]any, 0, len(steps))
	for _, step := range steps {
		out = append(out, webAgentStepEvent(step))
	}
	return out
}

func webAgentStepEvent(step agent.RuntimeStep) map[string]any {
	status := "completed"
	switch {
	case step.DeniedReason != "":
		status = "denied"
	case step.Error != "":
		status = "failed"
	case step.Observation == nil:
		status = "started"
	}
	return map[string]any{
		"seq":    step.Seq,
		"kind":   "tool_call_" + status,
		"tool":   step.Proposal.Tool,
		"status": status,
	}
}

type webAgentStateStore struct {
	repo *metadata.Repository
}

// spendChecker is the subset of *ai.SpendLimiter that spendLimitedProvider
// needs. Narrowing to an interface (rather than depending on the concrete
// Redis-backed type directly) lets tests inject a fake that forces a
// rejection deterministically, without a real Redis instance.
type spendChecker interface {
	Check(ctx context.Context, workspace string) error
	Record(ctx context.Context, workspace string, tokens int)
}

type spendLimitedProvider struct {
	next      providerpkg.Provider
	limiter   spendChecker
	workspace string
}

func (p spendLimitedProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	if p.limiter != nil {
		if err := p.limiter.Check(ctx, p.workspace); err != nil {
			return providerpkg.GenerationResult{}, err
		}
	}
	result, err := p.next.Generate(ctx, prompt)
	p.record(ctx, result)
	return result, err
}

func (p spendLimitedProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (providerpkg.GenerationResult, error) {
	if p.limiter != nil {
		if err := p.limiter.Check(ctx, p.workspace); err != nil {
			return providerpkg.GenerationResult{}, err
		}
	}
	result, err := p.next.GenerateAt(ctx, prompt, temperature)
	p.record(ctx, result)
	return result, err
}

func (p spendLimitedProvider) record(ctx context.Context, result providerpkg.GenerationResult) {
	if p.limiter == nil || result.Usage == nil {
		return
	}
	p.limiter.Record(ctx, p.workspace, result.Usage.Total)
}

func (s *webAgentStateStore) Save(ctx context.Context, runID string, state agent.RuntimeState) error {
	raw, err := agent.MarshalState(state)
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	if state.Terminal != nil {
		status := metadata.AgentRunStatusCompleted
		var confidence float64
		var answer string
		switch {
		case state.Terminal.Final != nil:
			confidence = state.Terminal.Final.Confidence
			answer = state.Terminal.Final.Answer
		case state.Terminal.Failure != nil:
			status = metadata.AgentRunStatusFailed
			answer = state.Terminal.Failure.Message
		}
		return s.repo.CompleteAgentRunTerminal(ctx, runID, status, confidence, answer, raw)
	}
	if state.QueryExecuteStarted {
		if err := s.repo.MarkAgentRunQueryExecuteStarted(ctx, runID); err != nil {
			return err
		}
	}
	return s.repo.SaveAgentRuntimeState(ctx, runID, raw)
}

func (l redisWebAgentLimiter) Acquire(ctx context.Context, workspaceID, userID string, ttl time.Duration) (func(context.Context), error) {
	if l.client == nil || workspaceID == "" || userID == "" {
		return nil, errWebAgentConcurrencyUnavailable
	}
	limit := l.max
	if limit <= 0 {
		limit = 2
	}
	key := fmt.Sprintf("web_agent:active:%s:%s", workspaceID, userID)
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return nil, errWebAgentConcurrencyUnavailable
	}
	if count == 1 {
		if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
			if decErr := l.client.Decr(ctx, key).Err(); decErr != nil {
				slog.WarnContext(ctx, "web agent concurrency rollback failed", "key", key, "error", decErr)
			}
			return nil, errWebAgentConcurrencyUnavailable
		}
	}
	if count > int64(limit) {
		if err := l.client.Decr(ctx, key).Err(); err != nil {
			slog.WarnContext(ctx, "web agent concurrency rollback failed", "key", key, "error", err)
		}
		return nil, errWebAgentConcurrencyLimit
	}
	return func(ctx context.Context) {
		if err := l.client.Decr(ctx, key).Err(); err != nil {
			slog.WarnContext(ctx, "web agent concurrency release failed", "key", key, "error", err)
		}
	}, nil
}

func (s *webAgentStateStore) Load(ctx context.Context, runID string) (agent.RuntimeState, bool, error) {
	raw, err := s.repo.LoadAgentRuntimeState(ctx, runID)
	if err != nil {
		if errors.Is(err, metadata.ErrAgentRunNotFound) {
			return agent.RuntimeState{}, false, nil
		}
		return agent.RuntimeState{}, false, err
	}
	state, err := agent.UnmarshalState(raw)
	if err != nil {
		return agent.RuntimeState{}, false, err
	}
	return state, true, nil
}
