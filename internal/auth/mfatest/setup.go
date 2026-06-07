package mfatest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type IntegrationStack struct {
	DB       *sql.DB
	Ctx      context.Context
	Config   *auth.Config
	JWT      *auth.JWTManager
	UserRepo *auth.UserRepository
	RBACRepo *rbac.RBACRepository
	Sessions *auth.SessionManager
	Auth     *auth.Service
	MFARepo  *mfa.Repository
	MFA      *mfa.Service
}

type BypassTestUsers struct {
	TargetUserID  string
	TargetEmail   string
	NormalActorID string
	SuperActorID  string
}

func uniqueTestEmail(t *testing.T, local string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s@example.com", local, uuid.NewString()[:8])
}

func deleteTestUsers(ctx context.Context, t *testing.T, db *sql.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id); err != nil {
			t.Fatalf("cleanup test user %s: %v", id, err)
		}
	}
}

func NewIntegrationStack(t *testing.T) *IntegrationStack {
	t.Helper()

	db := testutil.OpenAuthDB(t)
	ctx := context.Background()

	config := &auth.Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := auth.NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := auth.NewUserRepository(db, nil)
	rbacRepo := rbac.NewRBACRepository(db)
	sessionMgr := auth.NewSessionManager(db)
	service := auth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

	mfaRepo := mfa.NewMFARepository(db, nil)
	mfaSvc := mfa.NewMFAService(mfaRepo, userRepo, "Biqly")
	service.SetMFAService(mfaSvc)

	return &IntegrationStack{
		DB:       db,
		Ctx:      ctx,
		Config:   config,
		JWT:      jwtMgr,
		UserRepo: userRepo,
		RBACRepo: rbacRepo,
		Sessions: sessionMgr,
		Auth:     service,
		MFARepo:  mfaRepo,
		MFA:      mfaSvc,
	}
}

func (s *IntegrationStack) SeedBypassTestUsers(t *testing.T) BypassTestUsers {
	t.Helper()

	var users BypassTestUsers
	users.TargetEmail = uniqueTestEmail(t, "target")
	normalEmail := uniqueTestEmail(t, "normal")
	superEmail := uniqueTestEmail(t, "super")

	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Target User', 'hash', TRUE) RETURNING id`, users.TargetEmail,
	).Scan(&users.TargetUserID))

	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Normal User', 'hash', TRUE) RETURNING id`, normalEmail,
	).Scan(&users.NormalActorID))

	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Super Admin User', 'hash', TRUE) RETURNING id`, superEmail,
	).Scan(&users.SuperActorID))

	var saRoleID string
	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin,
	).Scan(&saRoleID))
	require.NoError(t, s.RBACRepo.AssignRole(s.Ctx, users.SuperActorID, saRoleID, nil, nil))

	t.Cleanup(func() {
		deleteTestUsers(s.Ctx, t, s.DB, users.TargetUserID, users.NormalActorID, users.SuperActorID)
	})

	return users
}

func EnableVerifiedMFA(ctx context.Context, t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_mfa (user_id, method, secret_encrypted, recovery_codes, bypass_codes, enabled, verified_at, updated_at)
		VALUES ($1, 'totp', 'secret', '{}', '{}', TRUE, NOW(), NOW())
	`, userID)
	require.NoError(t, err)
}
