package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
)

// ConfigResolver PurposeProvider is a providerpkg.Provider that resolves the live backend for
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
	p.mu.Lock()
	resolver := p.resolver
	fallback := p.fallback
	p.mu.Unlock()

	var cfg config.AIConfig
	var ok bool
	if resolver != nil {
		cfg, ok = resolver.ChatConfigForPurpose(ctx, p.purpose)
	} else {
		cfg, ok = p.store.ChatConfigForPurpose(p.purpose)
	}
	if !ok {
		if fallback != nil {
			return fallback
		}
		return notConfiguredProvider{purpose: p.purpose}
	}
	version := p.store.CacheVersion()
	userKey := ""
	if resolver != nil {
		userKey = userConfigCacheKey(ctx)
	}

	p.mu.Lock()
	if p.built != nil && p.builtVersion == version && p.builtUserKey == userKey {
		prov := p.built
		p.mu.Unlock()
		return prov
	}
	p.mu.Unlock()

	prov, err := providerpkg.NewProvider(cfg)
	if err != nil {
		slog.Warn("purpose provider build failed", "purpose", p.purpose, "error", err)
		p.mu.Lock()
		p.builtVersion = version
		p.builtUserKey = userKey
		p.built = nil
		p.mu.Unlock()
		if fallback != nil {
			return fallback
		}
		return notConfiguredProvider{purpose: p.purpose, err: err}
	}

	p.mu.Lock()
	if p.built != nil && p.builtVersion == version && p.builtUserKey == userKey {
		cached := p.built
		p.mu.Unlock()
		closeProvider(prov)
		return cached
	}
	old := p.built
	p.built = prov
	p.builtVersion = version
	p.builtUserKey = userKey
	p.mu.Unlock()
	if old != nil {
		closeProvider(old)
	}
	return prov
}

func userConfigCacheKey(ctx context.Context) string {
	return UserIDFromContext(ctx)
}

// notConfiguredProvider is returned when no model is configured in the database
// for a purpose and there is no fallback. Its Generate calls fail with a clear,
// actionable error instead of panicking on a nil provider.
type notConfiguredProvider struct {
	purpose Purpose
	err     error
}

func (n notConfiguredProvider) errorf() error {
	if n.err != nil {
		return fmt.Errorf("no usable AI model configured for %q: %w; configure a provider and default model under Administration → AI Providers", n.purpose, n.err)
	}
	return fmt.Errorf("no AI model configured for %q; configure a provider and default model under Administration → AI Providers", n.purpose)
}

func (n notConfiguredProvider) Generate(_ context.Context, _ string) (providerpkg.GenerationResult, error) {
	return providerpkg.GenerationResult{}, n.errorf()
}

func (n notConfiguredProvider) GenerateAt(_ context.Context, _ string, _ float64) (providerpkg.GenerationResult, error) {
	return providerpkg.GenerationResult{}, n.errorf()
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
