package auth

import (
	"context"
	"fmt"
	"slices"
)

const RoleSuperAdmin = "super_admin"

type ScopeType string

const (
	ScopeGlobal     ScopeType = "global"
	ScopeWorkspace  ScopeType = "workspace"
	ScopeDatasource ScopeType = "datasource"
	ScopeModel      ScopeType = "model"
)

type PermissionCheck struct {
	UserID     string
	Permission string
	ScopeType  ScopeType
	ScopeID    string
}

type RBACService struct {
	repo *RBACRepository
}

func NewRBACService(repo *RBACRepository) *RBACService {
	return &RBACService{repo: repo}
}

func (s *RBACService) HasRole(ctx context.Context, userID, role string) (bool, error) {
	roles, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(roles, role), nil
}

func (s *RBACService) IsSuperAdmin(ctx context.Context, userID string) (bool, error) {
	return s.HasRole(ctx, userID, RoleSuperAdmin)
}

func (s *RBACService) Check(ctx context.Context, check PermissionCheck) (bool, error) {
	if check.UserID == "" || check.Permission == "" {
		return false, fmt.Errorf("user_id and permission are required")
	}

	isSuper, err := s.IsSuperAdmin(ctx, check.UserID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}

	globalPerms, err := s.repo.GetUserPermissions(ctx, check.UserID)
	if err != nil {
		return false, err
	}
	if slices.Contains(globalPerms, check.Permission) {
		return true, nil
	}

	if check.ScopeType == ScopeWorkspace && check.ScopeID != "" {
		wsPerms, err := s.repo.GetUserWorkspacePermissions(ctx, check.UserID, check.ScopeID)
		if err != nil {
			return false, err
		}
		if slices.Contains(wsPerms, check.Permission) {
			return true, nil
		}
	}

	return false, nil
}

func (s *RBACService) RequireAny(ctx context.Context, userID string, permissions ...string) (bool, error) {
	isSuper, err := s.IsSuperAdmin(ctx, userID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}

	userPerms, err := s.repo.GetUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, p := range permissions {
		if slices.Contains(userPerms, p) {
			return true, nil
		}
	}
	return false, nil
}

func (s *RBACService) GetEffectivePermissions(ctx context.Context, userID, workspaceID string) ([]string, error) {
	isSuper, err := s.IsSuperAdmin(ctx, userID)
	if err != nil {
		return nil, err
	}
	if isSuper {
		return s.repo.ListAllPermissionNames(ctx)
	}

	globalPerms, err := s.repo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	if workspaceID == "" {
		return globalPerms, nil
	}

	wsPerms, err := s.repo.GetUserWorkspacePermissions(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]struct{}, len(globalPerms)+len(wsPerms))
	for _, p := range globalPerms {
		merged[p] = struct{}{}
	}
	for _, p := range wsPerms {
		merged[p] = struct{}{}
	}

	result := make([]string, 0, len(merged))
	for p := range merged {
		result = append(result, p)
	}
	return result, nil
}
