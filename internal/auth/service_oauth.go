package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"golang.org/x/oauth2"
)

func (s *Service) LoginOrRegisterOAuth(ctx context.Context, provider string, token *oauth2.Token, userInfo *OAuthUserInfo, userAgent, ipAddress *string) (*TokenResponse, error) {
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

	if !userInfo.EmailVerified {
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, errors.New("oauth provider email is not verified")
	}

	userID, err := s.userRepo.GetOAuthAccount(ctx, provider, userInfo.Sub)
	var user *User

	switch {
	case errors.Is(err, ErrOAuthAccountNotFound):
		enabled, signupErr := s.SelfSignupEnabled(ctx)
		if signupErr != nil {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, signupErr
		}
		if !enabled {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, ErrSelfSignupDisabled
		}
		user, err = s.userRepo.CreateUserWithOAuth(ctx, email, displayName, provider, userInfo.Sub, token)
		if err != nil {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, err
		}
	case err != nil:
		MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
		return nil, err
	default:
		user, err = s.userRepo.GetUserByID(ctx, userID)
		if err != nil {
			MetricLoginAttempts.WithLabelValues(provider, "failed").Inc()
			return nil, err
		}
		if err := s.userRepo.LinkOAuthAccount(ctx, userID, provider, userInfo.Sub, token); err != nil {
			slog.ErrorContext(ctx, "failed to link oauth account on login", "userID", userID, "provider", provider, "err", err)
		}
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

	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		slog.ErrorContext(ctx, "failed to update last login on oauth login", "userID", user.ID, "err", err)
	}

	s.auditEvent(ctx, user.ID, AuditLoginSuccess, ipAddress)

	MetricLoginAttempts.WithLabelValues(provider, "success").Inc()

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}
