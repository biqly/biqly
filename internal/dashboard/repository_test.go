package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDBPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test default DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"
	}
	dbPool, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping database tests; DB not available:", err)
	}
	t.Cleanup(func() { _ = dbPool.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := dbPool.PingContext(ctx); err != nil {
		t.Skip("skipping database tests; ping failed:", err)
	}
	return dbPool
}

func TestDashboardRepository_CRUD(t *testing.T) {
	db := openTestDBPool(t)
	ctx := context.Background()

	// Run migration manually or ensure table exists for integration test
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

	workspaceID := "9da3108c-02a8-4eb8-b9a5-1d0b30177729"
	d := &Dashboard{
		WorkspaceID: &workspaceID,
		Name:        "Sales Dashboard",
		Description: ptr("Daily sales KPI and charts"),
		Widgets:     json.RawMessage(`[{"id": "w1", "type": "kpi", "title": "Revenue"}]`),
	}

	// 1. Create
	err = repo.Create(ctx, d)
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID)

	// 2. Get
	fetched, err := repo.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, d.Name, fetched.Name)
	assert.Equal(t, *d.Description, *fetched.Description)
	assert.JSONEq(t, string(d.Widgets), string(fetched.Widgets))
	assert.Equal(t, *d.WorkspaceID, *fetched.WorkspaceID)

	// 3. List
	list, err := repo.List(ctx, workspaceID)
	require.NoError(t, err)
	assert.NotEmpty(t, list)
	found := false
	for _, item := range list {
		if item.ID == d.ID {
			found = true
			break
		}
	}
	assert.True(t, found)

	// 4. Update
	d.Name = "Updated Sales Dashboard"
	d.Widgets = json.RawMessage(`[]`)
	err = repo.Update(ctx, d)
	require.NoError(t, err)

	fetched2, err := repo.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Sales Dashboard", fetched2.Name)
	assert.JSONEq(t, `[]`, string(fetched2.Widgets))

	// 5. Delete
	err = repo.Delete(ctx, d.ID)
	require.NoError(t, err)

	_, err = repo.Get(ctx, d.ID)
	assert.Error(t, err)
}

func ptr(s string) *string {
	return &s
}
