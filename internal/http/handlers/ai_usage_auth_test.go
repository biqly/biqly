package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"

	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// TestGetAIUsageBreakdown_AnonymousGets401 pins the status code contract for
// requests whose token was dropped by OptionalJWTAuth (expired/invalid JWT):
// no identity at all must yield 401 so clients re-authenticate, not 403.
// TestGetMyAIUsage_AnonymousGetsZeros pins the contract for the Home page
// summary endpoint: no identity yields an empty usage payload (not an error),
// mirroring QueryHistory's behavior.
func TestGetMyAIUsage_AnonymousGetsZeros(t *testing.T) {
	h := NewAIExamplesHandler(nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ai/usage/me", http.NoBody)
	rec := httptest.NewRecorder()

	h.GetMyAIUsage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous request: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Days        int `json:"days"`
		QueryCount  int `json:"query_count"`
		TotalTokens int `json:"total_tokens"`
	}
	if err := sonic.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Days != 30 || body.QueryCount != 0 || body.TotalTokens != 0 {
		t.Fatalf("anonymous usage: expected zeros with days=30, got %+v", body)
	}
}

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
