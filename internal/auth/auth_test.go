package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecurePass123!"
	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	assert.True(t, VerifyPassword(password, hash))
	assert.False(t, VerifyPassword("WrongPass", hash))
}

func TestJWTManager(t *testing.T) {
	mgr, err := NewJWTManager("", "", 10*time.Minute)
	require.NoError(t, err)

	userID := "user-123"
	email := "test@example.com"
	roles := []string{"analyst", "viewer"}
	workspaceID := "workspace-456"
	datasources := []string{"ds-1", "ds-2"}

	token, err := mgr.GenerateToken(userID, email, roles, workspaceID, datasources)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.Subject)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, workspaceID, claims.WorkspaceID)
	assert.Equal(t, datasources, claims.AccessibleDatasources)

	// Validate public key export
	pubPEM, err := mgr.GetPublicKeyPEM()
	require.NoError(t, err)
	assert.Contains(t, pubPEM, "RSA PUBLIC KEY")
}

func TestJWTManagerUsesBase64PKCS8PrivateKeyEnv(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(privKey)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	t.Setenv("BI_AUTH_JWT_PRIVATE_KEY", base64.StdEncoding.EncodeToString(keyPEM))

	mgr, err := NewJWTManager("", "", 10*time.Minute)
	require.NoError(t, err)

	token, err := mgr.GenerateToken("user-123", "test@example.com", []string{"analyst"}, "workspace-1", nil)
	require.NoError(t, err)

	claims, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, "test@example.com", claims.Email)
}

func TestValidators(t *testing.T) {
	assert.NoError(t, ValidateEmail("test@example.com"))
	assert.Error(t, ValidateEmail("invalid-email"))
	assert.Error(t, ValidateEmail(""))

	assert.NoError(t, ValidatePassword("StrongPass1!"))
	assert.Error(t, ValidatePassword("short"))
	assert.Error(t, ValidatePassword("NoSpecialOrDigit"))
	assert.Error(t, ValidatePassword("lowercaseonly1!"))
}

func TestIntegrationAuthFlow(t *testing.T) {
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"
	}

	dbPool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping integration tests; DB not available:", err)
		return
	}
	defer func() { _ = dbPool.Close() }()

	ctx := context.Background()

	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping integration tests; ping failed:", err)
		return
	}

	// Clear test tables to keep tests clean and repeatable
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config)

	// 1. Register User
	email := "test_auth@example.com"
	displayName := "Test User"
	ua := "Go-Test-Agent"
	ip := "127.0.0.1"

	regResp, err := service.Register(ctx, RegisterRequest{
		Email:       email,
		Password:    "SecurePass123!",
		DisplayName: displayName,
	}, &ua, &ip)
	require.NoError(t, err)
	assert.NotEmpty(t, regResp.AccessToken)
	assert.NotEmpty(t, regResp.RefreshToken)
	assert.Equal(t, email, regResp.Email)

	// 2. Validate registered user structure in DB
	user, err := userRepo.GetUserByID(ctx, regResp.UserID)
	require.NoError(t, err)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, displayName, *user.DisplayName)
	assert.True(t, user.IsActive)

	// 3. Login
	loginResp, err := service.Login(ctx, LoginRequest{
		Email:    email,
		Password: "SecurePass123!",
	}, &ua, &ip)
	require.NoError(t, err)
	assert.NotEmpty(t, loginResp.AccessToken)
	assert.NotEmpty(t, loginResp.RefreshToken)

	// 4. Token Refresh Rotation
	refreshResp, err := service.Refresh(ctx, RefreshRequest{
		RefreshToken: loginResp.RefreshToken,
	}, &ua, &ip)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.NotEmpty(t, refreshResp.RefreshToken)
	assert.NotEqual(t, loginResp.RefreshToken, refreshResp.RefreshToken)

	// 5. Token Family Protection: attempt to reuse the old refresh token
	_, err = service.Refresh(ctx, RefreshRequest{
		RefreshToken: loginResp.RefreshToken,
	}, &ua, &ip)
	assert.ErrorIs(t, err, ErrSessionRevoked)

	// Verify the new token got revoked as well because of family violation
	_, err = service.Refresh(ctx, RefreshRequest{
		RefreshToken: refreshResp.RefreshToken,
	}, &ua, &ip)
	assert.ErrorIs(t, err, ErrSessionRevoked)

	// 6. Logout
	newLogin, err := service.Login(ctx, LoginRequest{
		Email:    email,
		Password: "SecurePass123!",
	}, &ua, &ip)
	require.NoError(t, err)

	err = service.Logout(ctx, newLogin.RefreshToken)
	require.NoError(t, err)

	// Attempt refresh on logged out token
	_, err = service.Refresh(ctx, RefreshRequest{
		RefreshToken: newLogin.RefreshToken,
	}, &ua, &ip)
	assert.ErrorIs(t, err, ErrSessionRevoked)
}

func TestOAuthFlow(t *testing.T) {
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"
	}

	dbPool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping database tests; DB not available:", err)
		return
	}
	defer func() { _ = dbPool.Close() }()

	ctx := context.Background()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
		return
	}

	// Clean tables
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM oauth_accounts")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config)

	// 1. Initial login/register via mock OAuth
	userInfo := &OAuthUserInfo{
		Sub:       "mock-github-12345",
		Email:     "oauth_user@example.com",
		Name:      "OAuth User",
		AvatarURL: "https://mock.com/avatar.png",
	}

	token := &oauth2.Token{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}

	ua := "Go-Test-Agent"
	ip := "127.0.0.1"

	// Call service OAuth method
	resp, err := service.LoginOrRegisterOAuth(ctx, "github", token, userInfo, &ua, &ip)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, userInfo.Email, resp.Email)

	// Verify DB state
	userID, err := userRepo.GetOAuthAccount(ctx, "github", userInfo.Sub)
	require.NoError(t, err)
	assert.Equal(t, resp.UserID, userID)

	// 2. Existing user OAuth callback (login path)
	resp2, err := service.LoginOrRegisterOAuth(ctx, "github", token, userInfo, &ua, &ip)
	require.NoError(t, err)
	assert.Equal(t, resp.UserID, resp2.UserID)
	assert.NotEqual(t, resp.RefreshToken, resp2.RefreshToken)
}

func TestWebAuthnFlow(t *testing.T) {
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"
	}

	dbPool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping database tests; DB not available:", err)
		return
	}
	defer func() { _ = dbPool.Close() }()

	ctx := context.Background()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
		return
	}

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM passkeys")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM webauthn_challenges")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		WebAuthnRPID:    "localhost",
		WebAuthnRPName:  "Biqly Test",
		WebAuthnOrigins: []string{"http://localhost:5173"},
	}

	userRepo := NewUserRepository(dbPool)
	waService, err := NewWebAuthnService(config, userRepo)
	require.NoError(t, err)

	user, err := userRepo.CreateUser(ctx, "webauthn@example.com", "SomePassword1!", "WebAuthn User")
	require.NoError(t, err)

	creation, session, err := waService.BeginRegistration(ctx, user)
	require.NoError(t, err)
	assert.NotEmpty(t, creation.Response.Challenge)
	assert.NotEmpty(t, session.Challenge)

	challengeBytes, err := base64.RawURLEncoding.DecodeString(session.Challenge)
	if err != nil {
		challengeBytes, err = base64.URLEncoding.DecodeString(session.Challenge)
		require.NoError(t, err)
	}

	uid, err := userRepo.GetWebAuthnChallenge(ctx, challengeBytes)
	require.NoError(t, err)
	require.NotNil(t, uid)
	assert.Equal(t, user.ID, *uid)

	uid2, err := userRepo.GetWebAuthnChallenge(ctx, challengeBytes)
	require.NoError(t, err)
	assert.Nil(t, uid2)

	mockCred := &webauthn.Credential{
		ID:              []byte("mock-credential-id"),
		PublicKey:       []byte("mock-public-key"),
		AttestationType: "none",
		Transport:       []protocol.AuthenticatorTransport{protocol.USB, protocol.Internal},
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SignCount: 42,
		},
	}

	err = userRepo.SavePasskey(ctx, user.ID, mockCred, "My Key")
	require.NoError(t, err)

	creds, err := userRepo.GetPasskeysByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, mockCred.ID, creds[0].ID)
	assert.Equal(t, mockCred.PublicKey, creds[0].PublicKey)
	assert.Equal(t, mockCred.AttestationType, creds[0].AttestationType)
	assert.Equal(t, mockCred.Authenticator.SignCount, creds[0].Authenticator.SignCount)

	userPasskeys, err := waService.GetUserPasskeys(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, userPasskeys, 1)
	assert.Equal(t, "My Key", userPasskeys[0].Name)
	assert.NotEmpty(t, userPasskeys[0].ID)

	assertion, sessionLogin, err := waService.BeginLogin(ctx, user.Email)
	require.NoError(t, err)
	assert.NotEmpty(t, assertion.Response.Challenge)
	assert.NotEmpty(t, sessionLogin.Challenge)

	err = userRepo.UpdatePasskeySignCount(ctx, mockCred.ID, 45)
	require.NoError(t, err)

	creds2, err := userRepo.GetPasskeysByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, uint32(45), creds2[0].Authenticator.SignCount)

	err = waService.DeletePasskey(ctx, user.ID, userPasskeys[0].ID)
	require.NoError(t, err)

	userPasskeys2, err := waService.GetUserPasskeys(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, userPasskeys2, 0)
}
