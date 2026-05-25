package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("user account is deactivated")
)

type AuthService struct {
	userRepo   *UserRepository
	rbacRepo   *RBACRepository
	sessionMgr *SessionManager
	jwtMgr     *JWTManager
	config     *Config
}

func NewAuthService(userRepo *UserRepository, rbacRepo *RBACRepository, sessionMgr *SessionManager, jwtMgr *JWTManager, config *Config) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		rbacRepo:   rbacRepo,
		sessionMgr: sessionMgr,
		jwtMgr:     jwtMgr,
		config:     config,
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

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress *string) (*TokenResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, ErrUserNotFound) {
		return nil, ErrInvalidCredentials
	} else if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInactiveUser
	}

	if user.PasswordHash == nil || !VerifyPassword(req.Password, *user.PasswordHash) {
		return nil, ErrInvalidCredentials
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
		return nil, err
	}

	// Fetch user ID from the newly created session
	var userID string
	err = s.sessionMgr.db.QueryRowContext(ctx, "SELECT user_id FROM sessions WHERE refresh_token = $1", newRefreshToken).Scan(&userID)
	if err != nil {
		return nil, fmt.Errorf("query session user: %w", err)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrInactiveUser
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
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		user, err = s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		_ = s.userRepo.LinkOAuthAccount(ctx, userID, provider, userInfo.Sub, token)
	}

	if !user.IsActive {
		return nil, ErrInactiveUser
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
