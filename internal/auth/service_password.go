package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (s *AuthService) recordLoginFailure(ctx context.Context, email string, user *User) {
	if s.redisClient == nil {
		return
	}
	lockKey := fmt.Sprintf("login_failures:%s", email)
	count, err := s.redisClient.Incr(ctx, lockKey).Result()
	if err != nil {
		return
	}
	_ = s.redisClient.Expire(ctx, lockKey, 15*time.Minute).Err()

	// On the threshold transition (5th consecutive failure), send a one-time
	// unlock email so the user can prove ownership and bypass the lockout.
	if count == 5 && user != nil && s.emailSender != nil {
		token, terr := s.userRepo.CreateUnlockToken(ctx, user.ID, time.Hour)
		if terr == nil {
			_ = s.emailSender.SendAccountUnlock(ctx, user.Email, token)
		}
	}
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
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

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
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

	err = s.userRepo.UpdateUserPassword(ctx, userID, hash)
	if err != nil {
		return err
	}

	_ = s.userRepo.MarkPasswordResetTokenUsed(ctx, token)
	return nil
}
