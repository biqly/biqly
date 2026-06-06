package rbac

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

const RoleSuperAdmin = "super_admin"

type ScopeType string

const (
	ScopeGlobal     ScopeType = "global"
	ScopeWorkspace  ScopeType = "workspace"
	ScopeDatasource ScopeType = "datasource"
	ScopeModel      ScopeType = "model"
)

type UserRoleInfo struct {
	RoleID    string    `json:"role_id"`
	RoleName  string    `json:"role_name"`
	ScopeType ScopeType `json:"scope_type"`
	ScopeID   string    `json:"scope_id"`
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Permission struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	CreatedAt   time.Time `json:"created_at"`
}

type PermissionCheck struct {
	UserID     string
	Permission string
	ScopeType  ScopeType
	ScopeID    string
}

type checkCacheEntry struct {
	allowed bool
	at      time.Time
}

type Service struct {
	repo       *RBACRepository
	checkMu    sync.RWMutex
	checkCache map[string]checkCacheEntry
	checkTTL   time.Duration
}

func NewRBACService(repo *RBACRepository) *Service {
	return &Service{
		repo:       repo,
		checkCache: make(map[string]checkCacheEntry),
		checkTTL:   2 * time.Minute,
	}
}

func checkCacheKey(check PermissionCheck) string {
	return fmt.Sprintf("%s:%s:%s:%s", check.UserID, check.Permission, check.ScopeType, check.ScopeID)
}

func (s *Service) evictExpiredCheckLocked(now time.Time) {
	for k, e := range s.checkCache {
		if now.Sub(e.at) >= s.checkTTL {
			delete(s.checkCache, k)
		}
	}
}

func (s *Service) HasRole(ctx context.Context, userID, role string) (bool, error) {
	roles, err := s.repo.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(roles, role), nil
}

func (s *Service) IsSuperAdmin(ctx context.Context, userID string) (bool, error) {
	return s.HasRole(ctx, userID, RoleSuperAdmin)
}

func (s *Service) Check(ctx context.Context, check PermissionCheck) (bool, error) {
	if check.UserID == "" || check.Permission == "" {
		return false, errors.New("user_id and permission are required")
	}

	cacheKey := checkCacheKey(check)
	s.checkMu.RLock()
	if e, ok := s.checkCache[cacheKey]; ok && time.Since(e.at) < s.checkTTL {
		allowed := e.allowed
		s.checkMu.RUnlock()
		return allowed, nil
	}
	s.checkMu.RUnlock()

	isSuper, err := s.IsSuperAdmin(ctx, check.UserID)
	if err != nil {
		return false, err
	}
	if isSuper {
		s.storeCheckResult(cacheKey, true)
		return true, nil
	}

	globalPerms, err := s.repo.GetUserPermissions(ctx, check.UserID)
	if err != nil {
		return false, err
	}
	if slices.Contains(globalPerms, check.Permission) {
		s.storeCheckResult(cacheKey, true)
		return true, nil
	}

	if check.ScopeType != "" && check.ScopeType != ScopeGlobal && check.ScopeID != "" {
		scopedPerms, err := s.repo.GetUserScopedPermissions(ctx, check.UserID, check.ScopeType, check.ScopeID)
		if err != nil {
			return false, err
		}
		if slices.Contains(scopedPerms, check.Permission) {
			s.storeCheckResult(cacheKey, true)
			return true, nil
		}
	}

	if check.ScopeType == ScopeWorkspace && check.ScopeID != "" {
		wsPerms, err := s.repo.GetUserWorkspacePermissions(ctx, check.UserID, check.ScopeID)
		if err != nil {
			return false, err
		}
		if slices.Contains(wsPerms, check.Permission) {
			s.storeCheckResult(cacheKey, true)
			return true, nil
		}
	}

	s.storeCheckResult(cacheKey, false)
	return false, nil
}

func (s *Service) storeCheckResult(key string, allowed bool) {
	now := time.Now()
	s.checkMu.Lock()
	s.checkCache[key] = checkCacheEntry{allowed: allowed, at: now}
	s.evictExpiredCheckLocked(now)
	s.checkMu.Unlock()
}

func (s *Service) RequireAny(ctx context.Context, userID string, permissions ...string) (bool, error) {
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

func (s *Service) GetEffectivePermissions(ctx context.Context, userID, workspaceID string) ([]string, error) {
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
	scopedPerms, err := s.repo.GetUserScopedPermissions(ctx, userID, ScopeWorkspace, workspaceID)
	if err != nil {
		return nil, err
	}
	for _, p := range scopedPerms {
		merged[p] = struct{}{}
	}

	result := make([]string, 0, len(merged))
	for p := range merged {
		result = append(result, p)
	}
	return result, nil
}
