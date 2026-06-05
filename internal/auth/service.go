package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/auth/ldap"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/mail"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user account is deactivated")
	ErrAccountLocked      = errors.New("too many failed login attempts; account is temporarily locked for 15 minutes")
	ErrMFARequired        = errors.New("mfa required for active workspace")
	ErrEmailChangePending = errors.New("email change confirmation pending")
	ErrPasswordReused     = errors.New("password was recently used")
	ErrNoPasswordSet      = errors.New("password login is not enabled for this account")
	ErrSuperAdminRequired = errors.New("super admin privilege required")
	ErrSelfSignupDisabled = errors.New("self-service registration is disabled")
	ErrMFANotEnabled      = errors.New("mfa not enabled")
	ErrNotWorkspaceOwner  = errors.New("not workspace owner")
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

type AuthService struct {
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
}

func (s *AuthService) UserRepo() *UserRepository {
	return s.userRepo
}

func (s *AuthService) RBACRepo() *rbac.RBACRepository {
	return s.rbacRepo
}

// SetMagicLinkRepository wires the magic-link repository post-construction.
// Optional; if unset the magic-link endpoints reply as if the address has
// no account, preventing enumeration when the feature is disabled.
func (s *AuthService) SetMagicLinkRepository(r *MagicLinkRepository) { s.magicLinks = r }

// SetWorkspaceService wires the workspace service after construction to avoid
// a constructor-arg ripple through tests; required for active-workspace switching.
func (s *AuthService) SetWorkspaceService(ws WorkspaceService) { s.workspaceSvc = ws }

// SetMFAService wires the MFA service after construction. Optional; if unset
// MFA checks are skipped and login proceeds with single factor.
func (s *AuthService) SetMFAService(m MFAService) { s.mfaSvc = m }

func NewAuthService(
	userRepo *UserRepository,
	rbacRepo *rbac.RBACRepository,
	sessionMgr *SessionManager,
	jwtMgr *JWTManager,
	config *Config,
	redisClient *redis.Client,
	emailSender mail.EmailSender,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		rbacRepo:    rbacRepo,
		sessionMgr:  sessionMgr,
		jwtMgr:      jwtMgr,
		config:      config,
		redisClient: redisClient,
		emailSender: emailSender,
	}
}

func (s *AuthService) Register(ctx context.Context, req RegisterRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	enabled, err := s.SelfSignupEnabled(ctx)
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

	return &TokenResponse{
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		UserID:              user.ID,
		Email:               user.Email,
		Roles:               roles,
		VerificationPending: true,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) { //nolint:funlen,gocognit
	email, emailErr := NormalizeEmail(req.Email)
	if emailErr != nil {
		email = strings.TrimSpace(strings.ToLower(req.Email))
	}
	if s.redisClient != nil {
		lockKey := "login_failures:" + email
		val, err := s.redisClient.Get(ctx, lockKey).Int()
		if err == nil && val >= 5 {
			MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
			MetricFailedLogins.WithLabelValues(LoginFailAccountLocked).Inc()
			return nil, ErrAccountLocked
		}
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		user = nil
		// Burn the same amount of CPU as a real bcrypt verify so the response
		// time does not reveal whether the email exists.
		VerifyDummyPassword(req.Password)
	} else if err != nil {
		return nil, err
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
	if !passwordOK {
		ldapUser, lerr := s.tryLDAP(ctx, req.Email, req.Password)
		if lerr != nil {
			slog.WarnContext(ctx, "ldap authentication error", "error", lerr)
		}
		if ldapUser != nil {
			user = ldapUser
			passwordOK = true
			method = "ldap"
		}
	}

	if user == nil {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailUserNotFound).Inc()
		s.recordLoginFailure(ctx, email, nil)
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		MetricLoginAttempts.WithLabelValues(method, "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailInactive).Inc()
		return nil, ErrInactiveUser
	}

	if !passwordOK {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailBadPassword).Inc()
		s.recordLoginFailure(ctx, email, user)
		return nil, ErrInvalidCredentials
	}

	state, err := s.userRepo.GetAccountState(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if state.IsDeleted() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailAccountDeleted).Inc()
		return nil, ErrAccountDeleted
	}
	if state.IsFrozen() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		MetricFailedLogins.WithLabelValues(LoginFailAccountFrozen).Inc()
		return nil, ErrAccountFrozen
	}

	if s.redisClient != nil {
		lockKey := "login_failures:" + email
		if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
			slog.WarnContext(ctx, "failed to delete login failures key", "key", lockKey, "err", err)
		}
	}

	workspaceMFARequired, err := s.activeWorkspaceRequiresMFA(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	if s.mfaSvc != nil {
		enabled, err := s.mfaSvc.IsEnabled(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("check mfa: %w", err)
		}
		if enabled {
			challenge, err := s.jwtMgr.GenerateMFAChallenge(user.ID)
			if err != nil {
				return nil, fmt.Errorf("generate mfa challenge: %w", err)
			}
			MetricLoginAttempts.WithLabelValues("password", "mfa_required").Inc()
			return &TokenResponse{MFARequired: true, MFAToken: challenge}, nil
		}
	}
	if workspaceMFARequired {
		MetricLoginAttempts.WithLabelValues("password", "mfa_required").Inc()
		return nil, ErrMFARequired
	}

	return s.issueSession(ctx, user, userAgent, ipAddress, method)
}

func (s *AuthService) activeWorkspaceRequiresMFA(ctx context.Context, userID string) (bool, error) {
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

func (s *AuthService) issueSession(ctx context.Context, user *User, userAgent, ipAddress *string, method string) (*TokenResponse, error) {
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
func (s *AuthService) isPasswordExpired(ctx context.Context, userID string) bool {
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
func (s *AuthService) CompleteMFALogin(ctx context.Context, req MFALoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
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

func (s *AuthService) Refresh(ctx context.Context, req RefreshRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	// Rotate session
	newRefreshToken, err := s.sessionMgr.RotateSession(ctx, req.RefreshToken, s.config.JWTRefreshTTL, userAgent, ipAddress)
	if err != nil {
		MetricTokenRefreshes.WithLabelValues("failed").Inc()
		return nil, err
	}

	// Fetch user ID from the newly created session
	var userID string
	err = s.sessionMgr.db.QueryRowContext(ctx, "SELECT user_id FROM sessions WHERE refresh_token = $1", newRefreshToken).Scan(&userID)
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

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionMgr.RevokeSession(ctx, token)
}

func (s *AuthService) GetMe(ctx context.Context, userID string) (*UserResponse, error) {
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

func (s *AuthService) sendRegisterVerification(ctx context.Context, userID, email string) {
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
