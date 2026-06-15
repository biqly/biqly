package auth

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
)

func TestSelfSignupDisabledBlocksRegister(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool)

	service, userRepo, _ := newPlatformSettingsTestService(t, dbPool)
	_, err := userRepo.CreateUser(ctx, "existing-user@example.com", "SecurePass123!", "Existing User")
	require.NoError(t, err)

	_, err = dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = false WHERE id = 1")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = true WHERE id = 1")
	})

	_, err = service.Register(ctx, RegisterRequest{
		Email:       "newuser@example.com",
		Password:    "SecurePass123!",
		DisplayName: "New User",
	}, nil, nil)
	assert.ErrorIs(t, err, ErrSelfSignupDisabled)
}

func TestFirstUserSetupAllowsRegisterWhenSelfSignupDisabled(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool)

	_, err := dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = false WHERE id = 1")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = true WHERE id = 1")
	})

	service, _, rbacRepo := newPlatformSettingsTestService(t, dbPool)

	required, err := service.FirstUserSetupRequired(ctx)
	require.NoError(t, err)
	assert.True(t, required)

	resp, err := service.Register(ctx, RegisterRequest{
		Email:       "first-admin@example.com",
		Password:    "SecurePass123!",
		DisplayName: "First Admin",
	}, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, resp.UserID)

	roles, err := rbacRepo.GetUserRoles(ctx, resp.UserID)
	require.NoError(t, err)
	assert.Contains(t, roles, rbac.RoleSuperAdmin)

	required, err = service.FirstUserSetupRequired(ctx)
	require.NoError(t, err)
	assert.False(t, required)

	_, err = service.Register(ctx, RegisterRequest{
		Email:       "second-user@example.com",
		Password:    "SecurePass123!",
		DisplayName: "Second User",
	}, nil, nil)
	assert.ErrorIs(t, err, ErrSelfSignupDisabled)
}

func TestFirstUserSetupAssignsOnlyOneSuperAdminConcurrently(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool)

	service, _, _ := newPlatformSettingsTestService(t, dbPool)

	const attempts = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := range attempts {
		email := fmt.Sprintf("first-race-%d@example.com", i)
		go func() {
			defer wg.Done()
			<-start
			_, _ = service.Register(ctx, RegisterRequest{
				Email:       email,
				Password:    "SecurePass123!",
				DisplayName: "Race User",
			}, nil, nil)
		}()
	}
	close(start)
	wg.Wait()

	var superAdmins int
	err := dbPool.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = $1
	`, rbac.RoleSuperAdmin).Scan(&superAdmins)
	require.NoError(t, err)
	assert.Equal(t, 1, superAdmins)
}

func TestSuperAdminUpdatesPlatformSettings(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool)

	service, userRepo, _ := newPlatformSettingsTestService(t, dbPool)

	superUser, err := userRepo.CreateUser(ctx, "platform-admin@example.com", "SecurePass123!", "Platform Admin")
	require.NoError(t, err)

	updated, err := service.UpdatePlatformSettings(ctx, superUser.ID, false)
	require.NoError(t, err)
	assert.False(t, updated.SelfSignupEnabled)

	restored, err := service.UpdatePlatformSettings(ctx, superUser.ID, true)
	require.NoError(t, err)
	assert.True(t, restored.SelfSignupEnabled)
}

func newPlatformSettingsTestService(t testing.TB, dbPool *sql.DB) (*Service, *UserRepository, *rbac.RBACRepository) {
	t.Helper()

	config := &Config{JWTAccessTTL: 5 * time.Minute, JWTRefreshTTL: 24 * time.Hour}
	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)
	service.SetPlatformSettingsRepository(NewPlatformSettingsRepository(dbPool))
	return service, userRepo, rbacRepo
}
