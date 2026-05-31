package auth

import (
	"context"
	"fmt"
	"time"
)

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

// PurgeExpiredAccounts is the cron entry point that scrubs PII for accounts
// whose purge_after has elapsed.
func (s *AuthService) PurgeExpiredAccounts(ctx context.Context) ([]string, error) {
	return s.userRepo.PurgeExpiredAccounts(ctx, time.Now())
}
