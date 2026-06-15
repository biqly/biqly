package workspace

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sharingIDORFixture struct {
	ctx           context.Context
	dbPool        *sql.DB
	sharingSvc    *SharingService
	wsSvc         *Service
	aliceID       string
	bobID         string
	alicePersonal string
	bobPersonal   string
}

func setupSharingIDORTest(t *testing.T) *sharingIDORFixture {
	t.Helper()

	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	testutil.ResetAuthUserTables(ctx, t, dbPool)

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

	aliceResp, err := svc.Register(ctx, auth.RegisterRequest{
		Email:       "alice_sharing@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Alice",
	}, &ua, &ip)
	require.NoError(t, err)

	bobResp, err := svc.Register(ctx, auth.RegisterRequest{
		Email:       "bob_sharing@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Bob",
	}, &ua, &ip)
	require.NoError(t, err)

	alicePersonal, err := userRepo.GetPersonalWorkspaceID(ctx, aliceResp.UserID)
	require.NoError(t, err)

	bobPersonal, err := userRepo.GetPersonalWorkspaceID(ctx, bobResp.UserID)
	require.NoError(t, err)

	return &sharingIDORFixture{
		ctx:           ctx,
		dbPool:        dbPool,
		sharingSvc:    NewSharingService(dbPool, wsSvc),
		wsSvc:         wsSvc,
		aliceID:       aliceResp.UserID,
		bobID:         bobResp.UserID,
		alicePersonal: alicePersonal,
		bobPersonal:   bobPersonal,
	}
}

func (f *sharingIDORFixture) cleanupShares(t *testing.T) {
	t.Helper()
	_, err := f.dbPool.ExecContext(f.ctx, "DELETE FROM resource_shares")
	require.NoError(t, err)
}

func TestShare_IDOR_Guards(t *testing.T) {
	f := setupSharingIDORTest(t)

	t.Run("share to own workspace succeeds", func(t *testing.T) {
		f.cleanupShares(t)

		share, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			WorkspaceID:  &f.alicePersonal,
			Permission:   "view",
		})
		require.NoError(t, err)
		require.NotNil(t, share)
		assert.Equal(t, f.aliceID, share.OwnerID)
		assert.Equal(t, "dashboard", share.ResourceType)
	})

	t.Run("share to workspace caller is not a member of is rejected", func(t *testing.T) {
		f.cleanupShares(t)

		share, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "b1c2d3e4-f5a6-7890-abcd-ef1234567890",
			WorkspaceID:  &f.bobPersonal,
			Permission:   "view",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a member of the target workspace")
		assert.Nil(t, share)
	})

	t.Run("share to user directly does not check workspace membership", func(t *testing.T) {
		f.cleanupShares(t)

		sharedWith := f.bobID
		share, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "report",
			ResourceID:   "c1d2e3f4-a5b6-7890-abcd-ef1234567890",
			SharedWith:   &sharedWith,
			Permission:   "edit",
		})
		require.NoError(t, err)
		require.NotNil(t, share)
		assert.Equal(t, f.aliceID, share.OwnerID)
		assert.Equal(t, sharedWith, *share.SharedWith)
	})

	t.Run("share to team workspace where caller is a member succeeds", func(t *testing.T) {
		f.cleanupShares(t)

		teamWS, err := f.wsSvc.Create(f.ctx, "Team Beta", "shared workspace", f.aliceID)
		require.NoError(t, err)

		share, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "d1e2f3a4-b5c6-7890-abcd-ef1234567890",
			WorkspaceID:  &teamWS.ID,
			Permission:   "view",
		})
		require.NoError(t, err)
		require.NotNil(t, share)
		assert.Equal(t, teamWS.ID, *share.WorkspaceID)
	})

	t.Run("bob cannot share to alice's team workspace", func(t *testing.T) {
		f.cleanupShares(t)

		teamWS, err := f.wsSvc.Create(f.ctx, "Team Gamma", "alice only", f.aliceID)
		require.NoError(t, err)

		share, err := f.sharingSvc.Share(f.ctx, f.bobID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "e1f2a3b4-c5d6-7890-abcd-ef1234567890",
			WorkspaceID:  &teamWS.ID,
			Permission:   "view",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a member of the target workspace")
		assert.Nil(t, share)
	})

	t.Run("missing resource_type or resource_id returns error", func(t *testing.T) {
		_, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "",
			ResourceID:   "f1a2b3c4-d5e6-7890-abcd-ef1234567890",
			Permission:   "view",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource_type and resource_id are required")

		_, err = f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "",
			Permission:   "view",
		})
		require.Error(t, err)
	})

	t.Run("missing shared_with and workspace_id returns error", func(t *testing.T) {
		_, err := f.sharingSvc.Share(f.ctx, f.aliceID, ShareRequest{
			ResourceType: "dashboard",
			ResourceID:   "f1a2b3c4-d5e6-7890-abcd-ef1234567890",
			Permission:   "view",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either shared_with or workspace_id must be provided")
	})
}
