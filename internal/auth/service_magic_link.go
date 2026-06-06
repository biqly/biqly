package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// RequestMagicLink issues a single-use email-only sign-in link. The response
// is always nil-error and gives no signal about whether the address has an
// account, preventing enumeration. When the address is known, a token is
// persisted (hashed) and emailed; otherwise the call is silently dropped.
// A per-address cooldown (MagicLinkRequestCooldown) is enforced via Redis
// when available.
func (s *Service) RequestMagicLink(ctx context.Context, email, ipAddress string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	if s.magicLinks == nil {
		return nil
	}
	if s.redisClient != nil {
		key := "magic_link_cooldown:" + normalized
		ok, redisErr := s.redisClient.SetNX(ctx, key, "1", MagicLinkRequestCooldown).Result()
		if redisErr == nil && !ok {
			// Cooldown active; behave like a successful no-op to keep timing
			// constant from the caller's perspective.
			return nil
		}
	}
	user, err := s.userRepo.GetUserByEmail(ctx, normalized)
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !user.IsActive {
		return nil
	}
	plain, err := generateMagicLinkToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(MagicLinkTokenTTL)
	if err := s.magicLinks.Issue(ctx, plain, normalized, user.ID, ipAddress, expiresAt); err != nil {
		return err
	}
	if s.emailSender != nil {
		if err := s.emailSender.SendMagicLink(ctx, normalized, plain); err != nil {
			slog.ErrorContext(ctx, "failed to send magic link email", "email", normalized, "err", err)
		}
	}
	return nil
}

// ConsumeMagicLink atomically validates and marks the token used, then
// returns a fresh session. Errors map to ErrMagicLinkInvalid / ErrMagicLinkUsed
// so handlers can return a uniform 400 without leaking which case happened.
func (s *Service) ConsumeMagicLink(ctx context.Context, plain string, userAgent, ipAddress *string) (*TokenResponse, error) {
	if s.magicLinks == nil {
		return nil, ErrMagicLinkInvalid
	}
	if plain == "" {
		return nil, ErrMagicLinkInvalid
	}
	userID, err := s.magicLinks.Consume(ctx, plain)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}
	state, err := s.userRepo.GetAccountState(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if state.IsDeleted() {
		return nil, ErrAccountDeleted
	}
	if state.IsFrozen() {
		return nil, ErrAccountFrozen
	}
	return s.issueSession(ctx, user, userAgent, ipAddress, "magic_link")
}
