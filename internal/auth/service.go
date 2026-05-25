package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user account is deactivated")
	ErrAccountLocked      = errors.New("too many failed login attempts; account is temporarily locked for 15 minutes")
)

type AuthService struct {
	userRepo    *UserRepository
	rbacRepo    *RBACRepository
	sessionMgr  *SessionManager
	jwtMgr      *JWTManager
	config      *Config
	redisClient *redis.Client
	emailSender EmailSender
}

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
	if err := ValidateEmail(req.Email); err != nil {
		return nil, err
	}
	if err := ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, req.Email, hash, req.DisplayName)
	if err != nil {
		return nil, err
	}

	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	workspaceID, err := s.userRepo.GetPersonalWorkspaceID(ctx, user.ID)
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
	if s.redisClient != nil {
		lockKey := fmt.Sprintf("login_failures:%s", req.Email)
		val, err := s.redisClient.Get(ctx, lockKey).Int()
		if err == nil && val >= 5 {
			MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
			return nil, ErrAccountLocked
		}
	}

	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, ErrUserNotFound) {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		s.recordLoginFailure(ctx, req.Email)
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	if !user.IsActive {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		return nil, ErrInactiveUser
	}

	if user.PasswordHash == nil || !VerifyPassword(req.Password, *user.PasswordHash) {
		MetricLoginAttempts.WithLabelValues("password", "failed").Inc()
		s.recordLoginFailure(ctx, req.Email)
		return nil, ErrInvalidCredentials
	}

	if s.redisClient != nil {
		lockKey := fmt.Sprintf("login_failures:%s", req.Email)
		_ = s.redisClient.Del(ctx, lockKey).Err()
	}

	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	workspaceID, err := s.userRepo.GetPersonalWorkspaceID(ctx, user.ID)
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

	_ = s.userRepo.UpdateLastLogin(ctx, user.ID)

	MetricLoginAttempts.WithLabelValues("password", "success").Inc()

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
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

	workspaceID, err := s.userRepo.GetPersonalWorkspaceID(ctx, user.ID)
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

	return &UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		AvatarURL:     user.AvatarURL,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
	}, nil
}

func (s *AuthService) LoginOrRegisterOAuth(ctx context.Context, provider string, token *oauth2.Token, userInfo *OAuthUserInfo, userAgent, ipAddress *string) (*TokenResponse, error) {
	userID, err := s.userRepo.GetOAuthAccount(ctx, provider, userInfo.Sub)
	var user *User

	if errors.Is(err, ErrOAuthAccountNotFound) {
		user, err = s.userRepo.CreateUserWithOAuth(ctx, userInfo.Email, userInfo.Name, provider, userInfo.Sub, token)
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

	workspaceID, err := s.userRepo.GetPersonalWorkspaceID(ctx, user.ID)
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

	workspaceID, err := s.userRepo.GetPersonalWorkspaceID(ctx, user.ID)
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

func (s *AuthService) recordLoginFailure(ctx context.Context, email string) {
	if s.redisClient != nil {
		lockKey := fmt.Sprintf("login_failures:%s", email)
		_ = s.redisClient.Incr(ctx, lockKey).Err()
		_ = s.redisClient.Expire(ctx, lockKey, 15*time.Minute).Err()
	}
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
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
		return s.emailSender.SendPasswordReset(ctx, email, token)
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}

	userID, err := s.userRepo.VerifyPasswordResetToken(ctx, token)
	if err != nil {
		return err
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
	user, err := s.userRepo.GetUserByEmail(ctx, email)
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
		return s.emailSender.SendEmailVerification(ctx, email, token)
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
