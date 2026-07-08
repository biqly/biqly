package auth

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrPersonalAccessTokensUnavailable = errors.New("personal access tokens are not available")

// PATIdentity is the identity resolved from a valid personal access token,
// shaped like the claims a session JWT would carry (see JWTClaims in
// internal/http/middleware/jwt.go) so callers can populate the same request
// context either way.
type PATIdentity struct {
	UserID        string
	Email         string
	EmailVerified bool
	Roles         []string
	WorkspaceID   string
}

// resolveIdentity computes a user's current roles and active workspace — the
// same live lookup issueSession/Register/Refresh perform when minting a JWT.
// Personal access tokens call this on every verification (not once at
// creation) so a revoked role or workspace change takes effect immediately,
// without waiting for a token to expire.
func (s *Service) resolveIdentity(ctx context.Context, userID string) (roles []string, workspaceID string, err error) {
	roles, err = s.rbacRepo.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	workspaceID, err = s.userRepo.GetActiveOrPersonalWorkspaceID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	return roles, workspaceID, nil
}

// CreateAccessToken generates a new personal access token for userID. The
// returned plaintext string is the only time the token value is available.
func (s *Service) CreateAccessToken(ctx context.Context, userID, name string, expiresAt *time.Time) (string, PersonalAccessToken, error) {
	if s.patMgr == nil {
		return "", PersonalAccessToken{}, ErrPersonalAccessTokensUnavailable
	}
	return s.patMgr.CreateToken(ctx, userID, name, expiresAt)
}

// ListAccessTokens returns userID's active personal access tokens (metadata
// only — the plaintext value and its hash are never returned after creation).
func (s *Service) ListAccessTokens(ctx context.Context, userID string) ([]PersonalAccessToken, error) {
	if s.patMgr == nil {
		return nil, ErrPersonalAccessTokensUnavailable
	}
	return s.patMgr.ListActiveTokens(ctx, userID)
}

// RevokeAccessToken revokes one of userID's own personal access tokens.
func (s *Service) RevokeAccessToken(ctx context.Context, userID, tokenID string) error {
	if s.patMgr == nil {
		return ErrPersonalAccessTokensUnavailable
	}
	return s.patMgr.RevokeTokenByID(ctx, userID, tokenID)
}

// VerifyAccessToken resolves a bearer credential to the current identity of
// its owner. Roles and workspace are re-resolved live on every call (see
// resolveIdentity) rather than trusted from a stored snapshot, so a token
// always reflects the owning user's present-day access — never a permission
// set frozen at token-creation time.
func (s *Service) VerifyAccessToken(ctx context.Context, plaintext string) (*PATIdentity, error) {
	if s.patMgr == nil {
		return nil, ErrPersonalAccessTokensUnavailable
	}

	rec, err := s.patMgr.FindActiveByHash(ctx, plaintext)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(ctx, rec.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}

	roles, workspaceID, err := s.resolveIdentity(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("resolve identity for personal access token: %w", err)
	}

	return &PATIdentity{
		UserID:        user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Roles:         roles,
		WorkspaceID:   workspaceID,
	}, nil
}
