package workspace

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/rbac"
	_ "github.com/jackc/pgx/v5/stdlib"
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

	cfg := &auth.Config{JWTAccessTTL: 5 * time.Minute, JWTRefreshTTL: 24 * time.Hour}
	jwtMgr, err := auth.NewJWTManager("", "", cfg.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := auth.NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := auth.NewSessionManager(dbPool)
	rbacSvc := rbac.NewRBACService(rbacRepo)
	dsAcc := rbac.NewDatasourceAccessService(dbPool, nil, rbacSvc)
	wsSvc := NewWorkspaceService(dbPool, dsAcc)

	svc := auth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg, nil, nil)
	svc.SetWorkspaceService(wsSvc)

	ua, ip := "Go-Test-Agent", "127.0.0.1"

	// Register two users: alice (will switch), bob (alice not member of bob's ws)
	aliceResp, err := svc.Register(ctx, auth.RegisterRequest{
		Email:       "alice@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Alice",
	}, &ua, &ip)
	require.NoError(t, err)

	bobResp, err := svc.Register(ctx, auth.RegisterRequest{
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
		loginResp, err := svc.Login(ctx, auth.LoginRequest{
			Email:    "alice@example.com",
			Password: "SecurePass123!",
		}, &ua, &ip)
		require.NoError(t, err)

		claims, err := jwtMgr.ValidateToken(loginResp.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, teamWS.ID, claims.WorkspaceID, "active workspace should persist across logins")
	})

	t.Run("workspace mfa policy blocks password-only login without enrollment", func(t *testing.T) {
		require.NoError(t, wsSvc.SetMFARequired(ctx, teamWS.ID, aliceResp.UserID, true))

		loginResp, err := svc.Login(ctx, auth.LoginRequest{
			Email:    "alice@example.com",
			Password: "SecurePass123!",
		}, &ua, &ip)
		require.ErrorIs(t, err, auth.ErrMFARequired)
		assert.Nil(t, loginResp)
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

func openTestDBPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test default DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"
	}
	dbPool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping database tests; DB not available:", err)
	}
	t.Cleanup(func() { _ = dbPool.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
	}
	return dbPool
}
