package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/mfatest"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

func TestJWTContextPropagation(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)

	handler := NewAuthHandler(stack.Auth, nil, stack.JWT, stack.Config, nil)

	userID := "user-123"
	email := "test@example.com"
	roles := []string{"viewer", "editor"}
	workspaceID := "workspace-456"

	token, err := stack.JWT.GenerateToken(userID, email, roles, workspaceID, nil)
	require.NoError(t, err)

	authMiddleware := handler.AuthMiddleware()

	var contextUserID string
	var contextEmail string
	var contextRoles []string
	var contextWorkspaceID string
	var called bool

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		ctx := r.Context()
		contextUserID = bimw.UserID(ctx)
		contextEmail = bimw.UserEmail(ctx)
		contextRoles = bimw.UserRoles(ctx)
		contextWorkspaceID = bimw.WorkspaceID(ctx)
		w.WriteHeader(http.StatusOK)
	})

	router := authMiddleware(testHandler)

	req := httptest.NewRequestWithContext(stack.Ctx, http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, called)
	assert.Equal(t, userID, contextUserID)
	assert.Equal(t, email, contextEmail)
	assert.Equal(t, roles, contextRoles)
	assert.Equal(t, workspaceID, contextWorkspaceID)
}
