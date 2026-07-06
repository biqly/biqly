package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/platform/observability"
)

// RuntimeStep is one planner decision and its outcome, persisted so the run
// can resume from exactly this point after a crash or NATS redelivery.
type RuntimeStep struct {
	Seq          int          `json:"seq"`
	Proposal     Proposal     `json:"proposal"`
	Observation  *Observation `json:"observation,omitempty"`
	DeniedReason string       `json:"denied_reason,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// TerminalResult is a run's immutable final outcome: DecisionFinal or
// DecisionFail, never anything else.
type TerminalResult struct {
	Kind    DecisionKind   `json:"kind"`
	Final   *FinalResponse `json:"final,omitempty"`
	Failure *Failure       `json:"failure,omitempty"`
}

// RuntimeState is the durable, resumable snapshot of one run's progress.
// A NATS-redelivered job resumes from exactly this state instead of
// restarting the run from step zero.
type RuntimeState struct {
	Steps               []RuntimeStep   `json:"steps,omitempty"`
	ClarificationRounds int             `json:"clarification_rounds"`
	QueryExecuteStarted bool            `json:"query_execute_started"`
	Terminal            *TerminalResult `json:"terminal,omitempty"`
}

// toolStepCount returns how many steps in state proposed a tool call — the
// count MaxSteps bounds. Steps the planner never gets to retry (policy
// denials) still consume a step: the planner must propose something else,
// not spin on the same denied call for free.
func (s RuntimeState) toolStepCount() int {
	return len(s.Steps)
}

func (s RuntimeState) nextSeq() int {
	return len(s.Steps) + 1
}

// StateStore persists RuntimeState between steps so a crash or NATS
// redelivery resumes instead of restarting. Implementations must persist
// synchronously — Run() calls Save before and after every external call
// specifically so a crash mid-call is recoverable from the last good state.
type StateStore interface {
	Save(ctx context.Context, runID string, state RuntimeState) error
	Load(ctx context.Context, runID string) (RuntimeState, bool, error)
}

// ErrRunAlreadyTerminal is returned by Run when the persisted state already
// reached a terminal result — terminal results are immutable, so Run does
// not re-plan; it just returns the existing state.
var ErrRunAlreadyTerminal = errors.New("agent run already reached a terminal state")

// Runtime executes one agent run's bounded planner/tool loop.
type Runtime struct {
	planner  Planner
	registry *Registry
	store    StateStore
	metrics  *observability.Metrics
}

// NewRuntime builds a Runtime backed by planner, registry, and store.
func NewRuntime(planner Planner, registry *Registry, store StateStore) *Runtime {
	return &Runtime{planner: planner, registry: registry, store: store}
}

// SetMetrics wires m as the destination for this Runtime's run/step metrics.
// Optional: a nil (or never-set) metrics recorder is a no-op on every
// Record* call, matching internal/platform/observability's nil-receiver
// convention.
func (rt *Runtime) SetMetrics(m *observability.Metrics) {
	rt.metrics = m
}

// Run executes (or resumes) runID's bounded planning loop until it reaches a
// terminal state, a clarification is needed, ctx is canceled, or the run's
// deadline elapses. It always returns the latest persisted RuntimeState,
// even on error, so a caller can inspect exactly how far the run got.
//
// Step sequence numbers are allocated from len(state.Steps): the (run_id,
// seq) unique constraint on agent_steps is what makes concurrent/duplicate
// allocation attempts fail loudly instead of silently colliding; a real
// StateStore.Save should write steps inside the same transaction that
// enforces it.
func (rt *Runtime) Run(ctx context.Context, run RunContext, runID string) (RuntimeState, error) {
	state, ok, err := rt.store.Load(ctx, runID)
	if err != nil {
		return RuntimeState{}, fmt.Errorf("load runtime state: %w", err)
	}
	if !ok {
		state = RuntimeState{}
	}
	if state.Terminal != nil {
		return state, ErrRunAlreadyTerminal
	}

	var deadline time.Time
	if run.Timeout > 0 {
		deadline = time.Now().Add(run.Timeout)
	}

	for {
		next, done, err := rt.step(ctx, run, runID, state, deadline)
		state = next
		if done || err != nil {
			return state, err
		}
	}
}

// step runs one planning iteration: bounds/cancellation checks, one planner
// decision (with a single bounded retry), and dispatch of that decision.
// done reports whether Run should stop looping — either a terminal result
// was just persisted, or a clarification paused the run.
func (rt *Runtime) step(
	ctx context.Context, run RunContext, runID string, state RuntimeState, deadline time.Time,
) (RuntimeState, bool, error) {
	if err := ctx.Err(); err != nil {
		final, ferr := rt.abandonOrFail(ctx, runID, state, "context_canceled", err)
		return final, true, ferr
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		final, err := rt.finalizeFail(ctx, runID, state, "timeout", "run exceeded its time budget")
		return final, true, err
	}

	decision, derr := rt.planner.Decide(ctx, run, state.Steps)
	if derr != nil {
		// One bounded retry: a single planner hiccup (bad decode, transient
		// provider error) should not fail the whole run outright.
		decision, derr = rt.planner.Decide(ctx, run, state.Steps)
	}
	if derr != nil {
		final, ferr := rt.abandonOrFail(ctx, runID, state, "planner_error", derr)
		return final, true, ferr
	}

	switch decision.Kind {
	case DecisionClarification:
		return rt.runClarificationStep(ctx, run, runID, state)
	case DecisionFinal:
		final, err := rt.finalizeOK(ctx, runID, state, decision.Final)
		return final, true, err
	case DecisionFail:
		state.Terminal = &TerminalResult{Kind: DecisionFail, Failure: decision.Failure}
		if err := rt.store.Save(ctx, runID, state); err != nil {
			return state, true, fmt.Errorf("save runtime state: %w", err)
		}
		rt.metrics.RecordAgentRunTerminal("failed")
		if decision.Failure != nil {
			// decision.Failure.ReasonCode is planner (LLM) output, not one of
			// this runtime's fixed reason codes — RecordAgentTerminalFailure
			// bounds it via BoundLabel, so an arbitrary planner string can
			// never create an unbounded label series.
			rt.metrics.RecordAgentTerminalFailure(decision.Failure.ReasonCode)
		}
		return state, true, nil
	case DecisionTool:
		return rt.runToolStep(ctx, run, runID, state, decision.Proposal)
	default:
		final, err := rt.finalizeFail(ctx, runID, state, "invalid_decision_kind",
			fmt.Sprintf("planner returned unknown decision kind %q", decision.Kind))
		return final, true, err
	}
}

func (rt *Runtime) runClarificationStep(ctx context.Context, run RunContext, runID string, state RuntimeState) (RuntimeState, bool, error) {
	state.ClarificationRounds++
	rt.metrics.RecordAgentClarificationRound(state.ClarificationRounds)
	if state.ClarificationRounds > run.MaxClarificationRounds {
		final, err := rt.finalizeFail(ctx, runID, state, "max_clarification_rounds_exceeded",
			"exceeded the maximum number of clarification rounds")
		return final, true, err
	}
	if err := rt.store.Save(ctx, runID, state); err != nil {
		return state, true, fmt.Errorf("save runtime state: %w", err)
	}
	return state, true, nil
}

// runToolStep executes one DecisionTool step: bounds-check, persist the
// proposal, dispatch it through the registry (policy then tool), and
// persist the outcome. done reports whether the caller should stop looping
// (a terminal failure already happened and was persisted).
func (rt *Runtime) runToolStep(
	ctx context.Context, run RunContext, runID string, state RuntimeState, proposal *Proposal,
) (RuntimeState, bool, error) {
	if state.toolStepCount() >= run.MaxSteps {
		final, err := rt.finalizeFail(ctx, runID, state, "max_steps_exceeded",
			"exceeded the maximum number of planner steps")
		return final, true, err
	}

	step := RuntimeStep{Seq: state.nextSeq(), Proposal: *proposal}
	state.Steps = append(state.Steps, step)
	if proposal.Tool == ToolQueryExecute {
		state.QueryExecuteStarted = true
	}
	// Persist BEFORE the external call: a crash here resumes with the
	// proposal recorded but no observation, which is exactly what a
	// redelivered job needs to retry it rather than silently drop it.
	if err := rt.store.Save(ctx, runID, state); err != nil {
		return state, true, fmt.Errorf("save runtime state before dispatch: %w", err)
	}

	dispatchStart := time.Now()
	obs, err := rt.registry.Execute(ctx, run, *proposal)
	rt.metrics.RecordAgentStepDuration(string(proposal.Tool), time.Since(dispatchStart))
	idx := len(state.Steps) - 1
	if err != nil {
		if denied, ok := errors.AsType[*PolicyDeniedError](err); ok {
			state.Steps[idx].DeniedReason = denied.ReasonCode
			rt.metrics.RecordAgentPolicyDenial(denied.ReasonCode)
			if serr := rt.store.Save(ctx, runID, state); serr != nil {
				return state, true, fmt.Errorf("save runtime state after denial: %w", serr)
			}
			return state, false, nil // let the planner see the denial and correct course
		}
		state.Steps[idx].Error = err.Error()
		final, ferr := rt.finalizeFail(ctx, runID, state, "tool_error", err.Error())
		return final, true, ferr
	}

	state.Steps[idx].Observation = &obs
	// Persist AFTER the external call: the observation is now durable even
	// if the process crashes before the planner sees it.
	if err := rt.store.Save(ctx, runID, state); err != nil {
		return state, true, fmt.Errorf("save runtime state after dispatch: %w", err)
	}
	return state, false, nil
}

// abandonOrFail handles context cancellation / planner errors before a
// terminal decision. Once Query Execute has started, a run must never be
// abandoned non-terminally — something else (a retry, a legacy fallback)
// could otherwise run concurrently against a query that already executed.
// Before Query Execute, it is safe to return the error and let the caller
// retry or resume the run later from its still-non-terminal state.
func (rt *Runtime) abandonOrFail(ctx context.Context, runID string, state RuntimeState, reason string, cause error) (RuntimeState, error) {
	if !state.QueryExecuteStarted {
		return state, cause
	}
	// Absorbed into a terminal result: the caller sees a clean (state, nil)
	// rather than cause, precisely so nothing downstream treats an executed
	// run as retryable/fallback-eligible.
	return rt.finalizeFail(context.WithoutCancel(ctx), runID, state, reason, cause.Error())
}

func (rt *Runtime) finalizeOK(ctx context.Context, runID string, state RuntimeState, final *FinalResponse) (RuntimeState, error) {
	state.Terminal = &TerminalResult{Kind: DecisionFinal, Final: final}
	if err := rt.store.Save(ctx, runID, state); err != nil {
		return state, fmt.Errorf("save runtime state: %w", err)
	}
	rt.metrics.RecordAgentRunTerminal("completed")
	return state, nil
}

func (rt *Runtime) finalizeFail(ctx context.Context, runID string, state RuntimeState, reasonCode, message string) (RuntimeState, error) {
	state.Terminal = &TerminalResult{Kind: DecisionFail, Failure: &Failure{ReasonCode: reasonCode, Message: message}}
	if err := rt.store.Save(ctx, runID, state); err != nil {
		return state, fmt.Errorf("save runtime state: %w", err)
	}
	rt.metrics.RecordAgentRunTerminal("failed")
	rt.metrics.RecordAgentTerminalFailure(reasonCode)
	return state, nil
}

// MarshalState is a convenience for StateStore implementations that persist
// RuntimeState as JSON (e.g. the agent_runs.runtime_state JSONB column).
func MarshalState(state RuntimeState) ([]byte, error) {
	return sonic.Marshal(state)
}

// UnmarshalState is the inverse of MarshalState.
func UnmarshalState(raw []byte) (RuntimeState, error) {
	var state RuntimeState
	if len(raw) == 0 {
		return state, nil
	}
	if err := sonic.Unmarshal(raw, &state); err != nil {
		return state, fmt.Errorf("unmarshal runtime state: %w", err)
	}
	return state, nil
}
