package auth

import (
	"context"
	"fmt"
)

func (s *Service) CreateTokenResponseForUser(ctx context.Context, user *User, userAgent, ipAddress *string) (*TokenResponse, error) {
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

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

// AdminForceLogout revokes every active session for the target user.
func (s *Service) AdminForceLogout(ctx context.Context, targetUserID string) error {
	return s.sessionMgr.RevokeAllUserSessions(ctx, targetUserID)
}

// ListActiveSessions returns active sessions for the user.
func (s *Service) ListActiveSessions(ctx context.Context, userID string) ([]ActiveSessionInfo, error) {
	return s.sessionMgr.ListActiveSessions(ctx, userID)
}

// RevokeSession revokes a specific session owned by the user.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	return s.sessionMgr.RevokeSessionByID(ctx, userID, sessionID)
}
