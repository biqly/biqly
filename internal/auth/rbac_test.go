package auth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var registerRBACScopeDriver sync.Once

func TestRBACRepositoryGetUserPermissionsOnlyReadsGlobalRoles(t *testing.T) {
	registerRBACScopeDriver.Do(func() {
		sql.Register("rbac_scope_check", rbacScopeDriver{})
	})

	dbPool, err := sql.Open("rbac_scope_check", "")
	require.NoError(t, err)
	defer func() { _ = dbPool.Close() }()

	perms, err := NewRBACRepository(dbPool).GetUserPermissions(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Empty(t, perms)
}

func TestRBACServiceAllowsOnlyMatchingResourceScope(t *testing.T) {
	registerRBACScopeDriver.Do(func() {
		sql.Register("rbac_scope_check", rbacScopeDriver{})
	})

	dbPool, err := sql.Open("rbac_scope_check", "")
	require.NoError(t, err)
	defer func() { _ = dbPool.Close() }()

	rbacSvc := NewRBACService(NewRBACRepository(dbPool))

	allowed, err := rbacSvc.Check(context.Background(), PermissionCheck{
		UserID:     "user-1",
		Permission: "query:execute",
		ScopeType:  ScopeDatasource,
		ScopeID:    "datasource-a",
	})
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = rbacSvc.Check(context.Background(), PermissionCheck{
		UserID:     "user-1",
		Permission: "query:execute",
		ScopeType:  ScopeDatasource,
		ScopeID:    "datasource-b",
	})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestRBACServiceChecksResourceScopedRole(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	const (
		email       = "rbac_resource_scope@example.com"
		datasourceA = "00000000-0000-0000-0000-00000000d5a1"
		datasourceB = "00000000-0000-0000-0000-00000000d5b2"
	)

	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)", email)
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)

	var userID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'RBAC Resource Scope', 'hash', TRUE)
		 RETURNING id`,
		email,
	).Scan(&userID))

	var analystRoleID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`SELECT id FROM roles WHERE name = 'analyst'`,
	).Scan(&analystRoleID))

	rbacRepo := NewRBACRepository(dbPool)
	require.NoError(t, rbacRepo.AssignRole(ctx, userID, analystRoleID, ptrString(string(ScopeDatasource)), ptrString(datasourceA)))

	rbacSvc := NewRBACService(rbacRepo)

	allowed, err := rbacSvc.Check(ctx, PermissionCheck{
		UserID:     userID,
		Permission: "query:execute",
		ScopeType:  ScopeDatasource,
		ScopeID:    datasourceA,
	})
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = rbacSvc.Check(ctx, PermissionCheck{
		UserID:     userID,
		Permission: "query:execute",
		ScopeType:  ScopeDatasource,
		ScopeID:    datasourceB,
	})
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = rbacSvc.Check(ctx, PermissionCheck{
		UserID:     userID,
		Permission: "query:execute",
	})
	require.NoError(t, err)
	assert.False(t, allowed)
}

func ptrString(v string) *string {
	return &v
}

type rbacScopeDriver struct{}

func (rbacScopeDriver) Open(string) (driver.Conn, error) {
	return rbacScopeConn{}, nil
}

type rbacScopeConn struct{}

func (rbacScopeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not supported")
}

func (rbacScopeConn) Close() error {
	return nil
}

func (rbacScopeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

func (rbacScopeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(query), " "))
	if strings.Contains(normalized, "ur.scope_type = $2") && strings.Contains(normalized, "ur.scope_id = $3") {
		if len(args) >= 3 && args[1].Value == string(ScopeDatasource) && args[2].Value == "datasource-a" {
			return &rbacScopeRows{values: []string{"query:execute"}}, nil
		}
		return &rbacScopeRows{}, nil
	}
	if strings.Contains(normalized, "ur.scope_type = 'global'") {
		return &rbacScopeRows{}, nil
	}
	return &rbacScopeRows{values: []string{"query:execute"}}, nil
}

type rbacScopeRows struct {
	values []string
	pos    int
}

func (r *rbacScopeRows) Columns() []string {
	return []string{"name"}
}

func (r *rbacScopeRows) Close() error {
	return nil
}

func (r *rbacScopeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.values) {
		return io.EOF
	}
	dest[0] = r.values[r.pos]
	r.pos++
	return nil
}
