package auth

import (
	"context"
	"database/sql"
	"testing"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func assertUserBootstrap(ctx context.Context, t *testing.T, db *sql.DB, userID, wantGlobalRole string) {
	t.Helper()

	var isPersonal bool
	var slug string
	err := db.QueryRowContext(ctx, `
		SELECT w.is_personal, w.slug
		FROM workspaces w
		JOIN workspace_members wm ON wm.workspace_id = w.id
		WHERE wm.user_id = $1 AND w.is_personal = TRUE
	`, userID).Scan(&isPersonal, &slug)
	require.NoError(t, err)
	assert.True(t, isPersonal)
	assert.Equal(t, userID+"-personal", slug)

	var roleName string
	err = db.QueryRowContext(ctx, `
		SELECT r.name
		FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1 AND ur.scope_type = 'global'
	`, userID).Scan(&roleName)
	require.NoError(t, err)
	assert.Equal(t, wantGlobalRole, roleName)

	var memberRoleName string
	err = db.QueryRowContext(ctx, `
		SELECT r.name
		FROM workspace_members wm
		JOIN roles r ON r.id = wm.role_id
		JOIN workspaces w ON w.id = wm.workspace_id
		WHERE wm.user_id = $1 AND w.is_personal = TRUE
	`, userID).Scan(&memberRoleName)
	require.NoError(t, err)
	assert.Equal(t, "admin", memberRoleName)
}

func TestCreateUser_BootstrapCreatesPersonalWorkspaceAndRoles(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthUserTables(ctx, t, db)

	repo := NewUserRepository(db, nil)
	user, err := repo.CreateUser(ctx, "bootstrap-first@example.com", "hash", "Alice")
	require.NoError(t, err)

	assertUserBootstrap(ctx, t, db, user.ID, rbac.RoleSuperAdmin)
}

func TestSecondUserGetsViewerAfterFirstSuperAdmin(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthUserTables(ctx, t, db)

	repo := NewUserRepository(db, nil)

	first, err := repo.CreateUser(ctx, "bootstrap-a@example.com", "hash", "Alice")
	require.NoError(t, err)
	assertUserBootstrap(ctx, t, db, first.ID, rbac.RoleSuperAdmin)

	second, err := repo.CreateUser(ctx, "bootstrap-b@example.com", "hash", "Bob")
	require.NoError(t, err)
	assertUserBootstrap(ctx, t, db, second.ID, "viewer")
}

func TestCreateUserWithOAuth_BootstrapWorkspace(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthUserTables(ctx, t, db)

	repo := NewUserRepository(db, nil)
	token := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh"}
	user, err := repo.CreateUserWithOAuth(ctx, "oauth-bootstrap@example.com", "OAuth User", "google", "google-uid-1", token)
	require.NoError(t, err)

	assertUserBootstrap(ctx, t, db, user.ID, rbac.RoleSuperAdmin)
}
