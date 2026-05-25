package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetActiveWorkspace(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	cfg := &Config{JWTAccessTTL: 5 * time.Minute, JWTRefreshTTL: 24 * time.Hour}
	jwtMgr, err := NewJWTManager("", "", cfg.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	rbacSvc := NewRBACService(rbacRepo)
	dsAcc := NewDatasourceAccessService(dbPool, nil, rbacSvc)
	wsSvc := NewWorkspaceService(dbPool, dsAcc)

	svc := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg, nil, nil)
	svc.SetWorkspaceService(wsSvc)

	ua, ip := "Go-Test-Agent", "127.0.0.1"

	// Register two users: alice (will switch), bob (alice not member of bob's ws)
	aliceResp, err := svc.Register(ctx, RegisterRequest{
		Email:       "alice@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Alice",
	}, &ua, &ip)
	require.NoError(t, err)

	bobResp, err := svc.Register(ctx, RegisterRequest{
		Email:       "bob@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Bob",
	}, &ua, &ip)
	require.NoError(t, err)

	// Personal workspaces auto-created — get bob's so we can deny alice access to it
	bobPersonal, err := userRepo.GetPersonalWorkspaceID(ctx, bobResp.UserID)
	require.NoError(t, err)

	// Create a team workspace owned by alice
	teamWS, err := wsSvc.Create(ctx, "Team Alpha", "shared workspace", aliceResp.UserID)
	require.NoError(t, err)

	t.Run("rejects empty workspace_id", func(t *testing.T) {
		_, err := svc.SetActiveWorkspace(ctx, aliceResp.UserID, "")
		require.Error(t, err)
	})

	t.Run("rejects non-member workspace", func(t *testing.T) {
		_, err := svc.SetActiveWorkspace(ctx, aliceResp.UserID, bobPersonal)
		require.ErrorIs(t, err, ErrNotWorkspaceOwner)
	})

	t.Run("switches to team workspace and reissues token with claim", func(t *testing.T) {
		resp, err := svc.SetActiveWorkspace(ctx, aliceResp.UserID, teamWS.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.AccessToken)
		assert.Equal(t, teamWS.ID, resp.ActiveWorkspaceID)

		claims, err := jwtMgr.ValidateToken(resp.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, teamWS.ID, claims.WorkspaceID)
		assert.Equal(t, aliceResp.UserID, claims.Subject)
	})

	t.Run("persisted across subsequent token issuance", func(t *testing.T) {
		// Re-login: new access token should now carry the team workspace
		loginResp, err := svc.Login(ctx, LoginRequest{
			Email:    "alice@example.com",
			Password: "SecurePass123!",
		}, &ua, &ip)
		require.NoError(t, err)

		claims, err := jwtMgr.ValidateToken(loginResp.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, teamWS.ID, claims.WorkspaceID, "active workspace should persist across logins")
	})

	t.Run("GetMe reports active workspace", func(t *testing.T) {
		me, err := svc.GetMe(ctx, aliceResp.UserID)
		require.NoError(t, err)
		assert.Equal(t, teamWS.ID, me.ActiveWorkspaceID)
	})

	t.Run("falls back to personal when active workspace deleted", func(t *testing.T) {
		// Delete the team workspace (alice is owner)
		require.NoError(t, wsSvc.Delete(ctx, teamWS.ID, aliceResp.UserID))

		// Active pointer is now dangling; should fall back to personal
		alicePersonal, err := userRepo.GetPersonalWorkspaceID(ctx, aliceResp.UserID)
		require.NoError(t, err)

		got, err := userRepo.GetActiveOrPersonalWorkspaceID(ctx, aliceResp.UserID)
		require.NoError(t, err)
		assert.Equal(t, alicePersonal, got)
	})
}
