package mfatest

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
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
	NormalActorID string
	SuperActorID  string
}

func ResetCoreAuthTables(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()
	testutil.ExecAuthSQL(ctx, t, db,
		"DELETE FROM user_mfa",
		"DELETE FROM user_roles",
		"DELETE FROM users",
	)
}

func NewIntegrationStack(t *testing.T) *IntegrationStack {
	t.Helper()

	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	ResetCoreAuthTables(ctx, t, db)

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
	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Target User', 'hash', TRUE) RETURNING id`, "target@example.com",
	).Scan(&users.TargetUserID))

	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('normal@example.com', 'Normal User', 'hash', TRUE) RETURNING id`,
	).Scan(&users.NormalActorID))

	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ('super@example.com', 'Super Admin User', 'hash', TRUE) RETURNING id`,
	).Scan(&users.SuperActorID))

	var saRoleID string
	require.NoError(t, s.DB.QueryRowContext(s.Ctx,
		`SELECT id FROM roles WHERE name = $1`, rbac.RoleSuperAdmin,
	).Scan(&saRoleID))
	require.NoError(t, s.RBACRepo.AssignRole(s.Ctx, users.SuperActorID, saRoleID, nil, nil))

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
