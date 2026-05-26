package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRotateSession_AbsoluteExpiry exercises §18.3 absolute timeout: refresh
// rotation must reject a session whose absolute_expires_at has passed, even if
// the rolling expires_at would still allow it.
func TestRotateSession_AbsoluteExpiry(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()
	defer func() { _, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-absolute@example.invalid'") }()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-absolute@example.invalid'")
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('rotate-absolute@example.invalid', '$2a$04$xxx') RETURNING id`,
	).Scan(&userID))

	// Insert an active session whose absolute window has already elapsed.
	_, err := dbPool.ExecContext(ctx, `
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, 'absolute-test-token', NOW() + INTERVAL '1 hour', NOW() - INTERVAL '1 second', NOW())
	`, userID)
	require.NoError(t, err)

	mgr := NewSessionManager(dbPool)
	mgr.SetLifecycleTTLs(30*24*time.Hour, 4*time.Hour)
	_, err = mgr.RotateSession(ctx, "absolute-test-token", time.Hour, nil, nil)
	if !errors.Is(err, ErrSessionAbsoluteExpired) {
		t.Fatalf("expected ErrSessionAbsoluteExpired, got %v", err)
	}
}

// TestRotateSession_IdleExpiry rejects rotation when last_active_at is older
// than the configured idle TTL.
func TestRotateSession_IdleExpiry(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()
	defer func() { _, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-idle@example.invalid'") }()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-idle@example.invalid'")
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('rotate-idle@example.invalid', '$2a$04$xxx') RETURNING id`,
	).Scan(&userID))

	_, err := dbPool.ExecContext(ctx, `
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, 'idle-test-token', NOW() + INTERVAL '1 hour', NOW() + INTERVAL '30 days', NOW() - INTERVAL '5 hours')
	`, userID)
	require.NoError(t, err)

	mgr := NewSessionManager(dbPool)
	mgr.SetLifecycleTTLs(30*24*time.Hour, 4*time.Hour)
	_, err = mgr.RotateSession(ctx, "idle-test-token", time.Hour, nil, nil)
	if !errors.Is(err, ErrSessionIdleExpired) {
		t.Fatalf("expected ErrSessionIdleExpired, got %v", err)
	}
}

// TestRotateSession_PreservesAbsoluteExpiry ensures the new (rotated) session
// row inherits the original absolute_expires_at — rotation must not extend it.
func TestRotateSession_PreservesAbsoluteExpiry(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()
	defer func() { _, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-preserve@example.invalid'") }()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-preserve@example.invalid'")
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('rotate-preserve@example.invalid', '$2a$04$xxx') RETURNING id`,
	).Scan(&userID))

	// Original absolute expiry: 10 minutes from now.
	_, err := dbPool.ExecContext(ctx, `
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, 'preserve-test-token', NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '10 minutes', NOW())
	`, userID)
	require.NoError(t, err)

	mgr := NewSessionManager(dbPool)
	// Configure a wide absolute TTL — rotation should still copy the original
	// 10-minute absolute deadline, not stamp a new 30-day one.
	mgr.SetLifecycleTTLs(30*24*time.Hour, 4*time.Hour)

	newToken, err := mgr.RotateSession(ctx, "preserve-test-token", time.Hour, nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, newToken)

	var originalAbsolute, rotatedAbsolute, rotatedExpires time.Time
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT absolute_expires_at FROM sessions WHERE refresh_token = 'preserve-test-token'`,
	).Scan(&originalAbsolute))
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT absolute_expires_at, expires_at FROM sessions WHERE refresh_token = $1`, newToken,
	).Scan(&rotatedAbsolute, &rotatedExpires))

	// Absolute expiry carries over verbatim.
	if !originalAbsolute.Equal(rotatedAbsolute) {
		t.Fatalf("absolute_expires_at mutated on rotation: was %s, now %s", originalAbsolute, rotatedAbsolute)
	}
	// Rolling expires_at is clamped to absolute when the requested TTL exceeds it.
	if rotatedExpires.After(rotatedAbsolute) {
		t.Fatalf("expires_at %s should not exceed absolute %s", rotatedExpires, rotatedAbsolute)
	}
}
