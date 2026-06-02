package semantic

import (
	"context"
	"testing"
	"time"
)

// fakeCompositeCache is an in-memory ResolvedCompositeCache for testing the
// read-through / invalidate behaviour without a live Redis.
type fakeCompositeCache struct {
	store       map[string]*SemanticModel
	getCalls    int
	setCalls    int
	invalidates int
}

func newFakeCompositeCache() *fakeCompositeCache {
	return &fakeCompositeCache{store: map[string]*SemanticModel{}}
}

func (f *fakeCompositeCache) Get(_ context.Context, compositeID string) (*SemanticModel, bool) {
	f.getCalls++
	m, ok := f.store[compositeID]
	return m, ok
}

func (f *fakeCompositeCache) Set(_ context.Context, compositeID string, _ int, model *SemanticModel) {
	f.setCalls++
	f.store[compositeID] = model
}

func (f *fakeCompositeCache) Invalidate(_ context.Context, compositeID string) {
	f.invalidates++
	delete(f.store, compositeID)
}

func TestRedisCompositeCache_NilClientYieldsNilCache(t *testing.T) {
	if cache := NewRedisCompositeCache(nil, time.Hour); cache != nil {
		t.Fatalf("expected nil cache for nil client, got %T", cache)
	}
}

func TestFakeCompositeCache_GetSetInvalidate(t *testing.T) {
	ctx := context.Background()
	cache := newFakeCompositeCache()
	const id = "composite-1"

	if _, ok := cache.Get(ctx, id); ok {
		t.Fatal("expected miss on empty cache")
	}

	model := &SemanticModel{ID: id, Name: "merged"}
	cache.Set(ctx, id, 3, model)

	got, ok := cache.Get(ctx, id)
	if !ok || got != model {
		t.Fatalf("expected hit returning stored model, ok=%v got=%v", ok, got)
	}

	cache.Invalidate(ctx, id)
	if _, ok := cache.Get(ctx, id); ok {
		t.Fatal("expected miss after invalidate")
	}
	if cache.invalidates != 1 {
		t.Fatalf("expected 1 invalidate, got %d", cache.invalidates)
	}
}

func TestCompositeCacheKey_Format(t *testing.T) {
	if got := compositeCacheKey("abc"); got != "composite:abc:resolved" {
		t.Fatalf("unexpected key %q", got)
	}
}
