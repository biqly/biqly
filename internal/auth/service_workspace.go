package auth

import (
	"context"
	"errors"
	"fmt"
)

// SetActiveWorkspace switches the user's active workspace and re-issues an
// access token with the new workspace_id claim. The refresh token continues
// to be valid — only the access token needs to be swapped client-side.
//
// Rejects switching to a workspace the user is not a member of (ErrNotWorkspaceOwner
// returned as a generic membership error to avoid leaking workspace existence).
func (s *Service) SetActiveWorkspace(ctx context.Context, userID, workspaceID string) (*SetActiveWorkspaceResponse, error) {
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

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &SetActiveWorkspaceResponse{
		AccessToken:       accessToken,
		ActiveWorkspaceID: workspaceID,
	}, nil
}
