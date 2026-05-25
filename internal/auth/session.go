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

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionRevoked  = errors.New("session revoked")
)

type SessionManager struct {
	db *sql.DB
}

func NewSessionManager(db *sql.DB) *SessionManager {
	return &SessionManager{db: db}
}

func (m *SessionManager) generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *SessionManager) CreateSession(ctx context.Context, userID string, userAgent, ipAddress *string, ttl time.Duration) (string, error) {
	token, err := m.generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	expiresAt := time.Now().Add(ttl)

	query := `
		INSERT INTO sessions (user_id, refresh_token, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = m.db.ExecContext(ctx, query, userID, token, userAgent, ipAddress, expiresAt)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return token, nil
}

func (m *SessionManager) RotateSession(ctx context.Context, oldToken string, ttl time.Duration, userAgent, ipAddress *string) (string, error) {
	var session Session
	var userAgentNull, ipAddressNull sql.NullString
	var revokedAtNull sql.NullTime

	queryGet := `
		SELECT id, user_id, refresh_token, user_agent, ip_address, created_at, expires_at, revoked_at
		FROM sessions
		WHERE refresh_token = $1
	`
	err := m.db.QueryRowContext(ctx, queryGet, oldToken).Scan(
		&session.ID, &session.UserID, &session.RefreshToken, &userAgentNull, &ipAddressNull,
		&session.CreatedAt, &session.ExpiresAt, &revokedAtNull,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSessionNotFound
	} else if err != nil {
		return "", err
	}

	if userAgentNull.Valid {
		session.UserAgent = &userAgentNull.String
	}
	if ipAddressNull.Valid {
		session.IPAddress = &ipAddressNull.String
	}
	if revokedAtNull.Valid {
		session.RevokedAt = &revokedAtNull.Time
	}

	// Token Family Protection: If the token is already revoked, it means it's a reuse!
	// Revoke all sessions for this user to mitigate session hijacking.
	if session.RevokedAt != nil {
		_ = m.RevokeAllUserSessions(ctx, session.UserID)
		return "", ErrSessionRevoked
	}

	if time.Now().After(session.ExpiresAt) {
		return "", ErrSessionExpired
	}

	// Revoke old token
	queryRevoke := `UPDATE sessions SET revoked_at = NOW() WHERE id = $1`
	_, err = m.db.ExecContext(ctx, queryRevoke, session.ID)
	if err != nil {
		return "", fmt.Errorf("revoke old session: %w", err)
	}

	// Create new token/session
	newToken, err := m.CreateSession(ctx, session.UserID, userAgent, ipAddress, ttl)
	if err != nil {
		return "", fmt.Errorf("create rotated session: %w", err)
	}

	return newToken, nil
}

func (m *SessionManager) RevokeSession(ctx context.Context, token string) error {
	query := `UPDATE sessions SET revoked_at = NOW() WHERE refresh_token = $1 AND revoked_at IS NULL`
	_, err := m.db.ExecContext(ctx, query, token)
	return err
}

func (m *SessionManager) RevokeAllUserSessions(ctx context.Context, userID string) error {
	query := `UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := m.db.ExecContext(ctx, query, userID)
	return err
}
