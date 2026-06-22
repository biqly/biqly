package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/rbac"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/testutil"
)

// TestRegisterAuthRoutes_NoConflict ensures the permission-gated admin and
// workspace route groups register without a chi path/method conflict (the
// /users/{id}/roles GET and POST live in separate permission groups).
func TestRegisterAuthRoutes_NoConflict(t *testing.T) {
	h := &RBACHandler{}
	r := chi.NewRouter()
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("route registration panicked: %v", rec)
		}
	}()
	h.RegisterAuthRoutes(r, func(next http.Handler) http.Handler { return next })
}

func TestCheckDatasourceAccessDenialReasonOnlyForDenied(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, db)
	handler := &RBACHandler{
		deps: RBACHandlerDeps{DsAccess: rbac.NewDatasourceAccessService(db, nil, nil)},
	}

	const (
		userID       = "00000000-0000-0000-0000-000000000001"
		datasourceID = "00000000-0000-0000-0000-00000000d001"
	)

	req := httptest.NewRequestWithContext(bimw.WithUserID(ctx, userID), http.MethodGet, "/me/datasources/"+datasourceID+"/check", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", datasourceID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()

	handler.handleCheckMyDatasource(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"allowed":false`)
	assert.Contains(t, rec.Body.String(), rbac.ErrDatasourceAccessDenied.Error())
}

func TestInternalCheckDatasourceAccessDoesNotLeakInternalError(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	require.NoError(t, db.Close())
	handler := &RBACHandler{
		deps: RBACHandlerDeps{DsAccess: rbac.NewDatasourceAccessService(db, nil, nil)},
	}

	body := bytes.NewBufferString(`{"user_id":"user-1","datasource_id":"ds-1","level":"read"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/check-datasource-access", body)
	rec := httptest.NewRecorder()

	handler.handleInternalCheckDSAccess(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	responseBody := strings.ToLower(rec.Body.String())
	assert.NotContains(t, responseBody, "database is closed")
	assert.NotContains(t, responseBody, "check direct access")
}

func TestHandleAdminAssignRoleRejectsSuperAdminGrantFromNonSuperAdmin(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()

	const (
		callerEmail = "handler_assign_role_caller@example.com"
		targetEmail = "handler_assign_role_target@example.com"
	)
	testutil.PurgeAuthUsersByEmail(ctx, t, db, callerEmail, targetEmail)

	var callerID, targetID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Role Caller', 'hash', TRUE)
		 RETURNING id`,
		callerEmail,
	).Scan(&callerID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Role Target', 'hash', TRUE)
		 RETURNING id`,
		targetEmail,
	).Scan(&targetID))
	t.Cleanup(func() {
		testutil.PurgeAuthUsersByEmail(ctx, t, db, callerEmail, targetEmail)
	})

	var viewerRoleID, superRoleID string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, "viewer").Scan(&viewerRoleID))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin).Scan(&superRoleID))

	rbacRepo := rbac.NewRBACRepository(db)
	require.NoError(t, rbacRepo.AssignRole(ctx, callerID, viewerRoleID, nil, nil))
	handler := NewRBACHandler(RBACHandlerDeps{Rbac: rbac.NewRBACService(rbacRepo), RbacRepo: rbacRepo})

	body := bytes.NewBufferString(`{"role_id":"` + superRoleID + `"}`)
	req := httptest.NewRequestWithContext(bimw.WithUserID(ctx, callerID), http.MethodPost, "/admin/users/"+targetID+"/roles", body)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", targetID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()

	handler.handleAdminAssignRole(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), rbac.ErrPrivilegedRoleEscalation.Error())
}
