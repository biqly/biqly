package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
)

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

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
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
