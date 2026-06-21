package rbac

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforcePrivilegedRoleAssignmentGuard(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	repo := NewRBACRepository(dbPool)

	const email = "privileged_role_guard@example.com"
	testutil.PurgeAuthUsersByEmail(ctx, t, dbPool, email)

	var callerID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Privileged Role Guard', 'hash', TRUE) RETURNING id`, email,
	).Scan(&callerID))

	var (
		superAdminRoleID string
		adminRoleID      string
		developerRoleID  string
	)
	require.NoError(t, dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, RoleSuperAdmin).Scan(&superAdminRoleID))
	require.NoError(t, dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID))
	require.NoError(t, dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = 'developer'`).Scan(&developerRoleID))

	require.NoError(t, repo.AssignRole(ctx, callerID, adminRoleID, nil, nil))

	assert.True(t, roleGrantsAdminPermissions(ctx, t, repo, superAdminRoleID))
	assert.True(t, roleGrantsAdminPermissions(ctx, t, repo, adminRoleID))
	assert.False(t, roleGrantsAdminPermissions(ctx, t, repo, developerRoleID))

	err := repo.EnforcePrivilegedRoleAssignmentGuard(ctx, callerID, superAdminRoleID)
	assert.ErrorIs(t, err, ErrPrivilegedRoleEscalation)

	err = repo.EnforcePrivilegedRoleAssignmentGuard(ctx, callerID, adminRoleID)
	assert.ErrorIs(t, err, ErrPrivilegedRoleEscalation)

	err = repo.EnforcePrivilegedRoleAssignmentGuard(ctx, callerID, developerRoleID)
	assert.NoError(t, err)

	require.NoError(t, repo.AssignRole(ctx, callerID, superAdminRoleID, nil, nil))

	err = repo.EnforcePrivilegedRoleAssignmentGuard(ctx, callerID, adminRoleID)
	assert.NoError(t, err)
}

func roleGrantsAdminPermissions(ctx context.Context, t *testing.T, repo *RBACRepository, roleID string) bool {
	t.Helper()
	privileged, err := repo.RoleGrantsAdminPermissions(ctx, roleID)
	require.NoError(t, err)
	return privileged
}
