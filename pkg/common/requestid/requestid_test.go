package requestid

import (
	"context"
	"testing"
)

func TestRequestIDRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := WithRequestID(context.Background(), "req-123")
	if got := FromContext(ctx); got != "req-123" {
		t.Fatalf("request id: got %q, want req-123", got)
	}
}

func TestEmptyRequestIDDoesNotWrapContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got := WithRequestID(ctx, ""); got != ctx {
		t.Fatal("empty request ID should return original context")
	}
}

func TestFromContextNil(t *testing.T) {
	t.Parallel()
	if got := FromContext(context.TODO()); got != "" {
		t.Fatalf("FromContext(nil) = %q, want empty", got)
	}
}

func TestFromContextMissingKey(t *testing.T) {
	t.Parallel()
	if got := FromContext(context.Background()); got != "" {
		t.Fatalf("FromContext(background) = %q, want empty", got)
	}
}
