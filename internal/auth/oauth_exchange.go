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
	oauthCallbackKeyPrefix = "oauth_callback:"
	oauthCallbackCodeTTL   = 90 * time.Second
)

var (
	ErrInvalidOAuthCallbackCode = errors.New("invalid or expired oauth callback code")
	ErrOAuthExchangeUnavailable = errors.New("oauth callback exchange unavailable")
)

type oauthCallbackPayload struct {
	AccessToken     string   `json:"access_token"`
	RefreshToken    string   `json:"refresh_token"`
	UserID          string   `json:"user_id"`
	Email           string   `json:"email"`
	Roles           []string `json:"roles,omitempty"`
	MFARequired     bool     `json:"mfa_required,omitempty"`
	MFAToken        string   `json:"mfa_token,omitempty"`
	PasswordExpired bool     `json:"password_expired,omitempty"`
}

func oauthCallbackPayloadFromResponse(resp *TokenResponse) oauthCallbackPayload {
	if resp == nil {
		return oauthCallbackPayload{}
	}
	return oauthCallbackPayload{
		AccessToken:     resp.AccessToken,
		RefreshToken:    resp.RefreshToken,
		UserID:          resp.UserID,
		Email:           resp.Email,
		Roles:           resp.Roles,
		MFARequired:     resp.MFARequired,
		MFAToken:        resp.MFAToken,
		PasswordExpired: resp.PasswordExpired,
	}
}

func (p oauthCallbackPayload) toTokenResponse() *TokenResponse {
	return &TokenResponse{
		AccessToken:     p.AccessToken,
		RefreshToken:    p.RefreshToken,
		UserID:          p.UserID,
		Email:           p.Email,
		Roles:           p.Roles,
		MFARequired:     p.MFARequired,
		MFAToken:        p.MFAToken,
		PasswordExpired: p.PasswordExpired,
	}
}

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

	code, err := generateOAuthCallbackCode()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(oauthCallbackPayloadFromResponse(resp))
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
	raw, err := s.redisClient.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidOAuthCallbackCode
		}
		return nil, fmt.Errorf("redeem oauth callback code: %w", err)
	}

	var payload oauthCallbackPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode oauth callback payload: %w", err)
	}
	if payload.AccessToken == "" || payload.RefreshToken == "" {
		return nil, ErrInvalidOAuthCallbackCode
	}
	return payload.toTokenResponse(), nil
}
