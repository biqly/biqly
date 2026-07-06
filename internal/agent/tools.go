package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Observation is a tool's structured result, handed back to the planner.
type Observation struct {
	Tool    ToolName
	Payload json.RawMessage
}

// Tool is a governed adapter to one upstream BI service capability. By the
// time Execute runs, arguments have already passed PolicyEngine.Evaluate —
// Execute trusts them and must not re-widen anything policy narrowed.
type Tool interface {
	Name() ToolName
	Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error)
}

// ErrToolNotRegistered is returned when a proposal names a tool the
// Registry has no adapter for.
var ErrToolNotRegistered = errors.New("tool not registered")

// PolicyDeniedError reports that PolicyEngine.Evaluate rejected a proposal
// before it ever reached a tool.
type PolicyDeniedError struct {
	Tool       ToolName
	ReasonCode string
}

func (e *PolicyDeniedError) Error() string {
	return fmt.Sprintf("tool %s denied by policy: %s", e.Tool, e.ReasonCode)
}

// Registry evaluates a proposal against policy and, only if allowed,
// dispatches it to the matching Tool with policy's (possibly narrowed)
// arguments.
type Registry struct {
	policy *PolicyEngine
	tools  map[ToolName]Tool
}

// NewRegistry builds a Registry backed by policy, one adapter per tool.
func NewRegistry(policy *PolicyEngine, tools ...Tool) *Registry {
	r := &Registry{policy: policy, tools: make(map[ToolName]Tool, len(tools))}
	for _, t := range tools {
		r.tools[t.Name()] = t
	}
	return r
}

// TransientError marks an upstream failure safe to retry once. Tool adapters
// wrap retryable upstream errors in TransientError; anything else (including
// context cancellation) is never retried.
type TransientError struct {
	Err error
}

func (e *TransientError) Error() string { return e.Err.Error() }
func (e *TransientError) Unwrap() error { return e.Err }

func isTransient(err error) bool {
	var t *TransientError
	return errors.As(err, &t)
}

// callWithSingleRetry invokes fn once; on a TransientError (and only then,
// and only when ctx has not already expired) it retries exactly once more.
func callWithSingleRetry[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	result, err := fn(ctx)
	if err == nil || ctx.Err() != nil || !isTransient(err) {
		return result, err
	}
	return fn(ctx)
}

// Execute evaluates proposal against run, then invokes the registered tool
// with the policy-approved arguments. Returns *PolicyDeniedError when
// policy denies, ErrToolNotRegistered when no adapter matches the tool.
func (r *Registry) Execute(ctx context.Context, run RunContext, proposal Proposal) (Observation, error) {
	decision := r.policy.Evaluate(ctx, run, proposal)
	if !decision.Allowed {
		return Observation{}, &PolicyDeniedError{Tool: proposal.Tool, ReasonCode: decision.ReasonCode}
	}
	tool, ok := r.tools[proposal.Tool]
	if !ok {
		return Observation{}, fmt.Errorf("%s: %w", proposal.Tool, ErrToolNotRegistered)
	}
	return tool.Execute(ctx, run, decision.Arguments)
}
