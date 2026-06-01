package app

import (
	"context"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// AIUserConfigResolver picks the chat model for a request: user preference when
// allowed, otherwise the global default for the purpose.
type AIUserConfigResolver struct {
	store *ai.ProviderStore
	auth  *bimw.AuthClient
}

func NewAIUserConfigResolver(store *ai.ProviderStore, auth *bimw.AuthClient) *AIUserConfigResolver {
	return &AIUserConfigResolver{store: store, auth: auth}
}

func (r *AIUserConfigResolver) ChatConfigForPurpose(ctx context.Context, purpose ai.Purpose) (config.AIConfig, bool) {
	return r.resolve(ctx, purpose)
}

func (r *AIUserConfigResolver) resolve(ctx context.Context, purpose ai.Purpose) (config.AIConfig, bool) {
	userID := ai.UserIDFromContext(ctx)
	if userID == "" || r.auth == nil {
		return r.store.ChatConfigForPurpose(purpose)
	}

	access, err := r.auth.UserAIAccess(ctx, userID)
	if err != nil || access == nil {
		return r.store.ChatConfigForPurpose(purpose)
	}

	purposeStr := string(purpose)
	if prefID := access.Preferences[purposeStr]; prefID != "" {
		if r.userMayUseModel(ctx, access, prefID) {
			if cfg, ok := r.store.ChatConfigForModelUUID(ctx, prefID); ok {
				return cfg, true
			}
		}
	}

	return r.store.ChatConfigForPurpose(purpose)
}

func (r *AIUserConfigResolver) userMayUseModel(ctx context.Context, access *bimw.UserAIAccess, modelUUID string) bool {
	if access == nil || !access.Restricted {
		return true
	}
	providerModels, err := r.store.ActiveModelUUIDsByProviders(ctx, access.ProviderIDs)
	if err != nil {
		return false
	}
	seen := make(map[string]struct{})
	for _, id := range access.ModelIDs {
		seen[id] = struct{}{}
	}
	for _, ids := range providerModels {
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	_, ok := seen[modelUUID]
	return ok
}
