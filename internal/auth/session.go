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
	ErrSessionNotFound         = errors.New("session not found")
	ErrSessionExpired          = errors.New("session expired")
	ErrSessionRevoked          = errors.New("session revoked")
	ErrSessionAbsoluteExpired  = errors.New("session absolute lifetime exceeded; please sign in again")
	ErrSessionIdleExpired      = errors.New("session idle for too long; please sign in again")
)

type SessionManager struct {
	db          *sql.DB
	absoluteTTL time.Duration
	idleTTL     time.Duration
}

func NewSessionManager(db *sql.DB) *SessionManager {
	return &SessionManager{db: db}
}

// SetLifecycleTTLs configures the absolute and idle session windows. absolute
// is the wall-clock max age of an authentication, preserved across refresh
// rotations; idle is the maximum gap between last_active_at and now during a
// rotation. A non-positive value disables the corresponding check.
func (m *SessionManager) SetLifecycleTTLs(absolute, idle time.Duration) {
	m.absoluteTTL = absolute
	m.idleTTL = idle
}

func (*SessionManager) generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *SessionManager) CreateSession(ctx context.Context, userID string, userAgent, ipAddress *string, ttl time.Duration) (string, error) {
	return m.CreateSessionWithFingerprint(ctx, userID, userAgent, ipAddress, "", ttl)
}

func (m *SessionManager) CreateSessionWithFingerprint(ctx context.Context, userID string, userAgent, ipAddress *string, fingerprint string, ttl time.Duration) (string, error) {
	now := time.Now()
	absoluteAt := now.Add(m.absoluteTTLOrDefault(ttl))
	return m.createSession(ctx, userID, userAgent, ipAddress, fingerprint, now.Add(ttl), absoluteAt)
}

// absoluteTTLOrDefault returns the configured absolute TTL, falling back to
// the per-rotation refresh-token TTL when nothing was wired (zero-value
// SessionManager from older callers).
func (m *SessionManager) absoluteTTLOrDefault(fallback time.Duration) time.Duration {
	if m.absoluteTTL > 0 {
		return m.absoluteTTL
	}
	return fallback
}

func (m *SessionManager) createSession(ctx context.Context, userID string, userAgent, ipAddress *string, fingerprint string, expiresAt, absoluteExpiresAt time.Time) (string, error) {
	token, err := m.generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}

	var fp *string
	if fingerprint != "" {
		fp = new(fingerprint)
	}

	query := `
		INSERT INTO sessions (user_id, refresh_token, user_agent, ip_address, device_fingerprint, expires_at, absolute_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = m.db.ExecContext(ctx, query, userID, token, userAgent, ipAddress, fp, expiresAt, absoluteExpiresAt)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	return token, nil
}

// EnforceMaxSessions revokes the oldest active sessions (by last_active_at)
// until the active-session count is at most max. Returns the IDs of the
// sessions that were evicted. Returns no-op when max <= 0.
func (m *SessionManager) EnforceMaxSessions(ctx context.Context, userID string, maxSessions int) ([]string, error) {
	if maxSessions <= 0 {
		return nil, nil
	}
	rows, err := m.db.QueryContext(ctx, `
		WITH ordered AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY last_active_at DESC, created_at DESC) AS rn
			FROM sessions
			WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		)
		UPDATE sessions SET revoked_at = NOW()
		WHERE id IN (SELECT id FROM ordered WHERE rn > $2)
		RETURNING id
	`, userID, maxSessions)
	if err != nil {
		return nil, fmt.Errorf("evict sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var evicted []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		evicted = append(evicted, id)
	}
	return evicted, rows.Err()
}

// TouchSession bumps last_active_at on the active session matching the refresh
// token. Best-effort: errors are not fatal.
func (m *SessionManager) TouchSession(ctx context.Context, refreshToken string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE sessions SET last_active_at = NOW() WHERE refresh_token = $1 AND revoked_at IS NULL`,
		refreshToken,
	)
	return err
}

// ListActiveSessions returns active (non-revoked, non-expired) sessions for a user.
func (m *SessionManager) ListActiveSessions(ctx context.Context, userID string) ([]ActiveSessionInfo, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, user_agent, ip_address::text, created_at, last_active_at, expires_at
		FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY last_active_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ActiveSessionInfo
	for rows.Next() {
		var info ActiveSessionInfo
		var ua, ip sql.NullString
		if err := rows.Scan(&info.ID, &ua, &ip, &info.CreatedAt, &info.LastActiveAt, &info.ExpiresAt); err != nil {
			return nil, err
		}
		if ua.Valid {
			info.UserAgent = new(ua.String)
		}
		if ip.Valid {
			info.IPAddress = new(ip.String)
		}
		out = append(out, info)
	}
	return out, rows.Err()
}

// RevokeSessionByID revokes a specific session row, scoped to a user (for the
// user's own session-management UI).
func (m *SessionManager) RevokeSessionByID(ctx context.Context, userID, sessionID string) error {
	res, err := m.db.ExecContext(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		sessionID, userID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (m *SessionManager) RotateSession(ctx context.Context, oldToken string, ttl time.Duration, userAgent, ipAddress *string) (string, error) {
	var sessionID, userID string
	var userAgentNull, ipAddressNull sql.NullString
	var revokedAtNull sql.NullTime
	var createdAt, expiresAt, lastActiveAt, absoluteExpiresAt time.Time

	queryGet := `
		SELECT id, user_id, user_agent, ip_address, created_at, expires_at, last_active_at, absolute_expires_at, revoked_at
		FROM sessions
		WHERE refresh_token = $1
	`
	err := m.db.QueryRowContext(ctx, queryGet, oldToken).Scan(
		&sessionID, &userID, &userAgentNull, &ipAddressNull,
		&createdAt, &expiresAt, &lastActiveAt, &absoluteExpiresAt, &revokedAtNull,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSessionNotFound
	} else if err != nil {
		return "", err
	}

	// Token family protection: refresh of a revoked token signals theft; burn
	// the entire family so the attacker cannot ride along.
	if revokedAtNull.Valid {
		_ = m.RevokeAllUserSessions(ctx, userID)
		return "", ErrSessionRevoked
	}

	now := time.Now()
	if now.After(expiresAt) {
		return "", ErrSessionExpired
	}
	if now.After(absoluteExpiresAt) {
		_ = m.RevokeSession(ctx, oldToken)
		return "", ErrSessionAbsoluteExpired
	}
	if m.idleTTL > 0 && now.Sub(lastActiveAt) > m.idleTTL {
		_ = m.RevokeSession(ctx, oldToken)
		return "", ErrSessionIdleExpired
	}

	// Revoke old token, then mint a new one that carries forward the original
	// absolute_expires_at — refresh rotation must not extend an authentication.
	if _, err := m.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = NOW() WHERE id = $1`, sessionID); err != nil {
		return "", fmt.Errorf("revoke old session: %w", err)
	}

	rotatedExpiresAt := now.Add(ttl)
	if rotatedExpiresAt.After(absoluteExpiresAt) {
		rotatedExpiresAt = absoluteExpiresAt
	}

	newToken, err := m.createSession(ctx, userID, userAgent, ipAddress, "", rotatedExpiresAt, absoluteExpiresAt)
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
