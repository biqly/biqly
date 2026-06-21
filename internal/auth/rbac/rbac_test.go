package rbac

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
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
	defer func() {
		if err := dbPool.Close(); err != nil {
			t.Errorf("dbPool.Close() error = %v", err)
		}
	}()

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
	defer func() {
		if err := dbPool.Close(); err != nil {
			t.Errorf("dbPool.Close() error = %v", err)
		}
	}()

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
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	const (
		email       = "rbac_resource_scope@example.com"
		datasourceA = "00000000-0000-0000-0000-00000000d5a1"
		datasourceB = "00000000-0000-0000-0000-00000000d5b2"
	)

	testutil.PurgeAuthUsersByEmail(ctx, t, dbPool, email)

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
	require.NoError(t, rbacRepo.AssignRole(ctx, userID, analystRoleID, new(string(ScopeDatasource)), new(datasourceA)))

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

func TestEnforcePrivilegedRoleAssignmentGuardRequiresSuperAdmin(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	const (
		viewerEmail = "privileged_role_viewer@example.com"
		superEmail  = "privileged_role_super@example.com"
	)
	testutil.PurgeAuthUsersByEmail(ctx, t, dbPool, viewerEmail, superEmail)

	var viewerID, superID string
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Privileged Role Viewer', 'hash', TRUE)
		 RETURNING id`,
		viewerEmail,
	).Scan(&viewerID))
	require.NoError(t, dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'Privileged Role Super', 'hash', TRUE)
		 RETURNING id`,
		superEmail,
	).Scan(&superID))
	t.Cleanup(func() {
		testutil.PurgeAuthUsersByEmail(ctx, t, dbPool, viewerEmail, superEmail)
	})

	var viewerRoleID, superRoleID string
	require.NoError(t, dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, "viewer").Scan(&viewerRoleID))
	require.NoError(t, dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = $1`, RoleSuperAdmin).Scan(&superRoleID))

	rbacRepo := NewRBACRepository(dbPool)
	require.NoError(t, rbacRepo.AssignRole(ctx, viewerID, viewerRoleID, nil, nil))
	require.NoError(t, rbacRepo.AssignRole(ctx, superID, superRoleID, nil, nil))

	assert.ErrorIs(t,
		rbacRepo.EnforcePrivilegedRoleAssignmentGuard(ctx, viewerID, superRoleID),
		ErrPrivilegedRoleEscalation,
	)
	assert.NoError(t, rbacRepo.EnforcePrivilegedRoleAssignmentGuard(ctx, superID, superRoleID))
}

func TestRoleInheritanceMigrationDefinesDefaultHierarchy(t *testing.T) {
	up, err := os.ReadFile("../../../migrations/auth/025a_create_role_inheritance.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(025a_create_role_inheritance.up.sql) error = %v, want nil", err)
	}

	sqlStr := string(up)
	for _, fragment := range []string{
		"role_inheritance",
		"parent_role_id",
		"child_role_id",
		"'super_admin', 'admin'",
		"'admin', 'developer'",
		"'developer', 'analyst'",
		"'analyst', 'viewer'",
		"CHECK (parent_role_id <> child_role_id)",
	} {
		if !strings.Contains(sqlStr, fragment) {
			t.Errorf("role inheritance migration contains %q = false, want true", fragment)
		}
	}
}

func TestRBACServiceInheritsGlobalRolePermissions(t *testing.T) {
	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()

	var tableName sql.NullString
	if err := dbPool.QueryRowContext(ctx, `SELECT to_regclass('public.role_inheritance')::text`).Scan(&tableName); err != nil {
		t.Fatalf("QueryRowContext(to_regclass(role_inheritance)) error = %v, want nil", err)
	}
	if !tableName.Valid {
		t.Skip("skipping role inheritance test; migration 025 is not applied")
	}

	const email = "rbac_role_inheritance@example.com"
	testutil.PurgeAuthUsersByEmail(ctx, t, dbPool, email)

	var userID string
	if err := dbPool.QueryRowContext(ctx,
		`INSERT INTO users (email, display_name, password_hash, email_verified)
		 VALUES ($1, 'RBAC Role Inheritance', 'hash', TRUE)
		 RETURNING id`,
		email,
	).Scan(&userID); err != nil {
		t.Fatalf("QueryRowContext(insert user %q) error = %v, want nil", email, err)
	}

	var adminRoleID string
	if err := dbPool.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatalf("QueryRowContext(select admin role) error = %v, want nil", err)
	}

	rbacRepo := NewRBACRepository(dbPool)
	if err := rbacRepo.AssignRole(ctx, userID, adminRoleID, nil, nil); err != nil {
		t.Fatalf("AssignRole(%q, admin) error = %v, want nil", userID, err)
	}

	rbacSvc := NewRBACService(rbacRepo)
	allowed, err := rbacSvc.Check(ctx, PermissionCheck{
		UserID:     userID,
		Permission: "datasource:create",
	})
	if err != nil {
		t.Fatalf("Check(%q, datasource:create) error = %v, want nil", userID, err)
	}
	if !allowed {
		t.Errorf("Check(%q, datasource:create) = false, want true", userID)
	}

	roles, err := rbacRepo.GetUserRoles(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserRoles(%q) error = %v, want nil", userID, err)
	}
	for _, role := range []string{"admin", "developer", "analyst", "viewer"} {
		if !slices.Contains(roles, role) {
			t.Errorf("GetUserRoles(%q) contains %q = false, want true; roles = %v", userID, role, roles)
		}
	}
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

func (*rbacScopeRows) Columns() []string {
	return []string{"name"}
}

func (*rbacScopeRows) Close() error {
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

func TestRBACServiceCacheInvalidation(t *testing.T) {
	registerRBACScopeDriver.Do(func() {
		sql.Register("rbac_scope_check", rbacScopeDriver{})
	})

	dbPool, err := sql.Open("rbac_scope_check", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := dbPool.Close(); err != nil {
			t.Errorf("dbPool.Close() error = %v", err)
		}
	})

	svc := NewRBACService(NewRBACRepository(dbPool))
	// Override TTL so the cache entry lives far enough that expiry doesn't interfere.
	svc.checkTTL = 10 * time.Minute

	// Check — first call goes to DB (mock returns nothing for global), cache stores false.
	allowed, err := svc.Check(context.Background(), PermissionCheck{
		UserID:     "user-1",
		Permission: "query:execute",
	})
	require.NoError(t, err)
	require.False(t, allowed)

	// Second call should hit cache (same result, no DB call).
	svc.checkMu.RLock()
	_, cached := svc.checkCache["user-1:query:execute::"]
	svc.checkMu.RUnlock()
	require.True(t, cached, "expected cache entry after first Check")

	// Invalidate user cache.
	svc.InvalidateUserCache("user-1")

	svc.checkMu.RLock()
	_, cached = svc.checkCache["user-1:query:execute::"]
	svc.checkMu.RUnlock()
	assert.False(t, cached, "expected cache entry to be removed after InvalidateUserCache")

	// Check other user's cache is unaffected.
	_, err = svc.Check(context.Background(), PermissionCheck{
		UserID:     "user-2",
		Permission: "query:execute",
	})
	require.NoError(t, err)

	svc.checkMu.RLock()
	_, cached = svc.checkCache["user-2:query:execute::"]
	svc.checkMu.RUnlock()
	require.True(t, cached, "expected user-2 cache entry to survive")

	// InvalidateAll clears everything.
	svc.InvalidateAllCache()
	svc.checkMu.RLock()
	remaining := len(svc.checkCache)
	svc.checkMu.RUnlock()
	assert.Zero(t, remaining, "expected empty cache after InvalidateAllCache")
}
