package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/mfatest"
)

func TestAdminListInvitationsPagination(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)
	users := stack.SeedBypassTestUsers(t)

	// Clean up any existing invitations
	_, err := stack.DB.ExecContext(stack.Ctx, "DELETE FROM user_invitations")
	require.NoError(t, err)

	// Seed 15 invitations
	for i := range 15 {
		email := fmt.Sprintf("invite-%d@example.com", i)
		err = stack.Auth.InviteUser(stack.Ctx, users.SuperActorID, email, "viewer")
		require.NoError(t, err)
	}

	handler := NewAuthHandler(stack.Auth, nil, stack.JWT, stack.Config, nil)
	router := chi.NewRouter()
	handler.RegisterAuthRoutes(router)

	// Generate super admin token to pass authMiddleware
	token, err := stack.JWT.GenerateToken(users.SuperActorID, "super@example.com", []string{"super_admin"}, "", nil)
	require.NoError(t, err)

	// Case 1: Default pagination parameters (Page 1, size 10)
	req := httptest.NewRequestWithContext(stack.Ctx, http.MethodGet, "/admin/invitations", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp1 map[string]any
	err = sonic.ConfigStd.Unmarshal(rr.Body.Bytes(), &resp1)
	require.NoError(t, err)

	invites1, ok := resp1["invitations"].([]any)
	require.True(t, ok)
	assert.Len(t, invites1, 10)
	assert.Equal(t, float64(15), resp1["total"])

	// Case 2: Custom pagination parameters (page=2, page_size=6)
	req = httptest.NewRequestWithContext(stack.Ctx, http.MethodGet, "/admin/invitations?page=2&page_size=6", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp2 map[string]any
	err = sonic.ConfigStd.Unmarshal(rr.Body.Bytes(), &resp2)
	require.NoError(t, err)

	invites2, ok := resp2["invitations"].([]any)
	require.True(t, ok)
	assert.Len(t, invites2, 6)
	assert.Equal(t, float64(15), resp2["total"])

	// Case 3: Out of bounds page (page=4, page_size=5 -> offset=15, 15 >= 15 total)
	req = httptest.NewRequestWithContext(stack.Ctx, http.MethodGet, "/admin/invitations?page=4&page_size=5", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp3 map[string]any
	err = sonic.ConfigStd.Unmarshal(rr.Body.Bytes(), &resp3)
	require.NoError(t, err)

	invites3, ok := resp3["invitations"].([]any)
	require.True(t, ok)
	assert.Len(t, invites3, 0)
	assert.Equal(t, float64(15), resp3["total"])
}
