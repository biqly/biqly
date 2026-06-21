package prompt

import (
	"context"
	"testing"
)

func TestWithUserIDEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	got := WithUserID(ctx, "")
	if got != ctx {
		t.Error("WithUserID with empty string should return original context")
	}
}

func TestWithUserIDStoresAndRetrieves(t *testing.T) {
	t.Parallel()
	ctx := WithUserID(context.Background(), "user-42")
	if id := UserIDFromContext(ctx); id != "user-42" {
		t.Fatalf("UserIDFromContext = %q, want %q", id, "user-42")
	}
}

func TestWithUserIDDifferentValues(t *testing.T) {
	t.Parallel()
	ctx1 := WithUserID(context.Background(), "alice")
	ctx2 := WithUserID(context.Background(), "bob")
	if UserIDFromContext(ctx1) != "alice" {
		t.Fatal("ctx1 should have alice")
	}
	if UserIDFromContext(ctx2) != "bob" {
		t.Fatal("ctx2 should have bob")
	}
}

func TestUserIDFromContextEmpty(t *testing.T) {
	t.Parallel()
	if id := UserIDFromContext(context.Background()); id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
}

type unrelatedContextKey struct{}

func TestUserIDFromContextNested(t *testing.T) {
	t.Parallel()
	inner := WithUserID(context.Background(), "user-99")
	outer := context.WithValue(inner, unrelatedContextKey{}, "some-other-value")
	if id := UserIDFromContext(outer); id != "user-99" {
		t.Fatalf("UserIDFromContext should traverse wrapper contexts: got %q", id)
	}
}
