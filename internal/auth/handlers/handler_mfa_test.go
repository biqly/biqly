package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/go-chi/chi/v5"
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
	t.Cleanup(func() { _ = dbPool.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
	}
	return dbPool
}

func TestAdminGenerateMFABypassHandler(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_mfa")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &auth.Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := auth.NewJWTManager("test-issuer", "test-audience", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := auth.NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := auth.NewSessionManager(dbPool)
	service := auth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

	mfaRepo := mfa.NewMFARepository(dbPool, nil)
	mfaSvc := mfa.NewMFAService(mfaRepo, userRepo, "Biqly")
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
		`SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin,
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
	req := httptest.NewRequestWithContext(context.WithValue(ctx, userIDKey, normalActorID), http.MethodPost, "/admin/users/"+targetUserID+"/mfa/bypass", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)

	// Case 2: Call as super admin -> Should return 200 OK with bypass code
	req = httptest.NewRequestWithContext(context.WithValue(ctx, userIDKey, superActorID), http.MethodPost, "/admin/users/"+targetUserID+"/mfa/bypass", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	bypassCode := resp["bypass_code"]
	assert.True(t, strings.HasPrefix(bypassCode, "BYPASS-"))
}
