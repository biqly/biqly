package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthStateCSRF pins the OAuth state CSRF contract: the state returned by
// the provider must match the one stored for the browser's bind-token cookie,
// and every state is single-use regardless of whether verification succeeded.
func TestOAuthStateCSRF(t *testing.T) {
	// redisClient is nil, so the service uses the in-process state map — the
	// same code path the handler exercises in single-node deployments.
	svc := &Service{}
	ctx := context.Background()

	// 1. Happy path: stored state verifies exactly once.
	require.NoError(t, svc.StoreOAuthState(ctx, "github", "bind-1", "state-1"))
	ok, err := svc.VerifyOAuthState(ctx, "github", "bind-1", "state-1")
	require.NoError(t, err)
	assert.True(t, ok)

	// 2. Replay: the same state must not verify a second time (single-use).
	ok, err = svc.VerifyOAuthState(ctx, "github", "bind-1", "state-1")
	require.NoError(t, err)
	assert.False(t, ok, "state replay must be rejected")

	// 3. CSRF: an attacker-supplied state that does not match the stored one
	// must be rejected, and the attempt must burn the stored state so the
	// genuine value cannot be used afterwards either.
	require.NoError(t, svc.StoreOAuthState(ctx, "github", "bind-2", "state-2"))
	ok, err = svc.VerifyOAuthState(ctx, "github", "bind-2", "forged-state")
	require.NoError(t, err)
	assert.False(t, ok, "mismatched state must be rejected")
	ok, err = svc.VerifyOAuthState(ctx, "github", "bind-2", "state-2")
	require.NoError(t, err)
	assert.False(t, ok, "state must be consumed by the failed attempt")

	// 4. The state is bound to the browser's bind token: the same state under
	// a different bind token (another browser/session) must not verify.
	require.NoError(t, svc.StoreOAuthState(ctx, "github", "bind-3", "state-3"))
	ok, err = svc.VerifyOAuthState(ctx, "github", "other-bind", "state-3")
	require.NoError(t, err)
	assert.False(t, ok, "state must be bound to the issuing bind token")

	// 5. The state is bound to the provider as well.
	ok, err = svc.VerifyOAuthState(ctx, "google", "bind-3", "state-3")
	require.NoError(t, err)
	assert.False(t, ok, "state must be bound to the provider")
}

func TestOAuthStateLocalFallbackExpires(t *testing.T) {
	key := "oauth_state:expired-bind:github"
	storeLocalOAuthState(key, "expired-state", -time.Second)

	stored, ok := consumeLocalOAuthState(key, time.Now())
	assert.False(t, ok, "expired local OAuth state must not verify")
	assert.Empty(t, stored)
}
