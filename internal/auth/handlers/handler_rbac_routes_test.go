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
		dsAccess: rbac.NewDatasourceAccessService(db, nil, nil),
	}

	req := httptest.NewRequestWithContext(bimw.WithUserID(ctx, "user-1"), http.MethodGet, "/me/datasources/ds-1/check", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "ds-1")
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
		dsAccess: rbac.NewDatasourceAccessService(db, nil, nil),
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
