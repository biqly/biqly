package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createShareTestTables(ctx context.Context, t *testing.T, db *sql.DB) {
	// Create dashboards table (copied from repository_idor_test.go)
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dashboards (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			workspace_id UUID,
			name TEXT NOT NULL,
			description TEXT,
			widgets JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	require.NoError(t, err)

	// Create dashboard_public_shares table (from migration 069a)
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS dashboard_public_shares (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			dashboard_id UUID NOT NULL REFERENCES dashboards(id) ON DELETE CASCADE,
			workspace_id UUID NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			created_by UUID,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			revoked_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ
		)
	`)
	require.NoError(t, err)

	// Create the partial index for at-most-one-active constraint
	_, err = db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_public_shares_active
			ON dashboard_public_shares(dashboard_id) WHERE revoked_at IS NULL
	`)
	require.NoError(t, err)
}

func TestShareRepository_Lifecycle(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	createShareTestTables(ctx, t, db)

	dashRepo := NewRepository(db)
	repo := NewShareRepository(db)
	ws := "11111111-1111-1111-1111-111111111111"
	otherWS := "22222222-2222-2222-2222-222222222222"

	d := &Dashboard{WorkspaceID: new(ws), Name: "shared", Widgets: json.RawMessage(`[]`)}
	require.NoError(t, dashRepo.Create(ctx, d))

	tok, err := GenerateShareToken()
	require.NoError(t, err)
	share := &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok)}
	require.NoError(t, repo.Rotate(ctx, share))
	require.NotEmpty(t, share.ID)

	t.Run("FindActiveByTokenHash finds the live share", func(t *testing.T) {
		got, err := repo.FindActiveByTokenHash(ctx, HashShareToken(tok))
		require.NoError(t, err)
		assert.Equal(t, d.ID, got.DashboardID)
		assert.Equal(t, ws, got.WorkspaceID)
	})

	t.Run("unknown hash is ErrNoRows", func(t *testing.T) {
		_, err := repo.FindActiveByTokenHash(ctx, HashShareToken("nope"))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Rotate revokes the previous share", func(t *testing.T) {
		tok2, err := GenerateShareToken()
		require.NoError(t, err)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok2)}))
		_, err = repo.FindActiveByTokenHash(ctx, HashShareToken(tok))
		assert.ErrorIs(t, err, sql.ErrNoRows, "old token must be dead after rotate")
		got, err := repo.FindActiveByTokenHash(ctx, HashShareToken(tok2))
		require.NoError(t, err)
		assert.Equal(t, d.ID, got.DashboardID)
	})

	t.Run("expired share is not found", func(t *testing.T) {
		tok3, err := GenerateShareToken()
		require.NoError(t, err)
		past := time.Now().Add(-time.Hour)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok3), ExpiresAt: &past}))
		_, err = repo.FindActiveByTokenHash(ctx, HashShareToken(tok3))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("GetActive and Revoke are workspace-scoped (IDOR guard)", func(t *testing.T) {
		tok4, err := GenerateShareToken()
		require.NoError(t, err)
		require.NoError(t, repo.Rotate(ctx, &PublicShare{DashboardID: d.ID, WorkspaceID: ws, TokenHash: HashShareToken(tok4)}))
		_, err = repo.GetActive(ctx, d.ID, otherWS)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.ErrorIs(t, repo.Revoke(ctx, d.ID, otherWS), sql.ErrNoRows)
		got, err := repo.GetActive(ctx, d.ID, ws)
		require.NoError(t, err)
		assert.Nil(t, got.RevokedAt)
		require.NoError(t, repo.Revoke(ctx, d.ID, ws))
		_, err = repo.GetActive(ctx, d.ID, ws)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}
