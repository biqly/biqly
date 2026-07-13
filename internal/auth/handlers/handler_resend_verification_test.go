package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/mfatest"
)

// TestAdminResendUserVerificationRequiresSuperAdmin verifies the admin
// resend-verification endpoint is gated by a super-admin check (S1). A normal
// authenticated user is forbidden (403), while a super admin passes the guard
// and reaches the service — which returns 400 here because the seeded target is
// already verified. The super-admin decision is DB/RBAC-based, so a "super_admin"
// JWT roles claim on the normal user would not (and does not) grant access.
func TestAdminResendUserVerificationRequiresSuperAdmin(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)
	users := stack.SeedBypassTestUsers(t)

	handler := NewAuthHandler(stack.Auth, nil, stack.JWT, stack.Config, nil)
	router := chi.NewRouter()
	handler.RegisterAuthRoutes(router)

	do := func(t *testing.T, actorID, email string, roles []string) int {
		t.Helper()
		token, err := stack.JWT.GenerateToken(actorID, email, roles, "", nil)
		require.NoError(t, err)
		req := httptest.NewRequestWithContext(stack.Ctx, http.MethodPost,
			"/admin/users/"+users.TargetUserID+"/resend-verification", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		return rr.Code
	}

	// Non-super-admin user is forbidden even with a super_admin JWT claim.
	assert.Equal(t, http.StatusForbidden,
		do(t, users.NormalActorID, "normal@example.com", []string{"super_admin"}))

	// Super admin passes the guard and reaches the service; the target is
	// already verified, so the service responds 400 (not 403).
	assert.Equal(t, http.StatusBadRequest,
		do(t, users.SuperActorID, "super@example.com", []string{"super_admin"}))
}
