package auth

import (
	"context"
	"fmt"
)

func (s *Service) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*UserResponse, error) {
	displayName, err := SanitizeDisplayName(req.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateUserDisplayName(ctx, userID, displayName); err != nil {
		return nil, err
	}
	if req.AvatarURL != nil {
		if err := s.userRepo.UpdateUserAvatarURL(ctx, userID, req.AvatarURL); err != nil {
			return nil, err
		}
	}
	return s.GetMe(ctx, userID)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.PasswordHash == nil {
		return ErrNoPasswordSet
	}
	if !VerifyPassword(currentPassword, *user.PasswordHash) {
		return ErrInvalidCredentials
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
	if err := s.userRepo.UpdateUserPassword(ctx, userID, hash); err != nil {
		return err
	}
	// Revoke every session so a stolen refresh token can't outlive the password
	// change — the primary self-remediation for a compromised account.
	if err := s.sessionMgr.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("revoke user sessions on password change: %w", err)
	}
	s.auditEvent(ctx, userID, AuditPasswordChanged, nil)
	return nil
}
