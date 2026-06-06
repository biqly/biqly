package handlers

import (
	"context"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/auth/mfatest"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminGenerateMFABypassHandler(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)
	users := stack.SeedBypassTestUsers(t)
	mfatest.EnableVerifiedMFA(stack.Ctx, t, stack.DB, users.TargetUserID)

	handler := NewAuthHandler(stack.Auth, nil, stack.JWT, stack.Config, nil)
	handler.SetMFA(stack.MFA)

	router := chi.NewRouter()
	authMW := func(next http.Handler) http.Handler {
		return next
	}
	handler.RegisterAccountAdminRoutes(router, authMW)

	// Case 1: Call as normal actor -> Should return 403 Forbidden
	req := httptest.NewRequestWithContext(context.WithValue(stack.Ctx, userIDKey, users.NormalActorID), http.MethodPost, "/admin/users/"+users.TargetUserID+"/mfa/bypass", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)

	// Case 2: Call as super admin -> Should return 200 OK with bypass code
	req = httptest.NewRequestWithContext(context.WithValue(stack.Ctx, userIDKey, users.SuperActorID), http.MethodPost, "/admin/users/"+users.TargetUserID+"/mfa/bypass", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	err := sonic.ConfigStd.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	bypassCode := resp["bypass_code"]
	assert.True(t, strings.HasPrefix(bypassCode, "BYPASS-"))
}
