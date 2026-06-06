package auth

import (
	"context"
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

	_, _ = dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = false WHERE id = 1")

	config := &Config{JWTAccessTTL: 5 * time.Minute, JWTRefreshTTL: 24 * time.Hour}
	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)
	service.SetPlatformSettingsRepository(NewPlatformSettingsRepository(dbPool))

	_, err = service.Register(ctx, RegisterRequest{
		Email:       "newuser@example.com",
		Password:    "SecurePass123!",
		DisplayName: "New User",
	}, nil, nil)
	assert.ErrorIs(t, err, ErrSelfSignupDisabled)

	_, _ = dbPool.ExecContext(ctx, "UPDATE platform_settings SET self_signup_enabled = true WHERE id = 1")
}

func TestSuperAdminUpdatesPlatformSettings(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	config := &Config{JWTAccessTTL: 5 * time.Minute, JWTRefreshTTL: 24 * time.Hour}
	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)
	service.SetPlatformSettingsRepository(NewPlatformSettingsRepository(dbPool))

	superUser, err := userRepo.CreateUser(ctx, "platform-admin@example.com", "SecurePass123!", "Platform Admin")
	require.NoError(t, err)

	var superRoleID string
	err = dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin).Scan(&superRoleID)
	require.NoError(t, err)
	_, err = dbPool.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, superUser.ID, superRoleID)
	require.NoError(t, err)

	updated, err := service.UpdatePlatformSettings(ctx, superUser.ID, false)
	require.NoError(t, err)
	assert.False(t, updated.SelfSignupEnabled)

	restored, err := service.UpdatePlatformSettings(ctx, superUser.ID, true)
	require.NoError(t, err)
	assert.True(t, restored.SelfSignupEnabled)
}
