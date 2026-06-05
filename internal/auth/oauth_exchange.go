package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	oauthCallbackKeyPrefix     = "oauth_callback:"
	oauthCallbackUsedKeyPrefix = "oauth_callback_used:"
	oauthCallbackCodeTTL       = 90 * time.Second
	// Allow a short replay window so that retries or simultaneous mounts
	// (e.g. StrictMode, slow networks) get the same tokens back instead of a
	// hard 400 that bounces the user to the sign-in page. The single-use
	// guarantee still holds beyond this grace period.
	oauthCallbackGraceTTL = 5 * time.Second
)

var (
	ErrInvalidOAuthCallbackCode = errors.New("invalid or expired oauth callback code")
	ErrOAuthExchangeUnavailable = errors.New("oauth callback exchange unavailable")
)

func generateOAuthCallbackCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate oauth callback code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *AuthService) IssueOAuthCallbackCode(ctx context.Context, resp *TokenResponse) (string, error) {
	if s.redisClient == nil {
		return "", ErrOAuthExchangeUnavailable
	}
	if resp == nil {
		return "", errors.New("token response required")
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		return "", errors.New("token response missing tokens")
	}

	code, err := generateOAuthCallbackCode()
	if err != nil {
		return "", err
	}

	//nolint:gosec // G117: ephemeral single-use OAuth payload in Redis (90s TTL), not logged.
	payload, err := json.Marshal(resp)
	if err != nil {
		return "", err
	}

	key := oauthCallbackKeyPrefix + code
	if err := s.redisClient.Set(ctx, key, payload, oauthCallbackCodeTTL).Err(); err != nil {
		return "", fmt.Errorf("store oauth callback code: %w", err)
	}
	return code, nil
}

func (s *AuthService) RedeemOAuthCallbackCode(ctx context.Context, code string) (*TokenResponse, error) {
	if s.redisClient == nil {
		return nil, ErrOAuthExchangeUnavailable
	}
	if code == "" {
		return nil, ErrInvalidOAuthCallbackCode
	}

	key := oauthCallbackKeyPrefix + code
	usedKey := oauthCallbackUsedKeyPrefix + code

	raw, err := s.redisClient.GetDel(ctx, key).Bytes()
	if err != nil { //nolint:nestif
		if !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("redeem oauth callback code: %w", err)
		}
		// Single-use already consumed; tolerate retries within the grace TTL.
		raw, err = s.redisClient.Get(ctx, usedKey).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return nil, ErrInvalidOAuthCallbackCode
			}
			return nil, fmt.Errorf("read oauth callback grace cache: %w", err)
		}
	} else {
		// Backup the response for the grace window so concurrent or rapid
		// retries from the same browser get identical tokens.
		_ = s.redisClient.Set(ctx, usedKey, raw, oauthCallbackGraceTTL).Err()
	}

	var resp TokenResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode oauth callback payload: %w", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		return nil, ErrInvalidOAuthCallbackCode
	}
	return &resp, nil
}
