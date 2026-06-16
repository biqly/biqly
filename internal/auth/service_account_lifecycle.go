package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (s *Service) FreezeAccount(ctx context.Context, userID string) error {
	if err := s.userRepo.FreezeAccount(ctx, userID); err != nil {
		return err
	}
	if err := s.sessionMgr.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("revoke user sessions on freeze: %w", err)
	}
	return nil
}

func (s *Service) UnfreezeAccount(ctx context.Context, userID string) error {
	return s.userRepo.UnfreezeAccount(ctx, userID)
}

// DeleteAccount soft-deletes the user, revokes all sessions, schedules purge.
// If password is provided (non-empty), it must match — used for self-service.
func (s *Service) DeleteAccount(ctx context.Context, userID, password string) (time.Time, error) {
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
	if err := s.sessionMgr.RevokeAllUserSessions(ctx, userID); err != nil {
		return time.Time{}, fmt.Errorf("revoke user sessions on delete: %w", err)
	}
	if s.emailSender != nil {
		if err := s.emailSender.SendAccountDeletionScheduled(ctx, user.Email, purgeAt); err != nil {
			slog.ErrorContext(ctx, "failed to send account deletion scheduled email", "email", user.Email, "err", err)
		}
	}
	return purgeAt, nil
}

func (s *Service) RestoreAccount(ctx context.Context, userID string) error {
	return s.userRepo.RestoreAccount(ctx, userID)
}

// UnlockAccount consumes an unlock token and clears the rate-limit counter for
// the associated user, restoring login ability.
func (s *Service) UnlockAccount(ctx context.Context, token string) (string, error) {
	userID, err := s.userRepo.ConsumeUnlockToken(ctx, token)
	if err != nil {
		return "", err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if s.redisClient != nil {
		if err := s.redisClient.Del(ctx, "login_failures:"+user.Email).Err(); err != nil {
			slog.ErrorContext(ctx, "failed to clear login failures on unlock", "email", user.Email, "err", err)
		}
	}
	return userID, nil
}
