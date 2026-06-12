package handlers

import (
	"context"
	"fmt"

	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// resolveDatasourceScope returns the set of datasource IDs the current
// caller may access, intersected with the active workspace's attached
// datasources (if a workspace is active).
//
// The boolean reports whether scoping applies: it is false when auth is
// disabled, the request carries no user, or the caller is a super admin.
//
// If requireWorkspace is true and no workspace ID is found in context,
// scoping does not apply (returns nil, false, nil). This matches the
// behavior needed for history query endpoints.
func resolveDatasourceScope(ctx context.Context, cfg *config.Config, requireWorkspace bool) (map[string]struct{}, bool, error) {
	if !cfg.Auth.Enabled {
		return nil, false, nil
	}
	userID := bimw.UserID(ctx)
	if userID == "" || bimw.HasRole(ctx, bimw.RoleSuperAdmin) {
		return nil, false, nil
	}
	wsID := bimw.WorkspaceID(ctx)
	if wsID == "" && requireWorkspace {
		return nil, false, nil
	}

	authClient := bimw.SharedAuthClient(cfg.Auth.ServiceURL, cfg.Auth.InternalToken)
	allowed, err := authClient.ListUserDatasources(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("list user datasources: %w", err)
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}

	if wsID != "" {
		wsIDs, err := authClient.ListWorkspaceDatasources(ctx, wsID)
		if err != nil {
			return nil, false, fmt.Errorf("list workspace datasources: %w", err)
		}
		wsSet := make(map[string]struct{}, len(wsIDs))
		for _, id := range wsIDs {
			wsSet[id] = struct{}{}
		}
		for id := range allowedSet {
			if _, ok := wsSet[id]; !ok {
				delete(allowedSet, id)
			}
		}
	}

	return allowedSet, true, nil
}
