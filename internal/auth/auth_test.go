package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/testutil"
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

func TestNormalizeEmailGmailCanonicalization(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Bob.Smith+promo@gmail.com", "bobsmith@gmail.com"},
		{"b.o.b.smith@googlemail.com", "bobsmith@gmail.com"},
		{"alice+anything@gmail.com", "alice@gmail.com"},
		{"Alice@Example.COM", "alice@example.com"},
		{"plain.name+tag@example.com", "plain.name+tag@example.com"},
	}
	for _, tc := range cases {
		got, err := NormalizeEmail(tc.in)
		require.NoError(t, err, "input=%q", tc.in)
		assert.Equal(t, tc.want, got, "input=%q", tc.in)
	}

	// NFKC: fullwidth @ collapses to ASCII @.
	nfkc, err := NormalizeEmail("alice＠example.com")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", nfkc)

	// Gmail local stripped to empty must be rejected.
	_, err = NormalizeEmail("...@gmail.com")
	assert.Error(t, err)
}

func TestEmailChangeMigrationHasDoubleVerificationAndWaitPeriod(t *testing.T) {
	up, err := os.ReadFile("../../migrations/auth/023a_create_email_change_requests.up.sql")
	require.NoError(t, err)

	sqlStr := string(up)
	for _, fragment := range []string{
		"old_email_token",
		"new_email_token",
		"old_email_confirmed_at",
		"new_email_confirmed_at",
		"not_before",
		"completed_at",
		"new_email",
	} {
		assert.Contains(t, sqlStr, fragment)
	}
	assert.True(t, strings.Contains(sqlStr, "UNIQUE (old_email_token)") || strings.Contains(sqlStr, "old_email_token TEXT NOT NULL UNIQUE"))
	assert.True(t, strings.Contains(sqlStr, "UNIQUE (new_email_token)") || strings.Contains(sqlStr, "new_email_token TEXT NOT NULL UNIQUE"))
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

	sqlStr := string(up)
	for _, fragment := range []string{
		"password_history",
		"user_id",
		"password_hash",
		"created_at",
		"idx_password_history_user_created",
	} {
		assert.Contains(t, sqlStr, fragment)
	}
	assert.Equal(t, 5, PasswordHistoryLimit)
	assert.ErrorIs(t, ErrPasswordReused, ErrPasswordReused)
}

func TestIntegrationAuthFlow(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean and repeatable
	testutil.ResetAuthUserTables(ctx, t, dbPool)

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
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

	// 2b. Duplicate registration must not leak account existence: same generic
	// response shape (200, verification_pending) with no tokens, regardless of
	// whether the email is fresh or already taken.
	mockMailer := mail.NewMockEmailSender()
	enumService := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, mockMailer)
	dupResp, err := enumService.Register(ctx, RegisterRequest{
		Email:       email,
		Password:    "SecurePass123!",
		DisplayName: displayName,
	}, &ua, &ip)
	require.NoError(t, err)
	assert.True(t, dupResp.VerificationPending)
	assert.Empty(t, dupResp.AccessToken)
	assert.Empty(t, dupResp.RefreshToken)
	assert.Empty(t, dupResp.UserID)
	require.NotEmpty(t, mockMailer.SentEmails[email], "duplicate registration must email the existing owner")
	assert.Contains(t, mockMailer.SentEmails[email][0], "Duplicate registration")

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
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	// Clean tables
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool, "DELETE FROM oauth_accounts")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
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
	defer func() {
		err := rClient.Close()
		assert.NoError(t, err)
	}()

	email := "brute@example.com"
	rClient.Del(ctx, "login_failures:"+email)

	dbPool := testutil.OpenAuthDB(t)

	testutil.ExecAuthSQL(ctx, t, dbPool, "DELETE FROM sessions", "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}
	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, rClient, nil)

	_, err = userRepo.CreateUser(ctx, email, "SomeHash", "Brute User")
	require.NoError(t, err)

	ua := "Test"
	ip := "127.0.0.1"
	for range 5 {
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
	defer func() {
		err := rClient.Close()
		assert.NoError(t, err)
	}()

	ip := "127.0.0.1"
	bucket := time.Now().Unix() / 60
	key := fmt.Sprintf("ratelimit:test:%s:%d", ip, bucket)
	rClient.Del(ctx, key)

	limiter := NewRateLimiter(rClient)
	handler := limiter.Limit(2, 1*time.Minute, "test")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)
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
	handler := CSRF(false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqGET, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	require.NoError(t, err)
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
	assert.True(t, csrfCookie.HttpOnly)

	headerToken := rrGET.Header().Get("X-CSRF-Token")
	require.NotEmpty(t, headerToken)
	assert.Equal(t, csrfCookie.Value, headerToken)

	reqPOSTNoCookie, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", http.NoBody)
	require.NoError(t, err)
	rrPOSTNoCookie := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTNoCookie, reqPOSTNoCookie)
	assert.Equal(t, http.StatusForbidden, rrPOSTNoCookie.Code)

	reqPOSTNoHeader, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", http.NoBody)
	require.NoError(t, err)
	reqPOSTNoHeader.AddCookie(csrfCookie)
	rrPOSTNoHeader := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTNoHeader, reqPOSTNoHeader)
	assert.Equal(t, http.StatusForbidden, rrPOSTNoHeader.Code)

	reqPOSTSuccess, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", http.NoBody)
	require.NoError(t, err)
	reqPOSTSuccess.AddCookie(csrfCookie)
	reqPOSTSuccess.Header.Set("X-CSRF-Token", headerToken)
	rrPOSTSuccess := httptest.NewRecorder()
	handler.ServeHTTP(rrPOSTSuccess, reqPOSTSuccess)
	assert.Equal(t, http.StatusOK, rrPOSTSuccess.Code)
}

func TestEmailVerificationAndReset(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	testutil.ExecAuthSQL(ctx, t, dbPool,
		"DELETE FROM email_verification_tokens",
		"DELETE FROM password_reset_tokens",
		"DELETE FROM users",
	)

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}
	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	mockSender := mail.NewMockEmailSender()

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

func TestJWTManagerProductionFailFast(t *testing.T) {
	t.Setenv("BI_ENV", "production")
	_, err := NewJWTManager("", "", 10*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT keys are unconfigured under production environment")
}
