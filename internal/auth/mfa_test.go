package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFABypassCodeFlow(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_mfa")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

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
		`SELECT id FROM roles WHERE name = $1`, RoleSuperAdmin,
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
	assert.ErrorIs(t, err, ErrSuperAdminRequired)

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

func TestAdminGenerateMFABypassHandler(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_mfa")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("test-issuer", "test-audience", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

	mfaRepo := NewMFARepository(dbPool, nil)
	mfaSvc := NewMFAService(mfaRepo, userRepo, "Biqly")
	service.SetMFAService(mfaSvc)

	handler := NewAuthHandler(service, nil, jwtMgr, config, nil)
	handler.SetMFA(mfaSvc)

	// Create a target user
	email := "target@example.com"
	var targetUserID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Target User', 'hash', TRUE) RETURNING id`, email,
	).Scan(&targetUserID))

	// Enable MFA for target user
	_, err = dbPool.ExecContext(ctx, `
		INSERT INTO user_mfa (user_id, method, secret_encrypted, recovery_codes, bypass_codes, enabled, verified_at, updated_at)
		VALUES ($1, 'totp', 'secret', '{}', '{}', TRUE, NOW(), NOW())
	`, targetUserID)
	require.NoError(t, err)

	// Create a super admin actor user
	var superActorID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('super@example.com', 'Super Admin User', 'hash', TRUE) RETURNING id`,
	).Scan(&superActorID))

	var saRoleID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = $1`, RoleSuperAdmin,
	).Scan(&saRoleID))
	require.NoError(t, rbacRepo.AssignRole(ctx, superActorID, saRoleID, nil, nil))

	// Create a normal actor user
	var normalActorID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('normal@example.com', 'Normal User', 'hash', TRUE) RETURNING id`,
	).Scan(&normalActorID))

	// Test route registration & HTTP call
	router := chi.NewRouter()
	authMW := func(next http.Handler) http.Handler {
		return next
	}
	handler.RegisterAccountAdminRoutes(router, authMW)

	// Case 1: Call as normal actor -> Should return 403 Forbidden
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetUserID+"/mfa/bypass", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, normalActorID))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)

	// Case 2: Call as super admin -> Should return 200 OK with bypass code
	req = httptest.NewRequest(http.MethodPost, "/admin/users/"+targetUserID+"/mfa/bypass", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, superActorID))
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	bypassCode := resp["bypass_code"]
	assert.True(t, strings.HasPrefix(bypassCode, "BYPASS-"))
}
