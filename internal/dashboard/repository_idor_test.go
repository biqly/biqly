package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDashboardRepository_WorkspaceIsolation pins the cross-tenant IDOR guard:
// Get/Update/Delete must be scoped to the caller's workspace. A dashboard
// created in workspace A must not be readable, mutable, or deletable from
// workspace B. Empty workspaceID (super_admin bypass) must reach any row.
func TestDashboardRepository_WorkspaceIsolation(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()

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

	repo := NewRepository(db)

	wsA := "11111111-1111-1111-1111-111111111111"
	wsB := "22222222-2222-2222-2222-222222222222"

	dashA := &Dashboard{
		WorkspaceID: new(wsA),
		Name:        "Workspace A Dashboard",
		Description: new("private to A"),
		Widgets:     json.RawMessage(`[]`),
	}
	require.NoError(t, repo.Create(ctx, dashA))
	require.NotEmpty(t, dashA.ID)

	t.Run("cross-workspace Get is denied", func(t *testing.T) {
		_, err := repo.Get(ctx, dashA.ID, wsB)
		assert.ErrorIs(t, err, sql.ErrNoRows, "workspace B must not read workspace A dashboard")
	})

	t.Run("cross-workspace Update is denied", func(t *testing.T) {
		err := repo.Update(ctx, &Dashboard{
			ID:          dashA.ID,
			Name:        "hijacked",
			Description: new("taken over"),
			Widgets:     json.RawMessage(`[]`),
		}, wsB)
		assert.ErrorIs(t, err, sql.ErrNoRows, "workspace B must not update workspace A dashboard")

		// Confirm A dashboard is unchanged.
		got, err := repo.Get(ctx, dashA.ID, wsA)
		require.NoError(t, err)
		assert.Equal(t, "Workspace A Dashboard", got.Name)
	})

	t.Run("cross-workspace Delete is denied", func(t *testing.T) {
		err := repo.Delete(ctx, dashA.ID, wsB)
		assert.ErrorIs(t, err, sql.ErrNoRows, "workspace B must not delete workspace A dashboard")

		// Confirm A dashboard still exists.
		_, err = repo.Get(ctx, dashA.ID, wsA)
		require.NoError(t, err)
	})

	t.Run("empty workspaceID (super_admin) bypass reaches any row", func(t *testing.T) {
		got, err := repo.Get(ctx, dashA.ID, "")
		require.NoError(t, err)
		assert.Equal(t, dashA.ID, got.ID)
	})

	t.Run("owner workspace can update and delete", func(t *testing.T) {
		err := repo.Update(ctx, &Dashboard{
			ID:          dashA.ID,
			Name:        "renamed by A",
			Description: new("updated"),
			Widgets:     json.RawMessage(`[]`),
		}, wsA)
		require.NoError(t, err)

		got, err := repo.Get(ctx, dashA.ID, wsA)
		require.NoError(t, err)
		assert.Equal(t, "renamed by A", got.Name)

		require.NoError(t, repo.Delete(ctx, dashA.ID, wsA))
		_, err = repo.Get(ctx, dashA.ID, wsA)
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}

// TestDashboardRepository_ListIsolation confirms List only returns dashboards
// for the caller's workspace (plus global NULL-workspace dashboards).
func TestDashboardRepository_ListIsolation(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()

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

	repo := NewRepository(db)

	wsA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	wsB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	for _, d := range []*Dashboard{
		{WorkspaceID: new(wsA), Name: "A1", Widgets: json.RawMessage(`[]`)},
		{WorkspaceID: new(wsA), Name: "A2", Widgets: json.RawMessage(`[]`)},
		{WorkspaceID: new(wsB), Name: "B1", Widgets: json.RawMessage(`[]`)},
		{WorkspaceID: nil, Name: "Global", Widgets: json.RawMessage(`[]`)},
	} {
		require.NoError(t, repo.Create(ctx, d))
	}

	listA, err := repo.List(ctx, wsA)
	require.NoError(t, err)
	namesA := dashboardNames(listA)
	assert.Contains(t, namesA, "A1")
	assert.Contains(t, namesA, "A2")
	assert.Contains(t, namesA, "Global", "NULL-workspace dashboards are visible to all")
	for _, n := range namesA {
		assert.NotEqual(t, "B1", n, "workspace B dashboard must not leak into A list")
	}

	listB, err := repo.List(ctx, wsB)
	require.NoError(t, err)
	namesB := dashboardNames(listB)
	assert.Contains(t, namesB, "B1")
	assert.Contains(t, namesB, "Global")
	for _, n := range namesB {
		assert.NotEqual(t, "A1", n)
		assert.NotEqual(t, "A2", n)
	}
}

func dashboardNames(list []Dashboard) []string {
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Name)
	}
	return out
}
