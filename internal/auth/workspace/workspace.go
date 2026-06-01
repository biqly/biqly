package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/auth/rbac"
)

var (
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrNotWorkspaceOwner = errors.New("not workspace owner")
)

type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	IsPersonal  bool      `json:"is_personal"`
	MFARequired bool      `json:"mfa_required"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceMember struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Email       string    `json:"email,omitempty"`
	DisplayName *string   `json:"display_name,omitempty"`
	RoleID      string    `json:"role_id"`
	RoleName    string    `json:"role_name,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
	InvitedBy   *string   `json:"invited_by,omitempty"`
}

type WorkspaceDatasource struct {
	WorkspaceID    string    `json:"workspace_id"`
	DatasourceID   string    `json:"datasource_id"`
	DatasourceName string    `json:"datasource_name,omitempty"`
	AccessLevel    string    `json:"access_level"`
	AttachedAt     time.Time `json:"attached_at"`
}

type WorkspaceService struct {
	db    *sql.DB
	dsAcc *rbac.DatasourceAccessService
}

func NewWorkspaceService(db *sql.DB, dsAcc *rbac.DatasourceAccessService) *WorkspaceService {
	return &WorkspaceService{db: db, dsAcc: dsAcc}
}

func (s *WorkspaceService) Create(ctx context.Context, name, description, createdBy string) (*Workspace, error) {
	slug := slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("invalid workspace name")
	}

	var ws Workspace
	var desc sql.NullString
	if description != "" {
		desc = sql.NullString{String: description, Valid: true}
	}

	err := s.db.QueryRowContext(ctx, `
		INSERT INTO workspaces (name, slug, description, is_personal, created_by)
		VALUES ($1, $2, $3, FALSE, $4)
		RETURNING id, name, slug, description, is_personal, mfa_required, created_by, created_at, updated_at
	`, name, slug, desc, createdBy).Scan(
		&ws.ID, &ws.Name, &ws.Slug, &desc, &ws.IsPersonal, &ws.MFARequired,
		&ws.CreatedBy, &ws.CreatedAt, &ws.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert workspace: %w", err)
	}
	if desc.Valid {
		ws.Description = &desc.String
	}

	var adminRoleID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		return nil, fmt.Errorf("get admin role: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`, ws.ID, createdBy, adminRoleID)
	if err != nil {
		return nil, fmt.Errorf("add owner as admin: %w", err)
	}

	return &ws, nil
}

func (s *WorkspaceService) Get(ctx context.Context, id string) (*Workspace, error) {
	var ws Workspace
	var desc sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, slug, description, is_personal, mfa_required, created_by, created_at, updated_at
		FROM workspaces WHERE id = $1
	`, id).Scan(&ws.ID, &ws.Name, &ws.Slug, &desc, &ws.IsPersonal, &ws.MFARequired, &ws.CreatedBy, &ws.CreatedAt, &ws.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		ws.Description = &desc.String
	}
	return &ws, nil
}

func (s *WorkspaceService) ListForUser(ctx context.Context, userID string) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT w.id, w.name, w.slug, w.description, w.is_personal, w.mfa_required, w.created_by, w.created_at, w.updated_at
		FROM workspaces w
		JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE wm.user_id = $1
		ORDER BY w.is_personal DESC, w.name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []Workspace
	for rows.Next() {
		var ws Workspace
		var desc sql.NullString
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Slug, &desc, &ws.IsPersonal, &ws.MFARequired, &ws.CreatedBy, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			ws.Description = &desc.String
		}
		list = append(list, ws)
	}
	return list, rows.Err()
}

func (s *WorkspaceService) Update(ctx context.Context, id, callerID, name, description string, mfaRequired *bool) (*Workspace, error) {
	if err := s.requireOwner(ctx, id, callerID); err != nil {
		return nil, err
	}

	var desc sql.NullString
	if description != "" {
		desc = sql.NullString{String: description, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE workspaces SET name = $1, description = $2, updated_at = NOW()
		WHERE id = $3
	`, name, desc, id)
	if err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	if mfaRequired != nil {
		if err := s.SetMFARequired(ctx, id, callerID, *mfaRequired); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *WorkspaceService) Delete(ctx context.Context, id, callerID string) error {
	ws, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if ws.CreatedBy != callerID {
		return ErrNotWorkspaceOwner
	}
	if ws.IsPersonal {
		return fmt.Errorf("cannot delete personal workspace")
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
	return err
}

func (s *WorkspaceService) ListMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT wm.workspace_id, wm.user_id, u.email, u.display_name, wm.role_id, r.name, wm.joined_at, wm.invited_by
		FROM workspace_members wm
		JOIN roles r ON wm.role_id = r.id
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.joined_at
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []WorkspaceMember
	for rows.Next() {
		var m WorkspaceMember
		var invitedBy sql.NullString
		var displayName sql.NullString
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Email, &displayName, &m.RoleID, &m.RoleName, &m.JoinedAt, &invitedBy); err != nil {
			return nil, err
		}
		if displayName.Valid {
			m.DisplayName = &displayName.String
		}
		if invitedBy.Valid {
			m.InvitedBy = &invitedBy.String
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *WorkspaceService) AddMember(ctx context.Context, workspaceID, userID, roleID, invitedBy string) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, invitedBy); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workspace_members (workspace_id, user_id, role_id, invited_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id
	`, workspaceID, userID, roleID, invitedBy)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	if s.dsAcc != nil {
		_ = s.dsAcc.InvalidateCache(ctx, userID)
	}
	return nil
}

func (s *WorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, userID, roleID, callerID string) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE workspace_members SET role_id = $1 WHERE workspace_id = $2 AND user_id = $3
	`, roleID, workspaceID, userID)
	return err
}

func (s *WorkspaceService) RemoveMember(ctx context.Context, workspaceID, userID, callerID string) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}

	ws, err := s.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.CreatedBy == userID {
		return fmt.Errorf("cannot remove workspace owner")
	}

	_, err = s.db.ExecContext(ctx, `
		DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2
	`, workspaceID, userID)
	if err == nil && s.dsAcc != nil {
		_ = s.dsAcc.InvalidateCache(ctx, userID)
	}
	return err
}

func (s *WorkspaceService) IsMember(ctx context.Context, workspaceID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)`,
		workspaceID, userID).Scan(&exists)
	return exists, err
}

func (s *WorkspaceService) IsMFARequired(ctx context.Context, workspaceID string) (bool, error) {
	var required bool
	err := s.db.QueryRowContext(ctx,
		`SELECT mfa_required FROM workspaces WHERE id = $1`,
		workspaceID).Scan(&required)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrWorkspaceNotFound
	}
	return required, err
}

func (s *WorkspaceService) SetMFARequired(ctx context.Context, workspaceID, callerID string, required bool) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE workspaces SET mfa_required = $1, updated_at = NOW()
		WHERE id = $2
	`, required, workspaceID)
	return err
}

func (s *WorkspaceService) ListDatasources(ctx context.Context, workspaceID string) ([]WorkspaceDatasource, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT workspace_id, datasource_id, access_level, attached_at
		FROM workspace_datasources
		WHERE workspace_id = $1
		ORDER BY attached_at
	`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("query workspace datasources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []WorkspaceDatasource
	for rows.Next() {
		var wd WorkspaceDatasource
		if err := rows.Scan(&wd.WorkspaceID, &wd.DatasourceID, &wd.AccessLevel, &wd.AttachedAt); err != nil {
			return nil, err
		}
		list = append(list, wd)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *WorkspaceService) AttachDatasource(ctx context.Context, workspaceID, datasourceID, level, callerID string) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}
	if !rbac.IsValidLevel(level) {
		return fmt.Errorf("invalid access level: %s", level)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO workspace_datasources (workspace_id, datasource_id, access_level, attached_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, datasource_id)
		DO UPDATE SET access_level = EXCLUDED.access_level
	`, workspaceID, datasourceID, level, callerID)
	if err != nil {
		return fmt.Errorf("attach datasource: %w", err)
	}

	if s.dsAcc != nil {
		_ = s.invalidateAllMembers(ctx, workspaceID)
	}
	return nil
}

func (s *WorkspaceService) DetachDatasource(ctx context.Context, workspaceID, datasourceID, callerID string) error {
	if err := s.requireOwnerOrAdmin(ctx, workspaceID, callerID); err != nil {
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM workspace_datasources WHERE workspace_id = $1 AND datasource_id = $2
	`, workspaceID, datasourceID)
	if err == nil && s.dsAcc != nil {
		_ = s.invalidateAllMembers(ctx, workspaceID)
	}
	return err
}

func (s *WorkspaceService) invalidateAllMembers(ctx context.Context, workspaceID string) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM workspace_members WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			continue
		}
		_ = s.dsAcc.InvalidateCache(ctx, uid)
	}
	return rows.Err()
}

func (s *WorkspaceService) requireOwner(ctx context.Context, workspaceID, userID string) error {
	ws, err := s.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.CreatedBy != userID {
		return ErrNotWorkspaceOwner
	}
	return nil
}

func (s *WorkspaceService) requireOwnerOrAdmin(ctx context.Context, workspaceID, userID string) error {
	ws, err := s.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.CreatedBy == userID {
		return nil
	}

	var roleName sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT r.name FROM workspace_members wm
		JOIN roles r ON wm.role_id = r.id
		WHERE wm.workspace_id = $1 AND wm.user_id = $2
	`, workspaceID, userID).Scan(&roleName)
	if err != nil {
		return ErrNotWorkspaceOwner
	}
	if roleName.Valid && (roleName.String == "admin" || roleName.String == "super_admin") {
		return nil
	}
	return ErrNotWorkspaceOwner
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return ""
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
