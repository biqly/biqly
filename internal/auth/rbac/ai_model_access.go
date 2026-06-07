package rbac

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type AIProviderWorkspaceGrant struct {
	WorkspaceID string    `json:"workspace_id"`
	ProviderID  string    `json:"provider_id"`
	GrantedBy   *string   `json:"granted_by,omitempty"`
	GrantedAt   time.Time `json:"granted_at"`
}

type AIModelWorkspaceGrant struct {
	WorkspaceID string    `json:"workspace_id"`
	ModelID     string    `json:"model_id"`
	GrantedBy   *string   `json:"granted_by,omitempty"`
	GrantedAt   time.Time `json:"granted_at"`
}

type AIProviderRoleGrant struct {
	RoleID     string    `json:"role_id"`
	ProviderID string    `json:"provider_id"`
	GrantedBy  *string   `json:"granted_by,omitempty"`
	GrantedAt  time.Time `json:"granted_at"`
}

type AIModelRoleGrant struct {
	RoleID    string    `json:"role_id"`
	ModelID   string    `json:"model_id"`
	GrantedBy *string   `json:"granted_by,omitempty"`
	GrantedAt time.Time `json:"granted_at"`
}

type UserAIModelPreference struct {
	UserID    string    `json:"user_id"`
	Purpose   string    `json:"purpose"`
	ModelID   string    `json:"model_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserAIAccess is the internal shape used to resolve models for a user.
type UserAIAccess struct {
	Restricted  bool              `json:"restricted"`
	ModelIDs    []string          `json:"model_ids"`
	ProviderIDs []string          `json:"provider_ids"`
	Preferences map[string]string `json:"preferences"`
}

type AIModelAccessService struct {
	db   *sql.DB
	rbac *Service
}

// NewAIModelAccessService wires AI model grant storage.
func NewAIModelAccessService(db *sql.DB, rbac *Service) *AIModelAccessService {
	return &AIModelAccessService{db: db, rbac: rbac}
}

func (s *AIModelAccessService) HasAnyGrants(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT (
			(SELECT count(*) FROM ai_provider_workspace_grants) +
			(SELECT count(*) FROM ai_model_workspace_grants) +
			(SELECT count(*) FROM ai_provider_role_grants) +
			(SELECT count(*) FROM ai_model_role_grants)
		)`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count ai grants: %w", err)
	}
	return n > 0, nil
}

func (s *AIModelAccessService) UserAIAccess(ctx context.Context, userID string) (*UserAIAccess, error) {
	restricted, err := s.HasAnyGrants(ctx)
	if err != nil {
		return nil, err
	}
	out := &UserAIAccess{
		Restricted:  restricted,
		Preferences: map[string]string{},
	}
	if restricted {
		modelIDs, providerIDs, err := s.listGrantedIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
		out.ModelIDs = modelIDs
		out.ProviderIDs = providerIDs
	}
	prefs, err := s.ListUserPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, p := range prefs {
		out.Preferences[p.Purpose] = p.ModelID
	}
	return out, nil
}

func (s *AIModelAccessService) listGrantedIDs(ctx context.Context, userID string) (modelIDs []string, providerIDs []string, err error) {
	const q = `
		SELECT DISTINCT model_id::text FROM (
			SELECT g.model_id FROM ai_model_workspace_grants g
			JOIN workspace_members wm ON wm.workspace_id = g.workspace_id
			WHERE wm.user_id = $1
			UNION
			SELECT g.model_id FROM ai_model_role_grants g
			JOIN workspace_members wm ON wm.role_id = g.role_id
			WHERE wm.user_id = $1
			UNION
			SELECT g.model_id FROM ai_model_role_grants g
			JOIN user_roles ur ON ur.role_id = g.role_id AND ur.user_id = $1
		) models
	`
	modelRows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list granted model ids: %w", err)
	}
	defer func() {
		if closeErr := modelRows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close granted model rows: %w", closeErr))
		}
	}()
	for modelRows.Next() {
		var id string
		if err := modelRows.Scan(&id); err != nil {
			return nil, nil, err
		}
		modelIDs = append(modelIDs, id)
	}
	if err := modelRows.Err(); err != nil {
		return nil, nil, err
	}

	const providerQ = `
		SELECT DISTINCT provider_id::text FROM (
			SELECT g.provider_id FROM ai_provider_workspace_grants g
			JOIN workspace_members wm ON wm.workspace_id = g.workspace_id
			WHERE wm.user_id = $1
			UNION
			SELECT g.provider_id FROM ai_provider_role_grants g
			JOIN workspace_members wm ON wm.role_id = g.role_id
			WHERE wm.user_id = $1
			UNION
			SELECT g.provider_id FROM ai_provider_role_grants g
			JOIN user_roles ur ON ur.role_id = g.role_id AND ur.user_id = $1
		) providers
	`
	provRows, err := s.db.QueryContext(ctx, providerQ, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list granted provider ids: %w", err)
	}
	defer func() {
		if closeErr := provRows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close granted provider rows: %w", closeErr))
		}
	}()
	for provRows.Next() {
		var id string
		if err := provRows.Scan(&id); err != nil {
			return nil, nil, err
		}
		providerIDs = append(providerIDs, id)
	}
	return modelIDs, providerIDs, provRows.Err()
}

func (s *AIModelAccessService) CanUseModel(ctx context.Context, userID, modelID string) (bool, error) {
	if s.rbac != nil {
		ok, err := s.rbac.IsSuperAdmin(ctx, userID)
		if err == nil && ok {
			return true, nil
		}
	}
	restricted, err := s.HasAnyGrants(ctx)
	if err != nil {
		return false, err
	}
	if !restricted {
		return true, nil
	}
	access, err := s.UserAIAccess(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, id := range access.ModelIDs {
		if id == modelID {
			return true, nil
		}
	}
	return false, nil
}

func (s *AIModelAccessService) SetUserPreference(ctx context.Context, userID, purpose, modelID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_ai_model_preferences (user_id, purpose, model_id, updated_at)
		VALUES ($1, $2, $3::uuid, NOW())
		ON CONFLICT (user_id, purpose) DO UPDATE SET model_id = EXCLUDED.model_id, updated_at = NOW()
	`, userID, purpose, modelID)
	if err != nil {
		return fmt.Errorf("set user ai preference: %w", err)
	}
	return nil
}

func (s *AIModelAccessService) DeleteUserPreference(ctx context.Context, userID, purpose string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_ai_model_preferences WHERE user_id = $1 AND purpose = $2`, userID, purpose)
	if err != nil {
		return fmt.Errorf("delete user ai preference: %w", err)
	}
	return nil
}

func (s *AIModelAccessService) ListUserPreferences(ctx context.Context, userID string) ([]UserAIModelPreference, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id::text, purpose, model_id::text, updated_at
		FROM user_ai_model_preferences WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user ai preferences: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var list []UserAIModelPreference
	for rows.Next() {
		var p UserAIModelPreference
		if err := rows.Scan(&p.UserID, &p.Purpose, &p.ModelID, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *AIModelAccessService) GrantProviderWorkspace(ctx context.Context, workspaceID, providerID, grantedBy string) error {
	var gb sql.NullString
	if grantedBy != "" {
		gb = sql.NullString{String: grantedBy, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_provider_workspace_grants (workspace_id, provider_id, granted_by)
		VALUES ($1, $2::uuid, $3)
		ON CONFLICT DO NOTHING
	`, workspaceID, providerID, gb)
	return err
}

func (s *AIModelAccessService) RevokeProviderWorkspace(ctx context.Context, workspaceID, providerID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_provider_workspace_grants WHERE workspace_id = $1 AND provider_id = $2::uuid`, workspaceID, providerID)
	return err
}

func (s *AIModelAccessService) GrantModelWorkspace(ctx context.Context, workspaceID, modelID, grantedBy string) error {
	var gb sql.NullString
	if grantedBy != "" {
		gb = sql.NullString{String: grantedBy, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_model_workspace_grants (workspace_id, model_id, granted_by)
		VALUES ($1, $2::uuid, $3)
		ON CONFLICT DO NOTHING
	`, workspaceID, modelID, gb)
	return err
}

func (s *AIModelAccessService) RevokeModelWorkspace(ctx context.Context, workspaceID, modelID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_model_workspace_grants WHERE workspace_id = $1 AND model_id = $2::uuid`, workspaceID, modelID)
	return err
}

func (s *AIModelAccessService) GrantProviderRole(ctx context.Context, roleID, providerID, grantedBy string) error {
	var gb sql.NullString
	if grantedBy != "" {
		gb = sql.NullString{String: grantedBy, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_provider_role_grants (role_id, provider_id, granted_by)
		VALUES ($1, $2::uuid, $3)
		ON CONFLICT DO NOTHING
	`, roleID, providerID, gb)
	return err
}

func (s *AIModelAccessService) RevokeProviderRole(ctx context.Context, roleID, providerID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_provider_role_grants WHERE role_id = $1 AND provider_id = $2::uuid`, roleID, providerID)
	return err
}

func (s *AIModelAccessService) GrantModelRole(ctx context.Context, roleID, modelID, grantedBy string) error {
	var gb sql.NullString
	if grantedBy != "" {
		gb = sql.NullString{String: grantedBy, Valid: true}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_model_role_grants (role_id, model_id, granted_by)
		VALUES ($1, $2::uuid, $3)
		ON CONFLICT DO NOTHING
	`, roleID, modelID, gb)
	return err
}

func (s *AIModelAccessService) RevokeModelRole(ctx context.Context, roleID, modelID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_model_role_grants WHERE role_id = $1 AND model_id = $2::uuid`, roleID, modelID)
	return err
}

// AIModelAccessGrants aggregates all workspace/role AI sharing grants.
type AIModelAccessGrants struct {
	ProviderWorkspaces []AIProviderWorkspaceGrant `json:"provider_workspaces"`
	ModelWorkspaces    []AIModelWorkspaceGrant    `json:"model_workspaces"`
	ProviderRoles      []AIProviderRoleGrant      `json:"provider_roles"`
	ModelRoles         []AIModelRoleGrant         `json:"model_roles"`
}

func scanGrantedBy(gb sql.NullString) *string {
	if gb.Valid {
		return new(gb.String)
	}
	return nil
}

func listGrants[G any](ctx context.Context, db *sql.DB, query string, scan func(*sql.Rows) (G, error)) ([]G, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var grants []G
	for rows.Next() {
		g, err := scan(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	if grants == nil {
		grants = make([]G, 0)
	}
	return grants, rows.Err()
}

func (s *AIModelAccessService) listProviderWorkspaceGrants(ctx context.Context) ([]AIProviderWorkspaceGrant, error) {
	return listGrants(ctx, s.db, `SELECT workspace_id::text, provider_id::text, granted_by::text, granted_at FROM ai_provider_workspace_grants`,
		func(rows *sql.Rows) (AIProviderWorkspaceGrant, error) {
			var g AIProviderWorkspaceGrant
			var gb sql.NullString
			if err := rows.Scan(&g.WorkspaceID, &g.ProviderID, &gb, &g.GrantedAt); err != nil {
				return g, err
			}
			g.GrantedBy = scanGrantedBy(gb)
			return g, nil
		})
}

func (s *AIModelAccessService) listModelWorkspaceGrants(ctx context.Context) ([]AIModelWorkspaceGrant, error) {
	return listGrants(ctx, s.db, `SELECT workspace_id::text, model_id::text, granted_by::text, granted_at FROM ai_model_workspace_grants`,
		func(rows *sql.Rows) (AIModelWorkspaceGrant, error) {
			var g AIModelWorkspaceGrant
			var gb sql.NullString
			if err := rows.Scan(&g.WorkspaceID, &g.ModelID, &gb, &g.GrantedAt); err != nil {
				return g, err
			}
			g.GrantedBy = scanGrantedBy(gb)
			return g, nil
		})
}

func (s *AIModelAccessService) listProviderRoleGrants(ctx context.Context) ([]AIProviderRoleGrant, error) {
	return listGrants(ctx, s.db, `SELECT role_id::text, provider_id::text, granted_by::text, granted_at FROM ai_provider_role_grants`,
		func(rows *sql.Rows) (AIProviderRoleGrant, error) {
			var g AIProviderRoleGrant
			var gb sql.NullString
			if err := rows.Scan(&g.RoleID, &g.ProviderID, &gb, &g.GrantedAt); err != nil {
				return g, err
			}
			g.GrantedBy = scanGrantedBy(gb)
			return g, nil
		})
}

func (s *AIModelAccessService) listModelRoleGrants(ctx context.Context) ([]AIModelRoleGrant, error) {
	return listGrants(ctx, s.db, `SELECT role_id::text, model_id::text, granted_by::text, granted_at FROM ai_model_role_grants`,
		func(rows *sql.Rows) (AIModelRoleGrant, error) {
			var g AIModelRoleGrant
			var gb sql.NullString
			if err := rows.Scan(&g.RoleID, &g.ModelID, &gb, &g.GrantedAt); err != nil {
				return g, err
			}
			g.GrantedBy = scanGrantedBy(gb)
			return g, nil
		})
}

func (s *AIModelAccessService) ListAllGrants(ctx context.Context) (AIModelAccessGrants, error) {
	var out AIModelAccessGrants
	var err error
	if out.ProviderWorkspaces, err = s.listProviderWorkspaceGrants(ctx); err != nil {
		return out, err
	}
	if out.ModelWorkspaces, err = s.listModelWorkspaceGrants(ctx); err != nil {
		return out, err
	}
	if out.ProviderRoles, err = s.listProviderRoleGrants(ctx); err != nil {
		return out, err
	}
	if out.ModelRoles, err = s.listModelRoleGrants(ctx); err != nil {
		return out, err
	}
	return out, nil
}

// ExpandModelIDs merges explicit model grants with all active models from granted providers.
func ExpandModelIDs(access *UserAIAccess, providerModels map[string][]string) []string {
	if access == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range access.ModelIDs {
		add(id)
	}
	for _, pid := range access.ProviderIDs {
		for _, mid := range providerModels[pid] {
			add(mid)
		}
	}
	return out
}

// FilterAllowedModelIDs returns ids from candidates the user may use.
func FilterAllowedModelIDs(access *UserAIAccess, providerModels map[string][]string, candidates []string) []string {
	if access == nil || !access.Restricted {
		return candidates
	}
	allowed := ExpandModelIDs(access, providerModels)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}
	var out []string
	for _, c := range candidates {
		if _, ok := allowedSet[c]; ok {
			out = append(out, c)
		}
	}
	return out
}
