package mail

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryEmailBlockList(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryEmailBlockListRepo()

	// Unknown address is not blocked.
	blocked, err := repo.IsBlocked(ctx, "Fresh@example.com")
	require.NoError(t, err)
	assert.False(t, blocked)

	// Block + IsBlocked check normalization (case-insensitive).
	require.NoError(t, repo.Block(ctx, "User@Example.COM", "bounced", "admin-1"))
	blocked, err = repo.IsBlocked(ctx, "user@example.com")
	require.NoError(t, err)
	assert.True(t, blocked)

	// Empty reason rejected.
	assert.Error(t, repo.Block(ctx, "x@example.com", "", "admin-1"))

	// Unblock removes the entry.
	require.NoError(t, repo.Unblock(ctx, "USER@example.com"))
	blocked, err = repo.IsBlocked(ctx, "user@example.com")
	require.NoError(t, err)
	assert.False(t, blocked)

	// List returns entries.
	require.NoError(t, repo.Block(ctx, "a@example.com", "spam", ""))
	require.NoError(t, repo.Block(ctx, "b@example.com", "spam", ""))
	all, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}
