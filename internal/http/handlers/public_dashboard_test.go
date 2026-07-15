package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/dashboard"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublicDashboardHandler_Get exercises the anonymous metadata endpoint
// against a real bi_metadata Postgres, mirroring dashboard_share_test.go: the
// PublicResolver is a concrete type over a *sql.DB, so a stub is impractical
// and an integration test gives cleaner coverage than a fake.
func TestPublicDashboardHandler_Get(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	createDashboardShareTestTables(ctx, t, db)

	dashRepo := dashboard.NewRepository(db)
	shareRepo := dashboard.NewShareRepository(db)
	resolver := dashboard.NewPublicResolver(db)

	ws := "55555555-5555-5555-5555-555555555555"
	// Widget carries both render config (config/title) and query internals
	// (logical_query/saved_query_id); the anonymous view must expose the former
	// and strip the latter.
	widgets := json.RawMessage(`[{"id":"w1","type":"chart","title":"Revenue","config":{"metric":"sales"},"logical_query":{"select":["x"]},"saved_query_id":"11111111-2222-3333-4444-555555555555"}]`)
	d := &dashboard.Dashboard{WorkspaceID: &ws, Name: "shared dashboard", Widgets: widgets}
	require.NoError(t, dashRepo.Create(ctx, d))

	token, err := dashboard.GenerateShareToken()
	require.NoError(t, err)
	require.NoError(t, shareRepo.Rotate(ctx, &dashboard.PublicShare{
		DashboardID: d.ID,
		WorkspaceID: ws,
		TokenHash:   dashboard.HashShareToken(token),
		CreatedBy:   testUserID,
	}))

	newRouter := func(sharingEnabled bool) *chi.Mux {
		h := NewPublicDashboardHandler(resolver, &stubSharingChecker{enabled: sharingEnabled})
		r := chi.NewRouter()
		r.Get("/public/dashboards/{token}", h.Get)
		return r
	}
	get := func(r *chi.Mux, tok string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/public/dashboards/"+tok, http.NoBody)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// Capture the invalid-token 404 up front so the kill-switch case can assert
	// byte-identical status + body (no leak of which check failed).
	invalidRec := get(newRouter(true), "not-a-real-token")

	t.Run("invalid token returns 404", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, invalidRec.Code)
	})

	t.Run("valid token returns sanitized dashboard", func(t *testing.T) {
		rec := get(newRouter(true), token)
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "shared dashboard")
		assert.Contains(t, body, "Revenue")
		assert.Contains(t, body, "config")
		assert.NotContains(t, body, "logical_query")
		assert.NotContains(t, body, "saved_query_id")
	})

	t.Run("kill-switch off returns the same 404 as an invalid token", func(t *testing.T) {
		rec := get(newRouter(false), token)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Equal(t, invalidRec.Code, rec.Code, "status must match the invalid-token case")
		assert.Equal(t, invalidRec.Body.String(), rec.Body.String(), "body must match the invalid-token case")
	})
}
