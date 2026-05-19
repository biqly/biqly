package tracecontext

import (
	"context"
	"testing"
)

const sampleTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestTraceparentRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := WithTraceparent(context.Background(), sampleTraceparent)
	if got := TraceparentFromContext(ctx); got != sampleTraceparent {
		t.Fatalf("traceparent: got %q, want %q", got, sampleTraceparent)
	}
}

func TestEmptyTraceparentDoesNotWrapContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := WithTraceparent(ctx, " "); got != ctx {
		t.Fatal("empty traceparent should return original context")
	}
}
