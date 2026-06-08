package handlers

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

func TestBuildAIHistoryEntrySetsUserIDFromContext(t *testing.T) {
	t.Parallel()

	ctx := ai.WithUserID(context.Background(), "user-abc")
	entry := buildAIHistoryEntry(ctx, aiQueryRequest{DatasourceID: "ds-1", Question: "q"}, nil, nil, nil)
	if entry.UserID == nil || *entry.UserID != "user-abc" {
		t.Fatalf("UserID = %#v, want user-abc", entry.UserID)
	}
}

func TestBuildAIHistoryEntryPrefersAIContextOverMiddleware(t *testing.T) {
	t.Parallel()

	ctx := ai.WithUserID(context.Background(), "from-ai")
	ctx = context.WithValue(ctx, bimw.UserIDKey, "from-middleware")
	entry := buildAIHistoryEntry(ctx, aiQueryRequest{DatasourceID: "ds-1", Question: "q"}, nil, nil, nil)
	if entry.UserID == nil || *entry.UserID != "from-ai" {
		t.Fatalf("UserID = %#v, want from-ai", entry.UserID)
	}
}

func TestBuildAIHistoryEntryFallsBackToMiddlewareUserID(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), bimw.UserIDKey, "from-middleware")
	entry := buildAIHistoryEntry(ctx, aiQueryRequest{DatasourceID: "ds-1", Question: "q"}, nil, nil, nil)
	if entry.UserID == nil || *entry.UserID != "from-middleware" {
		t.Fatalf("UserID = %#v, want from-middleware", entry.UserID)
	}
}
