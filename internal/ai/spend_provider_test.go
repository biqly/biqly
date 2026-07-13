package ai

import (
	"context"
	"errors"
	"testing"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
)

type fakeProvider struct {
	called bool
	result providerpkg.GenerationResult
	err    error
}

func (f *fakeProvider) Generate(context.Context, string) (providerpkg.GenerationResult, error) {
	f.called = true
	return f.result, f.err
}

func (f *fakeProvider) GenerateAt(context.Context, string, float64) (providerpkg.GenerationResult, error) {
	f.called = true
	return f.result, f.err
}

type fakeChecker struct {
	checkErr        error
	checkedWith     string
	recordedWith    string
	recordedTokens  int
	recordCallCount int
}

func (f *fakeChecker) Check(_ context.Context, workspace string) error {
	f.checkedWith = workspace
	return f.checkErr
}

func (f *fakeChecker) Record(_ context.Context, workspace string, tokens int) {
	f.recordedWith = workspace
	f.recordedTokens = tokens
	f.recordCallCount++
}

func TestWithWorkspace_RoundTrip(t *testing.T) {
	ctx := WithWorkspace(context.Background(), "ws-1")
	if got := workspaceFromContext(ctx); got != "ws-1" {
		t.Errorf("expected ws-1, got %q", got)
	}
	if got := workspaceFromContext(context.Background()); got != "" {
		t.Errorf("expected empty workspace for untagged ctx, got %q", got)
	}
}

func TestSpendLimitedProvider_BudgetExceededShortCircuits(t *testing.T) {
	next := &fakeProvider{}
	checker := &fakeChecker{checkErr: ErrSpendLimitExceeded}
	p := &SpendLimitedProvider{next: next, limiter: checker}

	ctx := WithWorkspace(context.Background(), "ws-1")
	_, err := p.Generate(ctx, "prompt")

	if !errors.Is(err, ErrSpendLimitExceeded) {
		t.Fatalf("expected ErrSpendLimitExceeded, got %v", err)
	}
	if next.called {
		t.Error("provider should not be called once the budget is exceeded")
	}
	if checker.checkedWith != "ws-1" {
		t.Errorf("expected Check for ws-1, got %q", checker.checkedWith)
	}
	if checker.recordCallCount != 0 {
		t.Error("no tokens should be recorded when generation is blocked")
	}
}

func TestSpendLimitedProvider_RecordsUsageOnSuccess(t *testing.T) {
	next := &fakeProvider{result: providerpkg.GenerationResult{
		Content: "ok",
		Usage:   &providerpkg.TokenUsage{Total: 42},
	}}
	checker := &fakeChecker{}
	p := &SpendLimitedProvider{next: next, limiter: checker}

	ctx := WithWorkspace(context.Background(), "ws-9")
	if _, err := p.Generate(ctx, "prompt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.called {
		t.Error("provider should be called when the budget check passes")
	}
	if checker.recordedWith != "ws-9" || checker.recordedTokens != 42 {
		t.Errorf("expected Record(ws-9, 42), got Record(%q, %d)", checker.recordedWith, checker.recordedTokens)
	}
}

func TestSpendLimitedProvider_NilLimiterPassesThrough(t *testing.T) {
	next := &fakeProvider{result: providerpkg.GenerationResult{Content: "ok"}}
	// NewSpendLimitedProvider with a nil *SpendLimiter must be a safe pass-through
	// (no Check, no Record, no panic) — the disabled-cap path.
	p := NewSpendLimitedProvider(next, nil)

	if _, err := p.Generate(context.Background(), "prompt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.called {
		t.Error("provider should be called when no limiter is configured")
	}
}
