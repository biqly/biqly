package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

func (s *Service) recordLoginFailure(ctx context.Context, email string, user *User) {
	if s.redisClient == nil {
		return
	}
	lockKey := "login_failures:" + email
	count, err := s.redisClient.Incr(ctx, lockKey).Result()
	if err != nil {
		return
	}
	if err := s.redisClient.Expire(ctx, lockKey, 15*time.Minute).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to set expire lock on login failures", "key", lockKey, "err", err)
	}

	// On the threshold transition (5th consecutive failure), send a one-time
	// unlock email so the user can prove ownership and bypass the lockout.
	if count == 5 && user != nil && s.emailSender != nil {
		token, terr := s.userRepo.CreateUnlockToken(ctx, user.ID, time.Hour)
		if terr == nil {
			if err := s.emailSender.SendAccountUnlock(ctx, user.Email, token); err != nil {
				slog.ErrorContext(ctx, "failed to send account unlock email", "email", user.Email, "err", err)
			}
		}
	}
}

// RecordMFAFailure increments the MFA verification fail counter for a user.
// After 5 consecutive failures the user is locked out for 15 minutes.
func (s *Service) RecordMFAFailure(ctx context.Context, userID string) {
	if s.redisClient == nil {
		return
	}
	lockKey := "mfa_failures:" + userID
	_, err := s.redisClient.Incr(ctx, lockKey).Result()
	if err != nil {
		slog.WarnContext(ctx, "failed to record mfa failure", "key", lockKey, "err", err)
		return
	}
	if err := s.redisClient.Expire(ctx, lockKey, 15*time.Minute).Err(); err != nil {
		slog.ErrorContext(ctx, "failed to set expire on mfa failures", "key", lockKey, "err", err)
	}
}

// CheckMFALockout returns ErrMFAVerificationLocked when the user has exceeded
// the MFA verification failure threshold (5 failures in 15 minutes).
func (s *Service) CheckMFALockout(ctx context.Context, userID string) error {
	if s.redisClient == nil {
		return nil
	}
	lockKey := "mfa_failures:" + userID
	val, err := s.redisClient.Get(ctx, lockKey).Int()
	if err == nil && val >= 5 {
		return ErrMFAVerificationLocked
	}
	return nil
}

// ClearMFAFailures deletes the MFA fail counter for a user on successful
// verification.
func (s *Service) ClearMFAFailures(ctx context.Context, userID string) {
	if s.redisClient == nil {
		return
	}
	lockKey := "mfa_failures:" + userID
	if err := s.redisClient.Del(ctx, lockKey).Err(); err != nil {
		slog.WarnContext(ctx, "failed to clear mfa failures", "key", lockKey, "err", err)
	}
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
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

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
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

	if err := s.userRepo.ConsumePasswordResetAndUpdatePassword(ctx, token, hash); err != nil {
		return err
	}
	s.auditEvent(ctx, userID, AuditPasswordReset, nil)
	return nil
}
