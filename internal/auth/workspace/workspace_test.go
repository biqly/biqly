package workspace

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
)

func TestWorkspaceRoleAssignmentRejectsSuperAdmin(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthUserTables(ctx, t, db)

	ownerID := insertWorkspaceTestUser(ctx, t, db, "owner@example.com")
	memberID := insertWorkspaceTestUser(ctx, t, db, "member@example.com")
	otherID := insertWorkspaceTestUser(ctx, t, db, "other@example.com")
	adminRoleID := workspaceRoleID(ctx, t, db, "admin")
	analystRoleID := workspaceRoleID(ctx, t, db, "analyst")
	superAdminRoleID := workspaceRoleID(ctx, t, db, rbac.RoleSuperAdmin)

	svc := NewWorkspaceService(db, nil)
	ws, err := svc.Create(ctx, "Security Workspace", "", ownerID)
	require.NoError(t, err)

	require.ErrorIs(t, svc.AddMember(ctx, ws.ID, memberID, superAdminRoleID, ownerID), rbac.ErrPrivilegedRoleEscalation)
	require.NoError(t, svc.AddMember(ctx, ws.ID, memberID, analystRoleID, ownerID))

	require.ErrorIs(t, svc.UpdateMemberRole(ctx, ws.ID, memberID, superAdminRoleID, ownerID), rbac.ErrPrivilegedRoleEscalation)
	require.NoError(t, svc.UpdateMemberRole(ctx, ws.ID, memberID, adminRoleID, ownerID))
	require.NoError(t, svc.AddMember(ctx, ws.ID, otherID, analystRoleID, memberID))
}

func insertWorkspaceTestUser(ctx context.Context, t testing.TB, db *sql.DB, email string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(ctx, `INSERT INTO users (email, email_verified) VALUES ($1, TRUE) RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

func workspaceRoleID(ctx context.Context, t testing.TB, db *sql.DB, name string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, name).Scan(&id)
	require.NoError(t, err)
	return id
}
