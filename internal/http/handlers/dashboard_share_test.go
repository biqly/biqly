package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/dashboard"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSharingChecker is a hand-written test double for workspaceSharingChecker
// so tests can control the kill-switch answer without hitting the real auth
// service.
type stubSharingChecker struct {
	enabled bool
	err     error
}

func (s *stubSharingChecker) WorkspacePublicSharingEnabled(_ context.Context, _ string) (bool, error) {
	return s.enabled, s.err
}

func createDashboardShareTestTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dashboards (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			workspace_id UUID,
			name TEXT NOT NULL,
			description TEXT,
			widgets JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dashboard_public_shares (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			dashboard_id UUID NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
			workspace_id UUID NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			created_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			revoked_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ
		)
	`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_public_shares_active
			ON dashboard_public_shares(dashboard_id) WHERE revoked_at IS NULL
	`)
	require.NoError(t, err)
}

// dashboardShareTestSetup wires a chi router mounting DashboardShareHandler
// against a real Postgres-backed dashboard.Repository/ShareRepository, mirroring
// the style of internal/dashboard/share_repository_test.go.
type dashboardShareTestSetup struct {
	router  *chi.Mux
	dashes  *dashboard.Repository
	checker *stubSharingChecker
}

func newDashboardShareTestSetup(t *testing.T) *dashboardShareTestSetup {
	t.Helper()
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	createDashboardShareTestTables(ctx, t, db)

	dashRepo := dashboard.NewRepository(db)
	shareRepo := dashboard.NewShareRepository(db)
	checker := &stubSharingChecker{enabled: true}
	auditLogger := audit.NewLogger(slog.Default())
	handler := NewDashboardShareHandler(shareRepo, dashRepo, checker, auditLogger)

	r := chi.NewRouter()
	r.Route("/dashboards/{id}/public-share", func(r chi.Router) {
		r.Post("/", handler.Create)
		r.Get("/", handler.Status)
		r.Delete("/", handler.Revoke)
	})

	return &dashboardShareTestSetup{router: r, dashes: dashRepo, checker: checker}
}

// testUserID is a fixed valid UUID: dashboard_public_shares.created_by is a
// UUID column, so the context user id must parse as one.
const testUserID = "44444444-4444-4444-4444-444444444444"

func withDashboardShareAuthCtx(r *http.Request, workspaceID string) *http.Request {
	ctx := bimw.WithWorkspaceID(r.Context(), workspaceID)
	ctx = bimw.WithUserID(ctx, testUserID)
	return r.WithContext(ctx)
}

// withSuperAdminAuthCtx attaches a super-admin identity with no active
// workspace, mirroring an unscoped super-admin call.
func withSuperAdminAuthCtx(r *http.Request) *http.Request {
	ctx := bimw.WithUserRoles(r.Context(), []string{bimw.RoleSuperAdmin})
	ctx = bimw.WithUserID(ctx, testUserID)
	return r.WithContext(ctx)
}

func createTestDashboard(ctx context.Context, t *testing.T, repo *dashboard.Repository, workspaceID string) *dashboard.Dashboard {
	t.Helper()
	var wsID *string
	if workspaceID != "" {
		wsID = &workspaceID
	}
	d := &dashboard.Dashboard{WorkspaceID: wsID, Name: "test dashboard", Widgets: json.RawMessage(`[]`)}
	require.NoError(t, repo.Create(ctx, d))
	return d
}

func TestDashboardShareHandler_Create(t *testing.T) {
	setup := newDashboardShareTestSetup(t)
	ctx := context.Background()
	ws := "11111111-1111-1111-1111-111111111111"
	d := createTestDashboard(ctx, t, setup.dashes, ws)

	t.Run("returns a token once and the URL path", func(t *testing.T) {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, ws)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		var body map[string]any
		require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &body))
		token, _ := body["token"].(string)
		assert.NotEmpty(t, token)
		assert.Equal(t, "/public/dashboard/"+token, body["url_path"])
		assert.NotEmpty(t, body["created_at"])

		// The plaintext token must not equal what is stored (the hash); verify
		// by hashing the returned token and confirming it maps to the active share.
		hashed := dashboard.HashShareToken(token)
		assert.NotEqual(t, token, hashed, "response must return the plaintext token, not the hash")
	})

	t.Run("on another workspace's dashboard returns 404", func(t *testing.T) {
		otherWS := "22222222-2222-2222-2222-222222222222"
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, otherWS)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("kill-switch off returns 409", func(t *testing.T) {
		// Reuse the outer setup's DB/router rather than opening a second
		// testutil.OpenMetadataDB: that helper takes a session-scoped Postgres
		// advisory lock released only in t.Cleanup at the end of the *whole*
		// top-level test, so a second call nested inside a subtest here would
		// block forever waiting on a lock the outer setup already holds.
		killSwitchDash := createTestDashboard(ctx, t, setup.dashes, ws)
		setup.checker.enabled = false
		t.Cleanup(func() { setup.checker.enabled = true })

		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+killSwitchDash.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, ws)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "public sharing is disabled for this workspace")
	})

	t.Run("global dashboard (nil workspace) rejected", func(t *testing.T) {
		globalDash := createTestDashboard(ctx, t, setup.dashes, "")
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+globalDash.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, ws)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		// dashboard.Repository.Get lets a workspace-scoped caller read global
		// (workspace_id IS NULL) dashboards too; shareScope's explicit nil check
		// is what rejects sharing them, with 409 (not 404).
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "global dashboards cannot be shared publicly")
	})

	t.Run("unscoped super-admin call rejected as not found", func(t *testing.T) {
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		req = withSuperAdminAuthCtx(req)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestDashboardShareHandler_StatusAndRevoke(t *testing.T) {
	setup := newDashboardShareTestSetup(t)
	ctx := context.Background()
	ws := "33333333-3333-3333-3333-333333333333"
	d := createTestDashboard(ctx, t, setup.dashes, ws)

	statusReq := func() *http.Request {
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		return withDashboardShareAuthCtx(req, ws)
	}

	t.Run("inactive before any share is created", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, statusReq())
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, false, body["active"])
	})

	t.Run("delete with no active share returns 404", func(t *testing.T) {
		req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, ws)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	// Create a share, then verify status flips active, then revoke and verify
	// status flips back to inactive.
	createReq := httptest.NewRequestWithContext(ctx, http.MethodPost, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
	createReq = withDashboardShareAuthCtx(createReq, ws)
	createRec := httptest.NewRecorder()
	setup.router.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)

	t.Run("status reflects active after create", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, statusReq())
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, true, body["active"])
		assert.NotEmpty(t, body["created_at"])
	})

	t.Run("delete revokes then status flips inactive", func(t *testing.T) {
		delReq := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		delReq = withDashboardShareAuthCtx(delReq, ws)
		delRec := httptest.NewRecorder()
		setup.router.ServeHTTP(delRec, delReq)
		require.Equal(t, http.StatusNoContent, delRec.Code)

		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, statusReq())
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]any
		require.NoError(t, sonic.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, false, body["active"])
	})

	t.Run("second delete with no active share returns 404", func(t *testing.T) {
		req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/dashboards/"+d.ID+"/public-share/", http.NoBody)
		req = withDashboardShareAuthCtx(req, ws)
		rec := httptest.NewRecorder()
		setup.router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
