package auth

import (
	"context"
	"database/sql"
	"fmt"
)

type RBACRepository struct {
	db *sql.DB
}

func NewRBACRepository(db *sql.DB) *RBACRepository {
	return &RBACRepository{db: db}
}

func (r *RBACRepository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT DISTINCT r.name
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.id
		WHERE ur.user_id = $1
		  AND ur.scope_type = 'global'
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

func (r *RBACRepository) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.name
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE ur.user_id = $1
		  AND ur.scope_type = 'global'
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return perms, nil
}

func (r *RBACRepository) GetUserScopedPermissions(ctx context.Context, userID string, scopeType ScopeType, scopeID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.name
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE ur.user_id = $1
		  AND ur.scope_type = $2
		  AND ur.scope_id = $3
	`
	rows, err := r.db.QueryContext(ctx, query, userID, string(scopeType), scopeID)
	if err != nil {
		return nil, fmt.Errorf("query scoped permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return perms, nil
}

func (r *RBACRepository) GetUserWorkspacePermissions(ctx context.Context, userID, workspaceID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.name
		FROM workspace_members wm
		JOIN role_permissions rp ON wm.role_id = rp.role_id
		JOIN permissions p ON rp.permission_id = p.id
		WHERE wm.workspace_id = $1 AND wm.user_id = $2
	`
	rows, err := r.db.QueryContext(ctx, query, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("query workspace permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, rows.Err()
}

func (r *RBACRepository) ListAllPermissionNames(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT name FROM permissions ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		perms = append(perms, name)
	}
	return perms, rows.Err()
}

func (r *RBACRepository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, created_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var roles []Role
	for rows.Next() {
		var role Role
		var desc sql.NullString
		if err := rows.Scan(&role.ID, &role.Name, &desc, &role.CreatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			role.Description = &desc.String
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *RBACRepository) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, description, resource, action, created_at FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []Permission
	for rows.Next() {
		var p Permission
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.Resource, &p.Action, &p.CreatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			p.Description = &desc.String
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *RBACRepository) AssignRole(ctx context.Context, userID, roleID string, scopeType *string, scopeID *string) error {
	st := "global"
	sid := "00000000-0000-0000-0000-000000000000"
	if scopeType != nil && *scopeType != "" {
		st = *scopeType
	}
	if scopeID != nil && *scopeID != "" {
		sid = *scopeID
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_roles (user_id, role_id, scope_type, scope_id) VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING`,
		userID, roleID, st, sid)
	return err
}

func (r *RBACRepository) RemoveRole(ctx context.Context, userID, roleID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`,
		userID, roleID)
	return err
}

func (r *RBACRepository) GetUserWorkspaceRole(ctx context.Context, userID, workspaceID string) (string, error) {
	query := `
		SELECT r.name
		FROM workspace_members wm
		JOIN roles r ON wm.role_id = r.id
		WHERE wm.workspace_id = $1 AND wm.user_id = $2
	`
	var roleName string
	err := r.db.QueryRowContext(ctx, query, workspaceID, userID).Scan(&roleName)
	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return roleName, nil
}

func (r *RBACRepository) GetUserRolesWithScope(ctx context.Context, userID string) ([]UserRoleInfo, error) {
	query := `
		SELECT ur.role_id, r.name, ur.scope_type, ur.scope_id
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.id
		WHERE ur.user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user roles with scope: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var roles []UserRoleInfo
	for rows.Next() {
		var role UserRoleInfo
		if err := rows.Scan(&role.RoleID, &role.RoleName, &role.ScopeType, &role.ScopeID); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}
