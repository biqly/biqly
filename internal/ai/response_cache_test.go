package ai

import (
	"context"
	"testing"
)

func TestGenerateCacheKeyDeterministic(t *testing.T) {
	q := "how many orders?"
	mID := "model-123"
	
	// Ordered fields vs unordered fields
	key1 := GenerateCacheKey(q, mID, []string{"b", "a", "c"})
	key2 := GenerateCacheKey(q, mID, []string{"a", "b", "c"})
	
	if key1 != key2 {
		t.Fatalf("expected keys to be deterministic and identical, got %s and %s", key1, key2)
	}

	// Different questions must produce different keys
	key3 := GenerateCacheKey("different question", mID, []string{"a", "b", "c"})
	if key1 == key3 {
		t.Fatalf("expected different keys for different questions, got identical %s", key1)
	}

	// Different model IDs must produce different keys
	key4 := GenerateCacheKey(q, "model-456", []string{"a", "b", "c"})
	if key1 == key4 {
		t.Fatalf("expected different keys for different model IDs, got identical %s", key1)
	}
}

func TestNilRedisResponseCache(t *testing.T) {
	// A nil-backed RedisResponseCache must not panic on any operation.
	cache := NewRedisResponseCache(nil)
	ctx := context.Background()

	resp, err := cache.Get(ctx, "some-key")
	if err != nil {
		t.Errorf("unexpected error on Get: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response on cache miss/disabled, got: %v", resp)
	}

	err = cache.Put(ctx, "some-key", &AIResponse{}, 0)
	if err != nil {
		t.Errorf("unexpected error on Put: %v", err)
	}

	err = cache.InvalidateModel(ctx, "model-123")
	if err != nil {
		t.Errorf("unexpected error on InvalidateModel: %v", err)
	}

	err = cache.Close()
	if err != nil {
		t.Errorf("unexpected error on Close: %v", err)
	}
}
