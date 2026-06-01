package ai

import (
	"context"
	"log/slog"
	"sync"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
)

// PurposeProvider is a providerpkg.Provider that resolves the live backend for
// a single purpose from a ProviderStore on every call. It rebuilds the
// underlying provider only when the store's cache version changes, so admin
// edits to the default model take effect without a process restart. When the
// database has no default for the purpose it delegates to a static fallback
// provider built from the env config.
// ConfigResolver optionally overrides the model used per request (e.g. user preference).
type ConfigResolver interface {
	ChatConfigForPurpose(ctx context.Context, purpose Purpose) (config.AIConfig, bool)
}

type PurposeProvider struct {
	store    *ProviderStore
	purpose  Purpose
	fallback providerpkg.Provider
	resolver ConfigResolver

	mu           sync.Mutex
	built        providerpkg.Provider
	builtVersion int64
	builtUserKey string
}

// NewPurposeProvider wraps store for the given purpose. fallback is used when
// the store has no DB-configured default (or fails to build a provider).
// resolver may be nil.
func NewPurposeProvider(store *ProviderStore, purpose Purpose, fallback providerpkg.Provider, resolver ConfigResolver) *PurposeProvider {
	return &PurposeProvider{store: store, purpose: purpose, fallback: fallback, resolver: resolver}
}

// SetResolver attaches a per-request config resolver and clears the cached provider.
func (p *PurposeProvider) SetResolver(resolver ConfigResolver) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resolver = resolver
	p.built = nil
	p.builtUserKey = ""
}

// Generate resolves the current backend and forwards the prompt.
func (p *PurposeProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	return p.current(ctx).Generate(ctx, prompt)
}

// GenerateAt resolves the current backend and forwards the prompt with an
// explicit temperature override.
func (p *PurposeProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (providerpkg.GenerationResult, error) {
	return p.current(ctx).GenerateAt(ctx, prompt, temperature)
}

func (p *PurposeProvider) current(ctx context.Context) providerpkg.Provider {
	var cfg config.AIConfig
	var ok bool
	if p.resolver != nil {
		cfg, ok = p.resolver.ChatConfigForPurpose(ctx, p.purpose)
	} else {
		cfg, ok = p.store.ChatConfigForPurpose(p.purpose)
	}
	if !ok {
		return p.fallback
	}
	version := p.store.CacheVersion()
	userKey := ""
	if p.resolver != nil {
		userKey = userConfigCacheKey(ctx)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.built != nil && p.builtVersion == version && p.builtUserKey == userKey {
		return p.built
	}
	prov, err := providerpkg.NewProvider(cfg)
	if err != nil {
		slog.Warn("purpose provider build failed; using fallback", "purpose", p.purpose, "error", err)
		p.builtVersion = version
		p.builtUserKey = userKey
		p.built = nil
		return p.fallback
	}
	if p.built != nil {
		closeProvider(p.built)
	}
	p.built = prov
	p.builtVersion = version
	p.builtUserKey = userKey
	return p.built
}

func userConfigCacheKey(ctx context.Context) string {
	return UserIDFromContext(ctx)
}

// Close releases the currently built provider's idle connections.
func (p *PurposeProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.built != nil {
		closeProvider(p.built)
		p.built = nil
	}
	return nil
}
