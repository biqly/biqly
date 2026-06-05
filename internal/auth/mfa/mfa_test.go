package mfa

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/rbac"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	t.Cleanup(func() {
		if err := dbPool.Close(); err != nil {
			t.Errorf("dbPool.Close() error = %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
	}
	return dbPool
}

func TestMFABypassCodeFlow(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean
	_, err := dbPool.ExecContext(ctx, "DELETE FROM user_mfa")
	require.NoError(t, err)
	_, err = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	require.NoError(t, err)
	_, err = dbPool.ExecContext(ctx, "DELETE FROM users")
	require.NoError(t, err)

	config := &auth.Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := auth.NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := auth.NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := auth.NewSessionManager(dbPool)
	service := auth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

	mfaRepo := NewMFARepository(dbPool, nil)
	mfaSvc := NewMFAService(mfaRepo, userRepo, "Biqly")
	service.SetMFAService(mfaSvc)

	// Create a target user
	email := "target@example.com"
	var targetUserID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Target User', 'hash', TRUE) RETURNING id`, email,
	).Scan(&targetUserID))

	// Create a normal actor user
	var normalActorID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('normal@example.com', 'Normal User', 'hash', TRUE) RETURNING id`,
	).Scan(&normalActorID))

	// Create a super admin actor user
	var superActorID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('super@example.com', 'Super Admin User', 'hash', TRUE) RETURNING id`,
	).Scan(&superActorID))

	var saRoleID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin,
	).Scan(&saRoleID))
	require.NoError(t, rbacRepo.AssignRole(ctx, superActorID, saRoleID, nil, nil))

	// 1. Try to generate bypass code when user has not enrolled in MFA at all
	_, err = service.GenerateMFABypassCode(ctx, superActorID, targetUserID)
	assert.ErrorIs(t, err, ErrMFANotEnrolled)

	// Enroll target user in MFA
	enrollResult, err := mfaSvc.Enroll(ctx, targetUserID, email)
	require.NoError(t, err)

	// 2. Try to generate bypass code when MFA enrollment is pending (not verified yet)
	_, err = service.GenerateMFABypassCode(ctx, superActorID, targetUserID)
	assert.ErrorIs(t, err, ErrMFANotEnabled)

	// Verify and enable MFA
	// Since we don't have a real time TOTP code for the secret in tests easily, let's update user_mfa directly to enabled
	_, err = dbPool.ExecContext(ctx, "UPDATE user_mfa SET enabled = TRUE, verified_at = NOW() WHERE user_id = $1", targetUserID)
	require.NoError(t, err)

	// 3. Try to generate bypass code as a normal user (not super admin)
	_, err = service.GenerateMFABypassCode(ctx, normalActorID, targetUserID)
	assert.ErrorIs(t, err, auth.ErrSuperAdminRequired)

	// 4. Generate bypass code as a super admin
	bypassCode, err := service.GenerateMFABypassCode(ctx, superActorID, targetUserID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(bypassCode, "BYPASS-"))
	assert.Len(t, bypassCode, 15) // BYPASS- (7 chars) + 8 chars Base32 = 15 chars

	// 5. Verify case-insensitive, padded bypass code verification works
	paddedBypassCode := "  " + strings.ToLower(bypassCode) + "  "
	err = mfaSvc.VerifyCode(ctx, targetUserID, paddedBypassCode)
	require.NoError(t, err)

	// 6. Verification code is single-use, so verifying it again should fail
	err = mfaSvc.VerifyCode(ctx, targetUserID, bypassCode)
	assert.ErrorIs(t, err, ErrMFACodeInvalid)

	// 7. Verify TOTP secret is intact after bypass code consumption
	enrollment, err := mfaRepo.Get(ctx, targetUserID)
	require.NoError(t, err)
	assert.Equal(t, enrollResult.Secret, enrollment.Secret)
}
