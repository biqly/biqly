package auth

import (
	"context"
	"errors"
	"strings"
	"time"
)

func (s *AuthService) RequestEmailChange(ctx context.Context, userID, newEmail string) (*EmailChangeRequest, error) {
	normalizedEmail, err := NormalizeEmail(newEmail)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(user.Email, normalizedEmail) {
		return nil, errors.New("new email must be different")
	}
	if _, err := s.userRepo.GetUserByEmail(ctx, normalizedEmail); err == nil {
		return nil, ErrUserAlreadyExists
	} else if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	oldToken, err := s.generateSecureToken()
	if err != nil {
		return nil, err
	}
	newToken, err := s.generateSecureToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	req, err := s.userRepo.CreateEmailChangeRequest(
		ctx,
		user.ID,
		user.Email,
		normalizedEmail,
		oldToken,
		newToken,
		now.Add(EmailChangeWaitPeriod),
		now.Add(EmailChangeTokenTTL),
	)
	if err != nil {
		return nil, err
	}

	if s.emailSender != nil {
		if err := s.emailSender.SendEmailChangeConfirmation(ctx, user.Email, oldToken, false); err != nil {
			return nil, err
		}
		if err := s.emailSender.SendEmailChangeConfirmation(ctx, normalizedEmail, newToken, true); err != nil {
			return nil, err
		}
	}
	return req, nil
}

func (s *AuthService) ConfirmEmailChange(ctx context.Context, token string) (*EmailChangeRequest, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("token is required")
	}
	req, err := s.userRepo.ConfirmEmailChangeToken(ctx, token, time.Now())
	if err != nil {
		return nil, err
	}
	if req.CompletedAt == nil {
		return req, ErrEmailChangePending
	}
	return req, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	_, err := s.userRepo.VerifyEmailToken(ctx, token)
	return err
}

func (s *AuthService) AdminResendUserVerification(ctx context.Context, targetUserID string) error {
	user, err := s.userRepo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return err
	}
	return s.ResendVerificationEmail(ctx, user.Email)
}

func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
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

	if user.EmailVerified {
		return ErrEmailAlreadyVerified
	}

	token, err := s.generateSecureToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	err = s.userRepo.CreateEmailVerificationToken(ctx, user.ID, token, expiresAt)
	if err != nil {
		return err
	}

	if s.emailSender != nil {
		return s.emailSender.SendEmailVerification(ctx, normalizedEmail, token)
	}

	return nil
}
