package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/require"
)

// createPATTestUser inserts a throwaway user against an already-open pool.
// testutil.OpenAuthDB serializes every caller in this package on a single
// process-wide advisory lock released only via t.Cleanup at the end of the
// *test function* — calling it more than once per test self-deadlocks (the
// second call blocks forever on a lock the first call's cleanup hasn't run
// yet to release). Tests needing multiple users must open the DB once via
// testutil.OpenAuthDB and create every user from that one pool.
func createPATTestUser(t *testing.T, dbPool *sql.DB, email string) string {
	t.Helper()
	ctx := context.Background()
	t.Cleanup(func() { _, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email) })
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)
	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, '$2a$04$xxx') RETURNING id`,
		email,
	).Scan(&userID))
	return userID
}

func setupPATTestUser(t *testing.T, email string) (*PersonalAccessTokenManager, string) {
	t.Helper()
	dbPool := testutil.OpenAuthDB(t)
	userID := createPATTestUser(t, dbPool, email)
	return NewPersonalAccessTokenManager(dbPool), userID
}

func TestPersonalAccessToken_CreateFindRevoke(t *testing.T) {
	mgr, userID := setupPATTestUser(t, "pat-create@example.invalid")
	ctx := context.Background()

	plaintext, rec, err := mgr.CreateToken(ctx, userID, "my mcp token", nil)
	require.NoError(t, err)
	require.NotEmpty(t, plaintext)
	require.Equal(t, "bqpat_", plaintext[:len("bqpat_")])
	require.Equal(t, "my mcp token", rec.Name)
	require.Nil(t, rec.ExpiresAt)

	found, err := mgr.FindActiveByHash(ctx, plaintext)
	require.NoError(t, err)
	require.Equal(t, userID, found.UserID)
	require.Equal(t, rec.ID, found.ID)

	// A different (unknown) plaintext must not resolve.
	_, err = mgr.FindActiveByHash(ctx, "bqpat_not-a-real-token")
	require.ErrorIs(t, err, ErrPersonalAccessTokenInvalid)

	tokens, err := mgr.ListActiveTokens(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, rec.ID, tokens[0].ID)

	require.NoError(t, mgr.RevokeTokenByID(ctx, userID, rec.ID))

	// Revoking again (already revoked) is a not-found, not a silent success.
	err = mgr.RevokeTokenByID(ctx, userID, rec.ID)
	require.ErrorIs(t, err, ErrPersonalAccessTokenNotFound)

	// A revoked token no longer authenticates.
	_, err = mgr.FindActiveByHash(ctx, plaintext)
	require.ErrorIs(t, err, ErrPersonalAccessTokenInvalid)

	tokens, err = mgr.ListActiveTokens(ctx, userID)
	require.NoError(t, err)
	require.Empty(t, tokens)
}

func TestPersonalAccessToken_Expiry(t *testing.T) {
	mgr, userID := setupPATTestUser(t, "pat-expiry@example.invalid")
	ctx := context.Background()

	past := time.Now().Add(-time.Hour)
	plaintext, _, err := mgr.CreateToken(ctx, userID, "expired token", &past)
	require.NoError(t, err)

	_, err = mgr.FindActiveByHash(ctx, plaintext)
	require.ErrorIs(t, err, ErrPersonalAccessTokenInvalid)

	future := time.Now().Add(time.Hour)
	plaintext, rec, err := mgr.CreateToken(ctx, userID, "valid token", &future)
	require.NoError(t, err)

	found, err := mgr.FindActiveByHash(ctx, plaintext)
	require.NoError(t, err)
	require.Equal(t, rec.ID, found.ID)
}

func TestPersonalAccessToken_RevokeScopedToOwner(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	userID := createPATTestUser(t, dbPool, "pat-owner-a@example.invalid")
	otherUserID := createPATTestUser(t, dbPool, "pat-owner-b@example.invalid")
	mgr := NewPersonalAccessTokenManager(dbPool)
	ctx := context.Background()

	_, rec, err := mgr.CreateToken(ctx, userID, "owned by a", nil)
	require.NoError(t, err)

	// User B must not be able to revoke user A's token.
	err = mgr.RevokeTokenByID(ctx, otherUserID, rec.ID)
	require.True(t, errors.Is(err, ErrPersonalAccessTokenNotFound))

	tokens, err := mgr.ListActiveTokens(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
}
