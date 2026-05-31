package ai

import (
	"context"
	"log/slog"
	"sync"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
)

// PurposeProvider is a providerpkg.Provider that resolves the live backend for
// a single purpose from a ProviderStore on every call. It rebuilds the
// underlying provider only when the store's cache version changes, so admin
// edits to the default model take effect without a process restart. When the
// database has no default for the purpose it delegates to a static fallback
// provider built from the env config.
type PurposeProvider struct {
	store    *ProviderStore
	purpose  Purpose
	fallback providerpkg.Provider

	mu           sync.Mutex
	built        providerpkg.Provider
	builtVersion int64
}

// NewPurposeProvider wraps store for the given purpose. fallback is used when
// the store has no DB-configured default (or fails to build a provider).
func NewPurposeProvider(store *ProviderStore, purpose Purpose, fallback providerpkg.Provider) *PurposeProvider {
	return &PurposeProvider{store: store, purpose: purpose, fallback: fallback}
}

// Generate resolves the current backend and forwards the prompt.
func (p *PurposeProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	return p.current().Generate(ctx, prompt)
}

// GenerateAt resolves the current backend and forwards the prompt with an
// explicit temperature override.
func (p *PurposeProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (providerpkg.GenerationResult, error) {
	return p.current().GenerateAt(ctx, prompt, temperature)
}

func (p *PurposeProvider) current() providerpkg.Provider {
	cfg, ok := p.store.ChatConfigForPurpose(p.purpose)
	if !ok {
		return p.fallback
	}
	version := p.store.CacheVersion()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.built != nil && p.builtVersion == version {
		return p.built
	}
	prov, err := providerpkg.NewProvider(cfg)
	if err != nil {
		slog.Warn("purpose provider build failed; using fallback", "purpose", p.purpose, "error", err)
		p.builtVersion = version
		p.built = nil
		return p.fallback
	}
	if p.built != nil {
		closeProvider(p.built)
	}
	p.built = prov
	p.builtVersion = version
	return p.built
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
