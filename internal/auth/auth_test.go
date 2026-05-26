package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
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

func TestJWTManagerSetsIssuerAudienceJTI(t *testing.T) {
	t.Setenv("BI_AUTH_JWT_ISSUER", "test-issuer")
	t.Setenv("BI_AUTH_JWT_AUDIENCE", "test-audience")

	mgr, err := NewJWTManager("", "", 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "test-issuer", mgr.Issuer())
	assert.Equal(t, "test-audience", mgr.Audience())

	token, err := mgr.GenerateToken("u-1", "u1@example.com", nil, "", nil)
	require.NoError(t, err)

	first, err := mgr.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "test-issuer", first.Issuer)
	require.Len(t, first.Audience, 1)
	assert.Equal(t, "test-audience", first.Audience[0])
	assert.NotEmpty(t, first.ID, "jti must be populated")

	// jti must change between tokens to enable revocation lists.
	token2, err := mgr.GenerateToken("u-1", "u1@example.com", nil, "", nil)
	require.NoError(t, err)
	second, err := mgr.ValidateToken(token2)
	require.NoError(t, err)
	assert.NotEqual(t, first.ID, second.ID)
}

func TestJWTManagerRejectsWrongAudience(t *testing.T) {
	t.Setenv("BI_AUTH_JWT_ISSUER", "real-issuer")
	t.Setenv("BI_AUTH_JWT_AUDIENCE", "real-aud")
	mgrReal, err := NewJWTManager("", "", 10*time.Minute)
	require.NoError(t, err)

	token, err := mgrReal.GenerateToken("u-1", "u1@example.com", nil, "", nil)
	require.NoError(t, err)

	// Validator configured for a different audience must reject.
	t.Setenv("BI_AUTH_JWT_AUDIENCE", "other-aud")
	mgrOther := *mgrReal
	mgrOther.audience = "other-aud"
	_, err = mgrOther.ValidateToken(token)
	require.Error(t, err)

	mgrOther.audience = "real-aud"
	mgrOther.issuer = "wrong-issuer"
	_, err = mgrOther.ValidateToken(token)
	require.Error(t, err)
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

func TestDummyBcryptHashSeeded(t *testing.T) {
	// dummyBcryptHash must be initialized at package init so timing-attack
	// mitigation has CPU work to spend on missing accounts.
	assert.NotEmpty(t, dummyBcryptHash)
	// Verify it cannot be matched by a guessable password — keeps the dummy
	// hash from accidentally accepting any input.
	assert.False(t, VerifyPassword("password", dummyBcryptHash))
	assert.False(t, VerifyPassword("", dummyBcryptHash))

	// VerifyDummyPassword must not panic for arbitrary input.
	VerifyDummyPassword("anything")
	VerifyDummyPassword("")
}

func TestValidators(t *testing.T) {
	assert.NoError(t, ValidateEmail("test@example.com"))
	assert.Error(t, ValidateEmail("invalid-email"))
	assert.Error(t, ValidateEmail(""))
	assert.Error(t, ValidateEmail(`Evil <test@example.com>`))
	assert.Error(t, ValidateEmail("<script>@example.com"))

	assert.NoError(t, ValidatePassword("StrongPass1!"))
	assert.Error(t, ValidatePassword("short"))
	assert.Error(t, ValidatePassword("NoSpecialOrDigit"))
	assert.Error(t, ValidatePassword("lowercaseonly1!"))

	email, err := NormalizeEmail("  TEST@Example.COM  ")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", email)

	displayName, err := SanitizeDisplayName("  Ada Lovelace  ")
	require.NoError(t, err)
	assert.Equal(t, "Ada Lovelace", displayName)
	assert.EqualError(t, ValidateDisplayName(`<img src=x onerror=alert(1)>`), "display name contains unsupported characters")
	assert.EqualError(t, ValidateDisplayName("Ada\x00Lovelace"), "display name contains unsupported characters")
}

func TestEmailChangeMigrationHasDoubleVerificationAndWaitPeriod(t *testing.T) {
	up, err := os.ReadFile("../../migrations/auth/023a_create_email_change_requests.up.sql")
	require.NoError(t, err)

	sql := string(up)
	for _, fragment := range []string{
		"old_email_token",
		"new_email_token",
		"old_email_confirmed_at",
		"new_email_confirmed_at",
		"not_before",
		"completed_at",
		"new_email",
	} {
		assert.Contains(t, sql, fragment)
	}
	assert.True(t, strings.Contains(sql, "UNIQUE (old_email_token)") || strings.Contains(sql, "old_email_token TEXT NOT NULL UNIQUE"))
	assert.True(t, strings.Contains(sql, "UNIQUE (new_email_token)") || strings.Contains(sql, "new_email_token TEXT NOT NULL UNIQUE"))
}

func TestEmailChangeContract(t *testing.T) {
	assert.Equal(t, 24*time.Hour, EmailChangeWaitPeriod)
	assert.Equal(t, 48*time.Hour, EmailChangeTokenTTL)

	req := EmailChangeRequest{
		UserID:   "user-1",
		OldEmail: "old@example.com",
		NewEmail: "new@example.com",
	}
	assert.Equal(t, "user-1", req.UserID)
	assert.Equal(t, "new@example.com", req.NewEmail)
}

func TestPasswordHistoryMigrationAndContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/auth/024a_create_password_history.up.sql")
	require.NoError(t, err)

	sql := string(up)
	for _, fragment := range []string{
		"password_history",
		"user_id",
		"password_hash",
		"created_at",
		"idx_password_history_user_created",
	} {
		assert.Contains(t, sql, fragment)
	}
	assert.Equal(t, 5, PasswordHistoryLimit)
	assert.ErrorIs(t, ErrPasswordReused, ErrPasswordReused)
}

func openTestDBPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec
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

func TestIntegrationAuthFlow(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

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

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

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
	dbPool := openTestDBPool(t)
	ctx := context.Background()

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

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, nil)

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
	dbPool := openTestDBPool(t)
	ctx := context.Background()

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

	userRepo := NewUserRepository(dbPool, nil)
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

func TestBruteForceLockout(t *testing.T) {
	redisDSN := os.Getenv("BI_AUTH_REDIS_DSN")
	if redisDSN == "" {
		redisDSN = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(redisDSN)
	if err != nil {
		t.Skip("skipping redis-dependent test: invalid redis URL:", err)
		return
	}
	rClient := redis.NewClient(opts)
	ctx := context.Background()
	if err := rClient.Ping(ctx).Err(); err != nil {
		t.Skip("skipping redis-dependent test: redis not available:", err)
		return
	}
	defer func() { _ = rClient.Close() }()

	email := "brute@example.com"
	rClient.Del(ctx, "login_failures:"+email)

	dbPool := openTestDBPool(t)

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
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
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, rClient, nil)

	_, err = userRepo.CreateUser(ctx, email, "SomeHash", "Brute User")
	require.NoError(t, err)

	ua := "Test"
	ip := "127.0.0.1"
	for i := 0; i < 5; i++ {
		_, err := service.Login(ctx, LoginRequest{Email: email, Password: "wrong-password"}, &ua, &ip)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	}

	_, err = service.Login(ctx, LoginRequest{Email: email, Password: "wrong-password"}, &ua, &ip)
	assert.ErrorIs(t, err, ErrAccountLocked)

	rClient.Del(ctx, "login_failures:"+email)
}

func TestRateLimiting(t *testing.T) {
	redisDSN := os.Getenv("BI_AUTH_REDIS_DSN")
	if redisDSN == "" {
		redisDSN = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(redisDSN)
	if err != nil {
		t.Skip("skipping redis-dependent test: invalid redis URL:", err)
		return
	}
	rClient := redis.NewClient(opts)
	ctx := context.Background()
	if err := rClient.Ping(ctx).Err(); err != nil {
		t.Skip("skipping redis-dependent test: redis not available:", err)
		return
	}
	defer func() { _ = rClient.Close() }()

	ip := "127.0.0.1"
	bucket := time.Now().Unix() / 60
	key := fmt.Sprintf("ratelimit:test:%s:%d", ip, bucket)
	rClient.Del(ctx, key)

	limiter := NewRateLimiter(rClient)
	handler := limiter.Limit(2, 1*time.Minute, "test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req)
	assert.Equal(t, http.StatusOK, rr1.Code)

	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	assert.Equal(t, http.StatusOK, rr2.Code)

	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req)
	assert.Equal(t, http.StatusTooManyRequests, rr3.Code)

	rClient.Del(ctx, key)
}

func TestCSRF(t *testing.T) {
	ctx := context.Background()
	handler := CSRF(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqGET, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	rrGET := httptest.NewRecorder()
	handler.ServeHTTP(rrGET, reqGET)
	assert.Equal(t, http.StatusOK, rrGET.Code)

	cookies := rrGET.Result().Cookies()
	var csrfCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "csrf_token" {
			csrfCookie = c
			break
		}
	}
	require.NotNil(t, csrfCookie)
	assert.NotEmpty(t, csrfCookie.Value)

	reqPOSTNoCookie, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	rrPOSTNoCookie := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTNoCookie, reqPOSTNoCookie)
	assert.Equal(t, http.StatusForbidden, rrPOSTNoCookie.Code)

	reqPOSTNoHeader, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	reqPOSTNoHeader.AddCookie(csrfCookie)
	rrPOSTNoHeader := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTNoHeader, reqPOSTNoHeader)
	assert.Equal(t, http.StatusForbidden, rrPOSTNoHeader.Code)

	reqPOSTSuccess, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
	reqPOSTSuccess.AddCookie(csrfCookie)
	reqPOSTSuccess.Header.Set("X-CSRF-Token", csrfCookie.Value)
	rrPOSTSuccess := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTSuccess, reqPOSTSuccess)
	assert.Equal(t, http.StatusOK, rrPOSTSuccess.Code)
}

func TestEmailVerificationAndReset(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM email_verification_tokens")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM password_reset_tokens")
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
	mockSender := NewMockEmailSender()

	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, mockSender)

	email := "verify@example.com"
	reg, err := service.Register(ctx, RegisterRequest{
		Email:       email,
		Password:    "SecurePass1!",
		DisplayName: "Verify User",
	}, nil, nil)
	require.NoError(t, err)

	require.Contains(t, mockSender.SentEmails, email)
	assert.Len(t, mockSender.SentEmails[email], 1)
	assert.Contains(t, mockSender.SentEmails[email][0], "Verification token:")

	msg := mockSender.SentEmails[email][0]
	token := msg[len("Verification token: "):]

	u, err := userRepo.GetUserByID(ctx, reg.UserID)
	require.NoError(t, err)
	assert.False(t, u.EmailVerified)

	err = service.VerifyEmail(ctx, token)
	require.NoError(t, err)

	u, err = userRepo.GetUserByID(ctx, reg.UserID)
	require.NoError(t, err)
	assert.True(t, u.EmailVerified)

	err = service.ForgotPassword(ctx, email)
	require.NoError(t, err)
	assert.Len(t, mockSender.SentEmails[email], 2)
	assert.Contains(t, mockSender.SentEmails[email][1], "Reset token:")

	resetMsg := mockSender.SentEmails[email][1]
	resetToken := resetMsg[len("Reset token: "):]

	err = service.ResetPassword(ctx, resetToken, "NewSecurePass2!")
	require.NoError(t, err)

	loginResp, err := service.Login(ctx, LoginRequest{
		Email:    email,
		Password: "NewSecurePass2!",
	}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, reg.UserID, loginResp.UserID)
}
