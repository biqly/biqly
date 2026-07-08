// Package auth implements authentication, sessions, workspaces, and user lifecycle.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/auth/ldap"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/mail"
)

var (
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrInactiveUser          = errors.New("user account is deactivated")
	ErrAccountLocked         = errors.New("too many failed login attempts; account is temporarily locked for 15 minutes")
	ErrMFARequired           = errors.New("mfa required for active workspace")
	ErrEmailChangePending    = errors.New("email change confirmation pending")
	ErrPasswordReused        = errors.New("password was recently used")
	ErrNoPasswordSet         = errors.New("password login is not enabled for this account")
	ErrSuperAdminRequired    = errors.New("super admin privilege required")
	ErrSelfSignupDisabled    = errors.New("self-service registration is disabled")
	ErrMFANotEnabled         = errors.New("mfa not enabled")
	ErrNotWorkspaceOwner     = errors.New("not workspace owner")
	ErrMFAVerificationLocked = errors.New("too many failed MFA verification attempts; temporarily locked for 15 minutes")
)

const PasswordHistoryLimit = 5

// WorkspaceService defines the workspace operations needed by AuthService.
type WorkspaceService interface {
	IsMFARequired(ctx context.Context, workspaceID string) (bool, error)
	IsMember(ctx context.Context, workspaceID, userID string) (bool, error)
}

// MFAService defines the multi-factor authentication operations needed by AuthService.
type MFAService interface {
	IsEnabled(ctx context.Context, userID string) (bool, error)
	VerifyCode(ctx context.Context, userID, code string) error
	GenerateBypassCode(ctx context.Context, userID string) (string, error)
}

type Service struct {
	userRepo         *UserRepository
	rbacRepo         *rbac.RBACRepository
	sessionMgr       *SessionManager
	jwtMgr           *JWTManager
	config           *Config
	redisClient      *redis.Client
	emailSender      mail.EmailSender
	workspaceSvc     WorkspaceService
	mfaSvc           MFAService
	magicLinks       *MagicLinkRepository
	platformSettings *PlatformSettingsRepository
	ldapConfig       *LDAPConfigRepository
	ldapAuth         ldap.Authenticator
	auditSvc         *AuditService
	patMgr           *PersonalAccessTokenManager
}

func (s *Service) UserRepo() *UserRepository {
	return s.userRepo
}

func (s *Service) RBACRepo() *rbac.RBACRepository {
	return s.rbacRepo
}

// SetMagicLinkRepository wires the magic-link repository post-construction.
// Optional; if unset the magic-link endpoints reply as if the address has
// no account, preventing enumeration when the feature is disabled.
func (s *Service) SetMagicLinkRepository(r *MagicLinkRepository) { s.magicLinks = r }

// SetWorkspaceService wires the workspace service after construction to avoid
// a constructor-arg ripple through tests; required for active-workspace switching.
func (s *Service) SetWorkspaceService(ws WorkspaceService) { s.workspaceSvc = ws }

// SetMFAService wires the MFA service after construction. Optional; if unset
// MFA checks are skipped and login proceeds with single factor.
func (s *Service) SetMFAService(m MFAService) { s.mfaSvc = m }

// SetPersonalAccessTokenManager wires the personal access token manager after
// construction. Optional; if unset, personal-access-token endpoints return
// ErrPersonalAccessTokensUnavailable.
func (s *Service) SetPersonalAccessTokenManager(m *PersonalAccessTokenManager) { s.patMgr = m }

func NewAuthService(
	userRepo *UserRepository,
	rbacRepo *rbac.RBACRepository,
	sessionMgr *SessionManager,
	jwtMgr *JWTManager,
	config *Config,
	redisClient *redis.Client,
	emailSender mail.EmailSender,
) *Service {
	return &Service{
		userRepo:    userRepo,
		rbacRepo:    rbacRepo,
		sessionMgr:  sessionMgr,
		jwtMgr:      jwtMgr,
		config:      config,
		redisClient: redisClient,
		emailSender: emailSender,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	enabled, err := s.RegistrationAllowed(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrSelfSignupDisabled
	}

	email, err := NormalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	displayName, err := SanitizeDisplayName(req.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := s.config.PasswordPolicy.Validate(req.Password, email, displayName); err != nil {
		return nil, err
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, email, hash, displayName)
	if errors.Is(err, ErrUserAlreadyExists) {
		// Silently notify the existing owner and return a generic response
		// so the API cannot be used to enumerate registered emails.
		if s.emailSender != nil {
			if err := s.emailSender.SendDuplicateRegistrationNotice(ctx, email); err != nil {
				slog.ErrorContext(ctx, "failed to send duplicate registration notice", "email", email, "err", err)
			}
		}
		return &TokenResponse{VerificationPending: true}, nil
	} else if err != nil {
		return nil, err
	}

	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.sessionMgr.CreateSession(ctx, user.ID, userAgent, ipAddress, s.config.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Trigger email verification sending if sender is configured
	if s.emailSender != nil {
		s.sendRegisterVerification(ctx, user.ID, user.Email)
	}

	s.auditEvent(ctx, user.ID, AuditRegisterSuccess, ipAddress)

	return &TokenResponse{
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		UserID:              user.ID,
		Email:               user.Email,
		Roles:               roles,
		VerificationPending: true,
	}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	email := normalizeLoginEmail(req.Email)
	if err := s.checkLoginLockout(ctx, email); err != nil {
		return nil, err
	}

	user, passwordOK, method, err := s.authenticateLogin(ctx, req, email)
	if err != nil {
		return nil, err
	}
	if err := s.rejectInvalidLogin(ctx, user, passwordOK, email, method); err != nil {
		return nil, err
	}
	if err := s.validateLoginAccountState(ctx, user); err != nil {
		return nil, err
	}

	s.clearLoginFailures(ctx, email)

	mfaResp, continueLogin, err := s.loginMFAGate(ctx, user)
	if err != nil {
		return nil, err
	}
	if !continueLogin {
		return mfaResp, nil
	}

	return s.issueSession(ctx, user, userAgent, ipAddress, method)
}

func normalizeLoginEmail(raw string) string {
	email, err := NormalizeEmail(raw)
	if err != nil {
		return strings.TrimSpace(strings.ToLower(raw))
	}
	return email
}

func (s *Service) checkLoginLockout(ctx context.Context, email string) error {
	if s.redisClient == nil {
		return nil
	}
	lockKey := "login_failures:" + email
	val, err := s.redisClient.Get(ctx, lockKey).Int()
	if err == nil && val >= 5 {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailAccountLocked).Inc()
		return ErrAccountLocked
	}
	return nil
}

func (s *Service) authenticateLogin(ctx context.Context, req LoginRequest, email string) (*User, bool, string, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		user = nil
		// Burn the same amount of CPU as a real bcrypt verify so the response
		// time does not reveal whether the email exists.
		VerifyDummyPassword(req.Password)
	} else if err != nil {
		return nil, false, "", err
	}

	// Compute local password verification regardless of IsActive / hash presence
	// so inactive or directory/oauth-only accounts spend the same wall-clock time
	// as password-backed accounts.
	passwordOK := user != nil && user.PasswordHash != nil && VerifyPassword(req.Password, *user.PasswordHash)
	if user != nil && user.PasswordHash == nil {
		VerifyDummyPassword(req.Password)
	}

	// Directory (LDAP) fallback — local-first. Only attempted when local auth did
	// not succeed, so a local bootstrap admin always works and a slow/unreachable
	// directory cannot affect successful local logins.
	method := "password"
	if passwordOK {
		return user, true, method, nil
	}

	ldapUser, lerr := s.tryLDAP(ctx, req.Email, req.Password)
	if lerr != nil {
		slog.WarnContext(ctx, "ldap authentication error", "error", lerr)
	}
	if ldapUser != nil {
		return ldapUser, true, "ldap", nil
	}
	return user, false, method, nil
}

func (s *Service) rejectInvalidLogin(ctx context.Context, user *User, passwordOK bool, email, method string) error {
	if user == nil {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailUserNotFound).Inc()
		s.recordLoginFailure(ctx, email, nil)
		return ErrInvalidCredentials
	}
	if !user.IsActive {
		MetricLoginAttempts.WithLabelValues(method, "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailInactive).Inc()
		return ErrInactiveUser
	}
	if !passwordOK {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailBadPassword).Inc()
		s.recordLoginFailure(ctx, email, user)
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) validateLoginAccountState(ctx context.Context, user *User) error {
	state, err := s.userRepo.GetAccountState(ctx, user.ID)
	if err != nil {
		return err
	}
	if state.IsDeleted() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailAccountDeleted).Inc()
		return ErrAccountDeleted
	}
	if state.IsFrozen() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailAccountFrozen).Inc()
		return ErrAccountFrozen
	}
	return nil
}

func (s *Service) clearLoginFailures(ctx context.Context, email string) {
	if s.redisClient == nil {
		return
	}
	lockKey := "login_failures:" + email
	if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
		slog.WarnContext(ctx, "failed to delete login failures key", "key", lockKey, "err", err)
	}
}

func (s *Service) loginMFAGate(ctx context.Context, user *User) (*TokenResponse, bool, error) {
	workspaceMFARequired, err := s.activeWorkspaceRequiresMFA(ctx, user.ID)
	if err != nil {
		return nil, false, err
	}

	if s.mfaSvc != nil {
		enabled, err := s.mfaSvc.IsEnabled(ctx, user.ID)
		if err != nil {
			return nil, false, fmt.Errorf("check mfa: %w", err)
		}
		if enabled {
			challenge, err := s.jwtMgr.GenerateMFAChallenge(user.ID)
			if err != nil {
				return nil, false, fmt.Errorf("generate mfa challenge: %w", err)
			}
			MetricLoginAttempts.WithLabelValues("password", "mfa_required").Inc()
			return &TokenResponse{MFARequired: true, MFAToken: challenge}, false, nil
		}
	}
	if workspaceMFARequired {
		MetricLoginAttempts.WithLabelValues("password", "mfa_required").Inc()
		return nil, false, ErrMFARequired
	}
	return nil, true, nil
}

func (s *Service) activeWorkspaceRequiresMFA(ctx context.Context, userID string) (bool, error) {
	if s.workspaceSvc == nil {
		return false, nil
	}
	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get active workspace: %w", err)
	}
	required, err := s.workspaceSvc.IsMFARequired(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("check workspace mfa policy: %w", err)
	}
	return required, nil
}

func (s *Service) issueSession(ctx context.Context, user *User, userAgent, ipAddress *string, method string) (*TokenResponse, error) {
	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	ua := ""
	if userAgent != nil {
		ua = *userAgent
	}
	ip := ""
	if ipAddress != nil {
		ip = *ipAddress
	}
	fingerprint := DeviceFingerprint(ua, ip)

	refreshToken, err := s.sessionMgr.CreateSessionWithFingerprint(ctx, user.ID, userAgent, ipAddress, fingerprint, s.config.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		slog.ErrorContext(ctx, "failed to update last login", "userID", user.ID, "err", err)
	}
	MetricLoginAttempts.WithLabelValues(method, "success").Inc()
	MetricTokensIssued.WithLabelValues(method).Inc()

	if maxSessions := s.config.MaxActiveSessions; maxSessions > 0 {
		if _, err := s.sessionMgr.EnforceMaxSessions(ctx, user.ID, maxSessions); err != nil {
			slog.ErrorContext(ctx, "failed to enforce max sessions", "userID", user.ID, "err", err)
		}
	}

	if isNew, err := s.userRepo.RecordKnownDevice(ctx, user.ID, fingerprint, userAgent, ipAddress); err == nil && isNew && s.emailSender != nil {
		if err := s.emailSender.SendNewDeviceLogin(ctx, user.Email, mail.DeviceLoginInfo{
			UserAgent:  ua,
			IPAddress:  ip,
			OccurredAt: time.Now(),
		}); err != nil {
			slog.ErrorContext(ctx, "failed to send new device login email", "email", user.Email, "err", err)
		}
	}

	passwordExpired := s.isPasswordExpired(ctx, user.ID)

	s.auditEvent(ctx, user.ID, AuditLoginSuccess, ipAddress)

	return &TokenResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		UserID:          user.ID,
		Email:           user.Email,
		Roles:           roles,
		PasswordExpired: passwordExpired,
	}, nil
}

// isPasswordExpired reports whether the user's password is older than the
// configured BI_AUTH_PASSWORD_MAX_AGE_DAYS (0 = disabled). Errors during the
// lookup are treated as "not expired" so they cannot lock the user out.
func (s *Service) isPasswordExpired(ctx context.Context, userID string) bool {
	days := s.config.PasswordMaxAgeDays
	if days <= 0 {
		return false
	}
	state, err := s.userRepo.GetAccountState(ctx, userID)
	if err != nil || state.PasswordChangedAt == nil {
		return false
	}
	return time.Since(*state.PasswordChangedAt) > time.Duration(days)*24*time.Hour
}

// CompleteMFALogin redeems a challenge token + TOTP/recovery code for a full session.
func (s *Service) CompleteMFALogin(ctx context.Context, req MFALoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	if s.mfaSvc == nil {
		return nil, ErrMFANotEnabled
	}
	userID, err := s.jwtMgr.ValidateMFAChallenge(req.MFAToken)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := s.mfaSvc.VerifyCode(ctx, userID, req.Code); err != nil {
		MetricLoginAttempts.WithLabelValues("mfa", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailMFAInvalid).Inc()
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}
	return s.issueSession(ctx, user, userAgent, ipAddress, "mfa")
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	// Rotate session
	newRefreshToken, err := s.sessionMgr.RotateSession(ctx, req.RefreshToken, s.config.JWTRefreshTTL, userAgent, ipAddress)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	// Fetch user ID from the newly created session
	var userID string
	err = s.sessionMgr.db.QueryRowContext(ctx, "SELECT user_id FROM sessions WHERE refresh_token = $1", s.sessionMgr.HashToken(newRefreshToken)).Scan(&userID)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, fmt.Errorf("query session user: %w", err)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	if !user.IsActive {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, ErrInactiveUser
	}
	if err := s.validateLoginAccountState(ctx, user); err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, user.ID)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	MetricTokenRefreshes.WithLabelValues("success").Inc()

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessionMgr.RevokeSession(ctx, token)
}

func (s *Service) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeWS, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, userID)
	if err != nil {
		activeWS = ""
	}

	return &UserResponse{
		ID:                user.ID,
		Email:             user.Email,
		Username:          user.Username,
		DisplayName:       user.DisplayName,
		AvatarURL:         user.AvatarURL,
		IsActive:          user.IsActive,
		EmailVerified:     user.EmailVerified,
		HasPassword:       user.PasswordHash != nil,
		ActiveWorkspaceID: activeWS,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
	}, nil
}

func (s *Service) sendRegisterVerification(ctx context.Context, userID, email string) {
	token, err := s.generateSecureToken()
	if err != nil {
		return
	}
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.userRepo.CreateEmailVerificationToken(ctx, userID, token, expiresAt); err != nil {
		slog.ErrorContext(ctx, "failed to create email verification token", "userID", userID, "err", err)
	}
	if err := s.emailSender.SendEmailVerification(ctx, email, token); err != nil {
		slog.ErrorContext(ctx, "failed to send email verification", "email", email, "err", err)
	}
}

var (
	localOAuthStateMap         = make(map[string]localOAuthStateEntry)
	localOAuthStateMu          sync.RWMutex
	localOAuthStateJanitorOnce sync.Once
)

const oauthStateTTL = 300 * time.Second

type localOAuthStateEntry struct {
	state     string
	expiresAt time.Time
}

// StoreOAuthState stores the OAuth state in Redis bound to a bindToken and provider.
func (s *Service) StoreOAuthState(ctx context.Context, provider, bindToken, state string) error {
	key := fmt.Sprintf("oauth_state:%s:%s", bindToken, provider)
	if s.redisClient == nil {
		storeLocalOAuthState(key, state, oauthStateTTL)
		return nil
	}
	return s.redisClient.Set(ctx, key, state, oauthStateTTL).Err()
}

// VerifyOAuthState verifies that the OAuth state stored in Redis matches the expected state.
// It also clears the state key.
func (s *Service) VerifyOAuthState(ctx context.Context, provider, bindToken, expectedState string) (bool, error) {
	key := fmt.Sprintf("oauth_state:%s:%s", bindToken, provider)
	if s.redisClient == nil {
		storedState, exists := consumeLocalOAuthState(key, time.Now())
		if !exists {
			return false, nil
		}
		return constantTimeEqualString(storedState, expectedState), nil
	}
	// GETDEL keeps the state single-use: concurrent duplicate callbacks
	// cannot both pass verification the way a separate GET+DEL pair could.
	storedState, err := s.redisClient.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return constantTimeEqualString(storedState, expectedState), nil
}

func constantTimeEqualString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func storeLocalOAuthState(key, state string, ttl time.Duration) {
	localOAuthStateJanitorOnce.Do(startLocalOAuthStateJanitor)
	localOAuthStateMu.Lock()
	localOAuthStateMap[key] = localOAuthStateEntry{
		state:     state,
		expiresAt: time.Now().Add(ttl),
	}
	localOAuthStateMu.Unlock()
}

func consumeLocalOAuthState(key string, now time.Time) (string, bool) {
	localOAuthStateMu.Lock()
	entry, exists := localOAuthStateMap[key]
	if exists {
		delete(localOAuthStateMap, key)
	}
	localOAuthStateMu.Unlock()
	if !exists || !entry.expiresAt.After(now) {
		return "", false
	}
	return entry.state, true
}

func startLocalOAuthStateJanitor() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			purgeExpiredLocalOAuthStates(now)
		}
	}()
}

func purgeExpiredLocalOAuthStates(now time.Time) {
	localOAuthStateMu.Lock()
	for key, entry := range localOAuthStateMap {
		if !entry.expiresAt.After(now) {
			delete(localOAuthStateMap, key)
		}
	}
	localOAuthStateMu.Unlock()
}
