package semantic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ResolvedCompositeCache caches the resolved (merged) SemanticModel for a
// composite so the query runtime avoids re-reading and decoding the published
// snapshot on every run. Implementations must be safe for concurrent use.
type ResolvedCompositeCache interface {
	// Get returns the cached resolved model and true on a hit.
	Get(ctx context.Context, compositeID string) (*SemanticModel, bool)
	// Set stores the resolved model for a composite at the given version.
	Set(ctx context.Context, compositeID string, version int, model *SemanticModel)
	// Invalidate drops any cached entry for the composite. Called whenever a
	// component model or the composite itself is (re)published or rolled back.
	Invalidate(ctx context.Context, compositeID string)
}

// redisCompositeCache is a Redis-backed ResolvedCompositeCache.
type redisCompositeCache struct {
	client *redis.Client
	ttl    time.Duration
}

type cachedResolvedComposite struct {
	Version  int            `json:"version"`
	Resolved *SemanticModel `json:"resolved"`
}

// NewRedisCompositeCache builds a ResolvedCompositeCache over the given client.
// A nil client yields a nil cache so callers can wire it unconditionally.
func NewRedisCompositeCache(client *redis.Client, ttl time.Duration) ResolvedCompositeCache {
	if client == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &redisCompositeCache{client: client, ttl: ttl}
}

// compositeCacheKey builds the canonical key. The version is embedded so cache
// dumps stay debuggable; lookups use the unversioned key because the runtime
// asks for "the latest published" without knowing the version up front, and
// publish/rollback invalidate the key explicitly.
func compositeCacheKey(compositeID string) string {
	return fmt.Sprintf("composite:%s:resolved", compositeID)
}

func (c *redisCompositeCache) Get(ctx context.Context, compositeID string) (*SemanticModel, bool) {
	raw, err := c.client.Get(ctx, compositeCacheKey(compositeID)).Bytes()
	if err != nil {
		return nil, false
	}
	var entry cachedResolvedComposite
	if err := json.Unmarshal(raw, &entry); err != nil || entry.Resolved == nil {
		return nil, false
	}
	return entry.Resolved, true
}

func (c *redisCompositeCache) Set(ctx context.Context, compositeID string, version int, model *SemanticModel) {
	if model == nil {
		return
	}
	payload, err := json.Marshal(cachedResolvedComposite{Version: version, Resolved: model})
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, compositeCacheKey(compositeID), payload, c.ttl).Err()
}

func (c *redisCompositeCache) Invalidate(ctx context.Context, compositeID string) {
	_ = c.client.Del(ctx, compositeCacheKey(compositeID)).Err()
}
