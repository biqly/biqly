package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/testutil"
)

// TestGDPRExportCompleteness seeds a user with OAuth accounts and sessions and
// verifies the export contains every DB-backed section, never leaks the
// password hash or raw refresh tokens, and reports no warnings for the
// sections it claims to cover.
func TestGDPRExportCompleteness(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()

	const email = "gdpr-export@example.invalid"
	cleanup := func() {
		t.Helper()
		statements := []string{
			"DELETE FROM oauth_accounts WHERE user_id IN (SELECT id FROM users WHERE email = $1)",
			"DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email = $1)",
			"DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)",
			"DELETE FROM workspace_members WHERE user_id IN (SELECT id FROM users WHERE email = $1)",
			"DELETE FROM workspaces WHERE created_by IN (SELECT id FROM users WHERE email = $1)",
			"DELETE FROM users WHERE email = $1",
		}
		for _, stmt := range statements {
			_, err := db.ExecContext(ctx, stmt, email)
			require.NoError(t, err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	userRepo := auth.NewUserRepository(db, nil)
	user, err := userRepo.CreateUser(ctx, email, "GdprSecPass1!", "GDPR User")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO oauth_accounts (user_id, provider, provider_uid, scope, token_expires_at)
		VALUES ($1, 'github', 'gdpr-uid-123', 'read:user', NOW() + INTERVAL '1 hour')
	`, user.ID)
	require.NoError(t, err)

	rawToken := "gdpr-refresh-token-secret" //nolint:gosec // test token
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, refresh_token, expires_at, absolute_expires_at, last_active_at)
		VALUES ($1, $2, NOW() + INTERVAL '1 hour', NOW() + INTERVAL '30 days', NOW())
	`, user.ID, auth.HashToken(rawToken))
	require.NoError(t, err)

	exporter := NewGDPRExporter(db, userRepo, nil, nil, nil, nil, nil)
	out, err := exporter.Export(ctx, user.ID)
	require.NoError(t, err)

	// Identity is present, credentials are not.
	assert.Equal(t, user.ID, out.User.ID)
	assert.Equal(t, email, out.User.Email)
	assert.Nil(t, out.User.PasswordHash, "password hash must never be exported")
	assert.WithinDuration(t, time.Now().UTC(), out.GeneratedAt, time.Minute)

	// OAuth accounts: present with provider identity, scope, and expiry.
	require.Len(t, out.OAuthAccounts, 1)
	assert.Equal(t, "github", out.OAuthAccounts[0].Provider)
	assert.Equal(t, "gdpr-uid-123", out.OAuthAccounts[0].ProviderUID)
	require.NotNil(t, out.OAuthAccounts[0].Scope)
	assert.Equal(t, "read:user", *out.OAuthAccounts[0].Scope)
	assert.NotNil(t, out.OAuthAccounts[0].TokenExpiresAt)

	// Sessions: present, but only a masked token hint — neither the raw token
	// nor its full stored hash may appear.
	require.Len(t, out.Sessions, 1)
	hint := out.Sessions[0].TokenHint
	assert.NotEmpty(t, hint)
	assert.NotContains(t, hint, rawToken)
	assert.NotEqual(t, auth.HashToken(rawToken), hint)

	// DB-backed sections succeeded, so no warnings may be reported for them.
	assert.Empty(t, out.Warnings)
}
