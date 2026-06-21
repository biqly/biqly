package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/ai"
)

func TestInjectAIUserContext_WithUserID(t *testing.T) {
	var gotUserID string
	handler := InjectAIUserContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = ai.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	ctx := WithUserID(context.Background(), "u-99")
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUserID != "u-99" {
		t.Fatalf("expected AI context userID 'u-99', got %q", gotUserID)
	}
}

func TestInjectAIUserContext_WithoutUserID(t *testing.T) {
	var gotUserID string
	handler := InjectAIUserContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = ai.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUserID != "" {
		t.Fatalf("expected empty AI userID when no auth context, got %q", gotUserID)
	}
}
