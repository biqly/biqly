package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// TestGetAIUsageBreakdown_AnonymousGets401 pins the status code contract for
// requests whose token was dropped by OptionalJWTAuth (expired/invalid JWT):
// no identity at all must yield 401 so clients re-authenticate, not 403.
func TestGetAIUsageBreakdown_AnonymousGets401(t *testing.T) {
	h := NewAIExamplesHandler(nil)
	h.SetAuthClient(bimw.NewAuthClient("http://auth.invalid", "token"))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ai/usage/breakdown", http.NoBody)
	rec := httptest.NewRecorder()

	h.GetAIUsageBreakdown(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous request: expected 401, got %d (%s)", rec.Code, rec.Body.String())
	}
}
