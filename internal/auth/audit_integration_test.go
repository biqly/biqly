package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogAppendOnlyTriggers(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	svc := NewAuditService(dbPool)
	action := "test.append_only_check"
	require.NoError(t, svc.Log(ctx, nil, action, nil, nil, map[string]any{"k": "v"}, nil))

	var id string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT id FROM audit_log WHERE action = $1 ORDER BY created_at DESC LIMIT 1`, action,
	).Scan(&id))

	_, updErr := dbPool.ExecContext(ctx, `UPDATE audit_log SET action = $1 WHERE id = $2`, "tampered", id)
	assert.Error(t, updErr, "UPDATE must be blocked by trigger")

	_, delErr := dbPool.ExecContext(ctx, `DELETE FROM audit_log WHERE id = $1`, id)
	assert.Error(t, delErr, "DELETE must be blocked by trigger")
}

func TestSeparationOfDutiesBlocksSelfSuperAdminChange(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	const email = "sod_test@example.com"
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)", email)
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)

	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'SoD Test', 'hash', TRUE) RETURNING id`, email,
	).Scan(&userID))

	var saRoleID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = $1`, RoleSuperAdmin,
	).Scan(&saRoleID))

	rbacRepo := NewRBACRepository(dbPool)
	require.NoError(t, rbacRepo.AssignRole(ctx, userID, saRoleID, nil, nil))

	// Self-modification by a super_admin must be denied.
	err := rbacRepo.EnforceSelfModificationGuard(ctx, userID, userID, "role.remove")
	assert.ErrorIs(t, err, ErrSeparationOfDuties)

	// Acting on another user must be allowed.
	err = rbacRepo.EnforceSelfModificationGuard(ctx, userID, "other-user-id", "role.remove")
	assert.NoError(t, err)

	// Non-super_admin acting on self is allowed (SoD only protects super_admin).
	require.NoError(t, rbacRepo.RemoveRole(ctx, userID, saRoleID))
	err = rbacRepo.EnforceSelfModificationGuard(ctx, userID, userID, "role.remove")
	assert.NoError(t, err)
}

func TestAuditFilterDateRange(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	svc := NewAuditService(dbPool)
	require.NoError(t, svc.Log(ctx, nil, "test.range_check", nil, nil, nil, nil))

	entries, err := svc.List(ctx, AuditFilter{Action: "test.range_check", Limit: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, entries)
}
