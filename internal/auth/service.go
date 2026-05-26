package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user account is deactivated")
	ErrAccountLocked      = errors.New("too many failed login attempts; account is temporarily locked for 15 minutes")
	ErrMFARequired        = errors.New("mfa required for active workspace")
	ErrEmailChangePending = errors.New("email change confirmation pending")
	ErrPasswordReused     = errors.New("password was recently used")
)

const PasswordHistoryLimit = 5

type AuthService struct {
	userRepo     *UserRepository
	rbacRepo     *RBACRepository
	sessionMgr   *SessionManager
	jwtMgr       *JWTManager
	config       *Config
	redisClient  *redis.Client
	emailSender  EmailSender
	workspaceSvc *WorkspaceService
	mfaSvc       *MFAService
}

// SetWorkspaceService wires the workspace service after construction to avoid
// a constructor-arg ripple through tests; required for active-workspace switching.
func (s *AuthService) SetWorkspaceService(ws *WorkspaceService) { s.workspaceSvc = ws }

// SetMFAService wires the MFA service after construction. Optional; if unset
// MFA checks are skipped and login proceeds with single factor.
func (s *AuthService) SetMFAService(m *MFAService) { s.mfaSvc = m }

func NewAuthService(
	userRepo *UserRepository,
	rbacRepo *RBACRepository,
	sessionMgr *SessionManager,
	jwtMgr *JWTManager,
	config *Config,
	redisClient *redis.Client,
	emailSender EmailSender,
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
	if err != nil {
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

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.sessionMgr.CreateSession(ctx, user.ID, userAgent, ipAddress, s.config.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// Trigger email verification sending if sender is configured
	if s.emailSender != nil {
		token, tokenErr := s.generateSecureToken()
		if tokenErr == nil {
			expiresAt := time.Now().Add(24 * time.Hour)
			_ = s.userRepo.CreateEmailVerificationToken(ctx, user.ID, token, expiresAt)
			_ = s.emailSender.SendEmailVerification(ctx, user.Email, token)
		}
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	email, emailErr := NormalizeEmail(req.Email)
	if emailErr != nil {
		email = strings.TrimSpace(strings.ToLower(req.Email))
	}
	if s.redisClient != nil {
		lockKey := fmt.Sprintf("login_failures:%s", email)
		val, err := s.redisClient.Get(ctx, lockKey).Int()
		if err == nil && val >= 5 {
			MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
			return nil, ErrAccountLocked
		}
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		// Burn the same amount of CPU as a real bcrypt verify so the response
		// time does not reveal whether the email exists.
		VerifyDummyPassword(req.Password)
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		s.recordLoginFailure(ctx, email, nil)
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	// Compute password verification regardless of IsActive / hash presence so
	// inactive or oauth-only accounts spend the same wall-clock time as
	// password-backed accounts.
	passwordOK := user.PasswordHash != nil && VerifyPassword(req.Password, *user.PasswordHash)
	if user.PasswordHash == nil {
		VerifyDummyPassword(req.Password)
	}

	if !user.IsActive {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		return nil, ErrInactiveUser
	}

	if !passwordOK {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		s.recordLoginFailure(ctx, email, user)
		return nil, ErrInvalidCredentials
	}

	state, err := s.userRepo.GetAccountState(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if state.IsDeleted() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		return nil, ErrAccountDeleted
	}
	if state.IsFrozen() {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		return nil, ErrAccountFrozen
	}

	if s.redisClient != nil {
		lockKey := fmt.Sprintf("login_failures:%s", email)
		_ = s.redisClient.Del(ctx, lockKey).Err()
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

	return s.issueSession(ctx, user, userAgent, ipAddress, "password")
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

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
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

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)
	MetricLoginAttempts.WithLabelValues(method, "success").Inc()

	if max := s.config.MaxActiveSessions; max > 0 {
		_, _ = s.sessionMgr.EnforceMaxSessions(ctx, user.ID, max)
	}

	if isNew, err := s.userRepo.RecordKnownDevice(ctx, user.ID, fingerprint, userAgent, ipAddress); err == nil && isNew && s.emailSender != nil {
		_ = s.emailSender.SendNewDeviceLogin(ctx, user.Email, DeviceLoginInfo{
			UserAgent:  ua,
			IPAddress:  ip,
			OccurredAt: time.Now(),
		})
	}

	passwordExpired := s.isPasswordExpired(user.ID)

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
func (s *AuthService) isPasswordExpired(userID string) bool {
	days := s.config.PasswordMaxAgeDays
	if days <= 0 {
		return false
	}
	state, err := s.userRepo.GetAccountState(context.Background(), userID)
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

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
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

	activeWS, _ := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, userID)

	return &UserResponse{
		ID:                user.ID,
		Email:             user.Email,
		Username:          user.Username,
		DisplayName:       user.DisplayName,
		AvatarURL:         user.AvatarURL,
		IsActive:          user.IsActive,
		EmailVerified:     user.EmailVerified,
		ActiveWorkspaceID: activeWS,
		CreatedAt:         user.CreatedAt,
		UpdatedAt:         user.UpdatedAt,
	}, nil
}

// SetActiveWorkspace switches the user's active workspace and re-issues an
// access token with the new workspace_id claim. The refresh token continues
// to be valid — only the access token needs to be swapped client-side.
//
// Rejects switching to a workspace the user is not a member of (ErrNotWorkspaceOwner
// returned as a generic membership error to avoid leaking workspace existence).
func (s *AuthService) SetActiveWorkspace(ctx context.Context, userID, workspaceID string) (*SetActiveWorkspaceResponse, error) {
	if workspaceID == "" {
		return nil, errors.New("workspace_id is required")
	}
	if s.workspaceSvc == nil {
		return nil, errors.New("workspace service not configured")
	}

	isMember, err := s.workspaceSvc.IsMember(ctx, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("check workspace membership: %w", err)
	}
	if !isMember {
		return nil, ErrNotWorkspaceOwner
	}

	if err := s.userRepo.SetActiveWorkspaceID(ctx, userID, workspaceID); err != nil {
		return nil, fmt.Errorf("set active workspace: %w", err)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.rbacRepo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &SetActiveWorkspaceResponse{
		AccessToken:       accessToken,
		ActiveWorkspaceID: workspaceID,
	}, nil
}

func (s *AuthService) RequestEmailChange(ctx context.Context, userID, newEmail string) (*EmailChangeRequest, error) {
	normalizedEmail, err := NormalizeEmail(newEmail)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(user.Email, normalizedEmail) {
		return nil, errors.New("new email must be different")
	}
	if _, err := s.userRepo.GetUserByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrUserAlreadyExists
	} else if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	oldToken, err := s.generateSecureToken()
	if err != nil {
		return nil, err
	}
	newToken, err := s.generateSecureToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	req, err := s.userRepo.CreateEmailChangeRequest(
		ctx,
		user.ID,
		user.Email,
		normalizedEmail,
		oldToken,
		newToken,
		now.Add(EmailChangeWaitPeriod),
		now.Add(EmailChangeTokenTTL),
	)
	if err != nil {
		return nil, err
	}

	if s.emailSender != nil {
		if err := s.emailSender.SendEmailChangeConfirmation(ctx, user.Email, oldToken, false); err != nil {
			return nil, err
		}
		if err := s.emailSender.SendEmailChangeConfirmation(ctx, normalizedEmail, newToken, true); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (s *AuthService) ConfirmEmailChange(ctx context.Context, token string) (*EmailChangeRequest, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("token is required")
	}
	req, err := s.userRepo.ConfirmEmailChangeToken(ctx, token, time.Now())
	if err != nil {
		return nil, err
	}
	if req.CompletedAt == nil {
		return req, ErrEmailChangePending
	}
	return req, nil
}

func (s *AuthService) LoginOrRegisterOAuth(ctx context.Context, provider string, token *oauth2.Token, userInfo *OAuthUserInfo, userAgent, ipAddress *string) (*TokenResponse, error) {
	email, err := NormalizeEmail(userInfo.Email)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	}
	displayName, err := SanitizeDisplayName(userInfo.Name)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	}

	userID, err := s.userRepo.GetOAuthAccount(ctx, provider, userInfo.Sub)
	var user *User

	if errors.Is(err, ErrOAuthAccountNotFound) {
		user, err = s.userRepo.CreateUserWithOAuth(ctx, email, displayName, provider, userInfo.Sub, token)
		if err != nil {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, err
		}
	} else if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	} else {
		user, err = s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, err
		}
		_ = s.userRepo.LinkOAuthAccount(ctx, userID, provider, userInfo.Sub, token)
	}

	if !user.IsActive {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, ErrInactiveUser
	}

	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	}

	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, user.ID)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.sessionMgr.CreateSession(ctx, user.ID, userAgent, ipAddress, s.config.JWTRefreshTTL)
	if err != nil {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, fmt.Errorf("create session: %w", err)
	}

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	MetricLoginAttempts.WithLabelValues(provider, "success").Inc()

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

func (s *AuthService) UnlinkOAuth(ctx context.Context, userID, provider string) error {
	return s.userRepo.UnlinkOAuthAccount(ctx, userID, provider)
}

func (s *AuthService) CreateTokenResponseForUser(ctx context.Context, user *User, userAgent, ipAddress *string) (*TokenResponse, error) {
	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	workspaceID, err := s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateToken(user.ID, user.Email, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.sessionMgr.CreateSession(ctx, user.ID, userAgent, ipAddress, s.config.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

func (s *AuthService) recordLoginFailure(ctx context.Context, email string, user *User) {
	if s.redisClient == nil {
		return
	}
	lockKey := fmt.Sprintf("login_failures:%s", email)
	count, err := s.redisClient.Incr(ctx, lockKey).Result()
	if err != nil {
		return
	}
	_ = s.redisClient.Expire(ctx, lockKey, 15*time.Minute).Err()

	// On the threshold transition (5th consecutive failure), send a one-time
	// unlock email so the user can prove ownership and bypass the lockout.
	if count == 5 && user != nil && s.emailSender != nil {
		token, terr := s.userRepo.CreateUnlockToken(ctx, user.ID, time.Hour)
		if terr == nil {
			_ = s.emailSender.SendAccountUnlock(ctx, user.Email, token)
		}
	}
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetUserByEmail(ctx, normalizedEmail)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	token, err := s.generateSecureToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(1 * time.Hour)
	err = s.userRepo.CreatePasswordResetToken(ctx, user.ID, token, expiresAt)
	if err != nil {
		return err
	}

	if s.emailSender != nil {
		return s.emailSender.SendPasswordReset(ctx, normalizedEmail, token)
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	userID, err := s.userRepo.VerifyPasswordResetToken(ctx, token)
	if err != nil {
		return err
	}

	// Pull the user's identity fields so the policy can reject passwords
	// that embed the email or display name (e.g. "alice.smith2024!").
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	identity := []string{user.Email}
	if user.DisplayName != nil {
		identity = append(identity, *user.DisplayName)
	}
	if user.Username != nil {
		identity = append(identity, *user.Username)
	}
	if err := s.config.PasswordPolicy.Validate(newPassword, identity...); err != nil {
		return err
	}
	reused, err := s.userRepo.PasswordWasUsed(ctx, userID, newPassword, PasswordHistoryLimit)
	if err != nil {
		return err
	}
	if reused {
		return ErrPasswordReused
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}

	err = s.userRepo.UpdateUserPassword(ctx, userID, hash)
	if err != nil {
		return err
	}

	_ = s.userRepo.MarkPasswordResetTokenUsed(ctx, token)
	return nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	_, err := s.userRepo.VerifyEmailToken(ctx, token)
	return err
}

func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	normalizedEmail, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetUserByEmail(ctx, normalizedEmail)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	} else if err != nil {
		return err
	}

	if user.EmailVerified {
		return errors.New("email is already verified")
	}

	token, err := s.generateSecureToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	err = s.userRepo.CreateEmailVerificationToken(ctx, user.ID, token, expiresAt)
	if err != nil {
		return err
	}

	if s.emailSender != nil {
		return s.emailSender.SendEmailVerification(ctx, normalizedEmail, token)
	}

	return nil
}

func (s *AuthService) generateSecureToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (s *AuthService) FreezeAccount(ctx context.Context, userID string) error {
	if err := s.userRepo.FreezeAccount(ctx, userID); err != nil {
		return err
	}
	_ = s.sessionMgr.RevokeAllUserSessions(ctx, userID)
	return nil
}

func (s *AuthService) UnfreezeAccount(ctx context.Context, userID string) error {
	return s.userRepo.UnfreezeAccount(ctx, userID)
}

// DeleteAccount soft-deletes the user, revokes all sessions, schedules purge.
// If password is provided (non-empty), it must match — used for self-service.
func (s *AuthService) DeleteAccount(ctx context.Context, userID, password string) (time.Time, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return time.Time{}, err
	}
	if password != "" {
		if user.PasswordHash == nil || !VerifyPassword(password, *user.PasswordHash) {
			return time.Time{}, ErrInvalidCredentials
		}
	}
	purgeAt, err := s.userRepo.SoftDeleteAccount(ctx, userID, s.config.GDPRPurgeAfterDays)
	if err != nil {
		return time.Time{}, err
	}
	_ = s.sessionMgr.RevokeAllUserSessions(ctx, userID)
	if s.emailSender != nil {
		_ = s.emailSender.SendAccountDeletionScheduled(ctx, user.Email, purgeAt)
	}
	return purgeAt, nil
}

func (s *AuthService) RestoreAccount(ctx context.Context, userID string) error {
	return s.userRepo.RestoreAccount(ctx, userID)
}

// UnlockAccount consumes an unlock token and clears the rate-limit counter for
// the associated user, restoring login ability.
func (s *AuthService) UnlockAccount(ctx context.Context, token string) (string, error) {
	userID, err := s.userRepo.ConsumeUnlockToken(ctx, token)
	if err != nil {
		return "", err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if s.redisClient != nil {
		_ = s.redisClient.Del(ctx, fmt.Sprintf("login_failures:%s", user.Email)).Err()
	}
	return userID, nil
}

// AdminForceLogout revokes every active session for the target user.
func (s *AuthService) AdminForceLogout(ctx context.Context, targetUserID string) error {
	return s.sessionMgr.RevokeAllUserSessions(ctx, targetUserID)
}

// ListActiveSessions returns active sessions for the user.
func (s *AuthService) ListActiveSessions(ctx context.Context, userID string) ([]ActiveSessionInfo, error) {
	return s.sessionMgr.ListActiveSessions(ctx, userID)
}

// RevokeSession revokes a specific session owned by the user.
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	return s.sessionMgr.RevokeSessionByID(ctx, userID, sessionID)
}

// PurgeExpiredAccounts is the cron entry point that scrubs PII for accounts
// whose purge_after has elapsed.
func (s *AuthService) PurgeExpiredAccounts(ctx context.Context) ([]string, error) {
	return s.userRepo.PurgeExpiredAccounts(ctx, time.Now())
}
