package ai

import (
	"context"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
)

// workspaceCtxKey carries the workspace whose daily AI token budget applies to
// provider calls made under a context.
type workspaceCtxKey struct{}

// WithWorkspace tags ctx with the workspace whose AI spend budget applies to
// any provider call made under it. Background workers (e.g. the agent service)
// build one shared provider across many jobs, so the per-job workspace must
// ride on the context rather than being captured at construction time — this is
// the counterpart to the HTTP path capturing bimw.WorkspaceID(ctx) per request.
func WithWorkspace(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, workspaceCtxKey{}, workspaceID)
}

// workspaceFromContext returns the workspace tagged by WithWorkspace, or "" when
// none is set (which disables the cap for that call, matching SpendLimiter).
func workspaceFromContext(ctx context.Context) string {
	ws, _ := ctx.Value(workspaceCtxKey{}).(string)
	return ws
}

// spendChecker is the subset of *SpendLimiter that SpendLimitedProvider needs.
// Narrowing to an interface lets tests force a rejection deterministically
// without a real Redis instance.
type spendChecker interface {
	Check(ctx context.Context, workspace string) error
	Record(ctx context.Context, workspace string, tokens int)
}

// SpendLimitedProvider wraps a Provider and enforces a per-workspace daily
// token budget: it Checks the budget before each generation and Records the
// tokens used afterwards. The workspace is read from the request context (see
// WithWorkspace); an empty workspace or a nil limiter disables the cap for that
// call. Use this for background/worker provider paths where the workspace is
// not known at construction time; the HTTP path uses its own request-scoped
// wrapper that captures the workspace directly.
type SpendLimitedProvider struct {
	next    providerpkg.Provider
	limiter spendChecker
}

// NewSpendLimitedProvider wraps next so every generation is gated on limiter.
// A nil limiter (or one built without Redis / with a zero budget) is a safe
// pass-through.
func NewSpendLimitedProvider(next providerpkg.Provider, limiter *SpendLimiter) *SpendLimitedProvider {
	return &SpendLimitedProvider{next: next, limiter: limiter}
}

// Generate enforces the budget, generates, then records usage.
func (p *SpendLimitedProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	ws := workspaceFromContext(ctx)
	if p.limiter != nil {
		if err := p.limiter.Check(ctx, ws); err != nil {
			return providerpkg.GenerationResult{}, err
		}
	}
	result, err := p.next.Generate(ctx, prompt)
	p.record(ctx, ws, result)
	return result, err
}

// GenerateAt is Generate with an explicit temperature.
func (p *SpendLimitedProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (providerpkg.GenerationResult, error) {
	ws := workspaceFromContext(ctx)
	if p.limiter != nil {
		if err := p.limiter.Check(ctx, ws); err != nil {
			return providerpkg.GenerationResult{}, err
		}
	}
	result, err := p.next.GenerateAt(ctx, prompt, temperature)
	p.record(ctx, ws, result)
	return result, err
}

func (p *SpendLimitedProvider) record(ctx context.Context, workspace string, result providerpkg.GenerationResult) {
	if p.limiter == nil || result.Usage == nil {
		return
	}
	p.limiter.Record(ctx, workspace, result.Usage.Total)
}
