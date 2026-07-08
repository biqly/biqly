package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// personalAccessTokenPrefix marks a bearer credential as a long-lived personal
// access token rather than a session JWT. Duplicated (not imported) in
// internal/http/middleware/jwt.go, since the api service never imports the
// auth package (separately deployed services, see ADR on service boundaries).
const personalAccessTokenPrefix = "bqpat_"

var (
	ErrPersonalAccessTokenNotFound = errors.New("personal access token not found")
	ErrPersonalAccessTokenInvalid  = errors.New("personal access token invalid, expired, or revoked")
)

// PersonalAccessToken is a long-lived, revocable, user-generated credential for
// programmatic access (e.g. the MCP integration), used in place of the
// short-lived session JWT. Only its hash is ever persisted; the plaintext
// value is returned to the caller once, at creation time.
type PersonalAccessToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"-"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

type PersonalAccessTokenManager struct {
	db *sql.DB
}

func NewPersonalAccessTokenManager(db *sql.DB) *PersonalAccessTokenManager {
	return &PersonalAccessTokenManager{db: db}
}

// generateToken returns a new plaintext token and the short prefix stored
// alongside its hash for display purposes (e.g. "bqpat_ab12").
func generateToken() (plaintext, prefix string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	plaintext = personalAccessTokenPrefix + secret
	prefix = plaintext[:len(personalAccessTokenPrefix)+4]
	return plaintext, prefix, nil
}

// CreateToken generates a new token for userID, persists its hash, and
// returns the plaintext value — the only time it is ever available.
func (m *PersonalAccessTokenManager) CreateToken(ctx context.Context, userID, name string, expiresAt *time.Time) (string, PersonalAccessToken, error) {
	plaintext, prefix, err := generateToken()
	if err != nil {
		return "", PersonalAccessToken{}, fmt.Errorf("generate token: %w", err)
	}

	rec := PersonalAccessToken{UserID: userID, Name: name, TokenPrefix: prefix, ExpiresAt: expiresAt}
	err = m.db.QueryRowContext(ctx, `
		INSERT INTO personal_access_tokens (user_id, name, token_hash, token_prefix, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, userID, name, HashToken(plaintext), prefix, expiresAt).Scan(&rec.ID, &rec.CreatedAt)
	if err != nil {
		return "", PersonalAccessToken{}, fmt.Errorf("insert personal access token: %w", err)
	}

	return plaintext, rec, nil
}

// ListActiveTokens returns userID's non-revoked tokens (expired tokens are
// still listed so the user can see and clean them up), newest first.
func (m *PersonalAccessTokenManager) ListActiveTokens(ctx context.Context, userID string) ([]PersonalAccessToken, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, token_prefix, created_at, expires_at, last_used_at
		FROM personal_access_tokens
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []PersonalAccessToken
	for rows.Next() {
		var rec PersonalAccessToken
		var expiresAt, lastUsedAt sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.TokenPrefix, &rec.CreatedAt, &expiresAt, &lastUsedAt); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			rec.ExpiresAt = new(expiresAt.Time)
		}
		if lastUsedAt.Valid {
			rec.LastUsedAt = new(lastUsedAt.Time)
		}
		rec.UserID = userID
		out = append(out, rec)
	}
	return out, rows.Err()
}

// RevokeTokenByID revokes a specific token row, scoped to a user (for the
// user's own token-management UI) — mirrors SessionManager.RevokeSessionByID.
func (m *PersonalAccessTokenManager) RevokeTokenByID(ctx context.Context, userID, tokenID string) error {
	res, err := m.db.ExecContext(ctx,
		`UPDATE personal_access_tokens SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		tokenID, userID,
	)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("personal access token rows affected: %w", err)
	}
	if rows == 0 {
		return ErrPersonalAccessTokenNotFound
	}
	return nil
}

// FindActiveByHash looks up a token by its plaintext value's hash, rejecting
// revoked or expired rows, and best-effort touches last_used_at. A failure to
// record last_used_at does not fail the lookup — it is a courtesy field, not
// a security boundary.
func (m *PersonalAccessTokenManager) FindActiveByHash(ctx context.Context, plaintext string) (*PersonalAccessToken, error) {
	var rec PersonalAccessToken
	var expiresAt, lastUsedAt sql.NullTime
	err := m.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, token_prefix, created_at, expires_at, last_used_at
		FROM personal_access_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
	`, HashToken(plaintext)).Scan(&rec.ID, &rec.UserID, &rec.Name, &rec.TokenPrefix, &rec.CreatedAt, &expiresAt, &lastUsedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPersonalAccessTokenInvalid
	} else if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		rec.ExpiresAt = new(expiresAt.Time)
	}
	if lastUsedAt.Valid {
		rec.LastUsedAt = new(lastUsedAt.Time)
	}

	if _, err := m.db.ExecContext(ctx, `UPDATE personal_access_tokens SET last_used_at = NOW() WHERE id = $1`, rec.ID); err != nil {
		return &rec, nil //nolint:nilerr // last_used_at is a courtesy field; do not fail verification over it
	}
	return &rec, nil
}
