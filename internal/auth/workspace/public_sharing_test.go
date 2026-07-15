package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/testutil"
)

func TestPublicSharingFlag(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthUserTables(ctx, t, db)

	ownerID := insertWorkspaceTestUser(ctx, t, db, "owner@example.com")
	nonAdminID := insertWorkspaceTestUser(ctx, t, db, "nonadmin@example.com")

	svc := NewWorkspaceService(db, nil)
	ws, err := svc.Create(ctx, "Public Sharing Workspace", "", ownerID)
	require.NoError(t, err)

	enabled, err := svc.IsPublicSharingEnabled(ctx, ws.ID)
	require.NoError(t, err)
	assert.False(t, enabled, "default must be off")

	require.NoError(t, svc.SetPublicSharingEnabled(ctx, ws.ID, ownerID, true))
	enabled, err = svc.IsPublicSharingEnabled(ctx, ws.ID)
	require.NoError(t, err)
	assert.True(t, enabled)

	err = svc.SetPublicSharingEnabled(ctx, ws.ID, nonAdminID, false)
	assert.Error(t, err, "non-admin must not toggle the kill-switch")

	_, err = svc.IsPublicSharingEnabled(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrWorkspaceNotFound)
}
