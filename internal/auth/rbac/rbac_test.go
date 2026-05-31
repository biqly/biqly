package rbac

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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

func TestRoleInheritanceMigrationDefinesDefaultHierarchy(t *testing.T) {
	up, err := os.ReadFile("../../../migrations/auth/025a_create_role_inheritance.up.sql")
	if err != nil {
		t.Fatalf("ReadFile(025a_create_role_inheritance.up.sql) error = %v, want nil", err)
	}

	sql := string(up)
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
		if !strings.Contains(sql, fragment) {
			t.Errorf("role inheritance migration contains %q = false, want true", fragment)
		}
	}
}

func TestRBACServiceInheritsGlobalRolePermissions(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	var tableName sql.NullString
	if err := dbPool.QueryRowContext(ctx, `SELECT to_regclass('public.role_inheritance')::text`).Scan(&tableName); err != nil {
		t.Fatalf("QueryRowContext(to_regclass(role_inheritance)) error = %v, want nil", err)
	}
	if !tableName.Valid {
		t.Skip("skipping role inheritance test; migration 025 is not applied")
	}

	const email = "rbac_role_inheritance@example.com"
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)", email)
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)

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
		if !slicesContainString(roles, role) {
			t.Errorf("GetUserRoles(%q) contains %q = false, want true; roles = %v", userID, role, roles)
		}
	}
}

func ptrString(v string) *string {
	return &v
}

func slicesContainString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func openTestDBPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_AUTH_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test default DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_auth?sslmode=disable"
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
