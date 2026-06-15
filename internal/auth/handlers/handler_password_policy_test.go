package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/mfatest"
	"github.com/biqly/biqly/internal/testutil"
)

func TestPasswordPolicyReportsFirstUserSetupState(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)
	testutil.ResetAuthIntegrationTables(stack.Ctx, t, stack.DB)

	handler := NewAuthHandler(stack.Auth, nil, stack.JWT, stack.Config, nil)
	router := chi.NewRouter()
	handler.RegisterAuthRoutes(router)

	resp := requestPasswordPolicy(t, router)
	assert.True(t, resp.FirstUserSetupRequired)

	_, err := stack.UserRepo.CreateUser(stack.Ctx, "policy-user@example.com", "SecurePass123!", "Policy User")
	require.NoError(t, err)

	resp = requestPasswordPolicy(t, router)
	assert.False(t, resp.FirstUserSetupRequired)
}

func requestPasswordPolicy(t *testing.T, router http.Handler) PasswordPolicyResponse {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/password-policy", http.NoBody)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp PasswordPolicyResponse
	require.NoError(t, sonic.ConfigStd.Unmarshal(rr.Body.Bytes(), &resp))
	return resp
}
