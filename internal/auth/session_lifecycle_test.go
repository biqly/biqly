package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/require"
)

func setupRotateSessionUser(t *testing.T, email string) (*sql.DB, context.Context, string) {
	t.Helper()
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email) })
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, '$2a$04$xxx') RETURNING id`,
		email,
	).Scan(&userID))
	return dbPool, ctx, userID
}

//nolint:revive // test helper keeps *testing.T as the first parameter
func rotateSessionWithToken(t *testing.T, ctx context.Context, dbPool *sql.DB, userID, token, sessionSQL string, wantErr error) {
	t.Helper()
	_, err := dbPool.ExecContext(ctx, sessionSQL, userID)
	require.NoError(t, err)
	mgr := NewSessionManager(dbPool)
	mgr.SetLifecycleTTLs(30*24*time.Hour, 4*time.Hour)
	_, err = mgr.RotateSession(ctx, token, time.Hour, nil, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// TestRotateSession_AbsoluteExpiry exercises §18.3 absolute timeout: refresh
// rotation must reject a session whose absolute_expires_at has passed, even if
// the rolling expires_at would still allow it.
func TestRotateSession_AbsoluteExpiry(t *testing.T) {
	dbPool, ctx, userID := setupRotateSessionUser(t, "rotate-absolute@example.invalid")
	hashedToken := HashToken("absolute-test-token")
	rotateSessionWithToken(t, ctx, dbPool, userID, "absolute-test-token", fmt.Sprintf(`
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, '%s', NOW() + INTERVAL '1 hour', NOW() - INTERVAL '1 second', NOW())
	`, hashedToken), ErrSessionAbsoluteExpired)
}

// TestRotateSession_IdleExpiry rejects rotation when last_active_at is older
// than the configured idle TTL.
func TestRotateSession_IdleExpiry(t *testing.T) {
	dbPool, ctx, userID := setupRotateSessionUser(t, "rotate-idle@example.invalid")
	hashedToken := HashToken("idle-test-token")
	rotateSessionWithToken(t, ctx, dbPool, userID, "idle-test-token", fmt.Sprintf(`
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, '%s', NOW() + INTERVAL '1 hour', NOW() + INTERVAL '30 days', NOW() - INTERVAL '5 hours')
	`, hashedToken), ErrSessionIdleExpired)
}

// TestRotateSession_PreservesAbsoluteExpiry ensures the new (rotated) session
// row inherits the original absolute_expires_at — rotation must not extend it.
func TestRotateSession_PreservesAbsoluteExpiry(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-preserve@example.invalid'")
	}()

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = 'rotate-preserve@example.invalid'")
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('rotate-preserve@example.invalid', '$2a$04$xxx') RETURNING id`,
	).Scan(&userID))

	hashedToken := HashToken("preserve-test-token")
	// Original absolute expiry: 10 minutes from now.
	_, err := dbPool.ExecContext(ctx, `
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, $2, NOW() + INTERVAL '5 minutes', NOW() + INTERVAL '10 minutes', NOW())
	`, userID, hashedToken)
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
		`SELECT absolute_expires_at FROM sessions WHERE refresh_token = $1`, hashedToken,
	).Scan(&originalAbsolute))
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT absolute_expires_at, expires_at FROM sessions WHERE refresh_token = $1`, HashToken(newToken),
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
