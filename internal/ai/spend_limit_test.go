package ai

import (
	"context"
	"testing"
	"time"
)

func TestSpendLimiterDisabledAlwaysAllows(t *testing.T) {
	ctx := context.Background()

	// nil limiter
	var nilLimiter *SpendLimiter
	if err := nilLimiter.Check(ctx, "ws1"); err != nil {
		t.Fatalf("nil limiter Check = %v, want nil", err)
	}
	nilLimiter.Record(ctx, "ws1", 100) // must not panic

	// nil client
	l := NewSpendLimiter(nil, 1000)
	if err := l.Check(ctx, "ws1"); err != nil {
		t.Fatalf("nil-client Check = %v, want nil", err)
	}
	l.Record(ctx, "ws1", 100) // no-op, must not panic

	// budget <= 0 disables
	if NewSpendLimiter(nil, 0).enabled("ws1") {
		t.Fatal("budget 0 must disable the limiter")
	}
	// empty workspace disables
	if NewSpendLimiter(nil, 1000).enabled("") {
		t.Fatal("empty workspace must disable the limiter")
	}
}

func TestSpendLimiterKeyIsPerWorkspacePerUTCDay(t *testing.T) {
	l := NewSpendLimiter(nil, 1000)
	at := time.Date(2026, 7, 3, 23, 59, 0, 0, time.UTC)
	if got, want := l.key("ws-42", at), "ai_spend:ws-42:20260703"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	// A different workspace on the same day gets a distinct key.
	if l.key("ws-99", at) == l.key("ws-42", at) {
		t.Fatal("keys must differ per workspace")
	}
	// Local-time evening that is the next UTC day buckets under the UTC date.
	atLocalNextUTC := time.Date(2026, 7, 3, 23, 0, 0, 0, time.FixedZone("UTC-2", -2*60*60))
	if got, want := l.key("ws-42", atLocalNextUTC), "ai_spend:ws-42:20260704"; got != want {
		t.Fatalf("key (tz) = %q, want %q", got, want)
	}
}
