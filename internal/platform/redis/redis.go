// Package rediscache provides Redis caching utilities.
package rediscache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/query"
	"github.com/redis/go-redis/v9"
)

// Cache provides query result caching.
type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCache creates a new query cache.
func NewCache(addr string, ttl time.Duration) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Cache{
		client: client,
		ttl:    ttl,
	}, nil
}

// NewCacheDisabled creates a cache that does nothing (when Redis is unavailable).
func NewCacheDisabled() *Cache {
	return &Cache{}
}

// Key generates a cache key from the logical query.
//
// JSON encoding is used instead of fmt's %+v because Go's print formatter
// renders maps with non-deterministic key order — two equivalent queries
// could hash to different keys, breaking the cache. encoding/json sorts
// map keys, so the same LogicalQuery produces the same fingerprint every
// run.
func (*Cache) Key(datasourceID, modelID string, lq query.LogicalQuery, userScope string) string {
	data := struct {
		DatasourceID string             `json:"d"`
		ModelID      string             `json:"m"`
		Query        query.LogicalQuery `json:"q"`
		UserScope    string             `json:"u"`
	}{datasourceID, modelID, lq, userScope}

	encoded, err := json.Marshal(data)
	if err != nil {
		// JSON encoding of these shapes does not fail; fall back so cache
		// behaves like a miss rather than a panic.
		encoded = fmt.Appendf(nil, "%+v", data)
	}
	hash := sha256.Sum256(encoded)
	return fmt.Sprintf("bi:query:%x", hash)
}

// Get retrieves a cached result.
func (c *Cache) Get(ctx context.Context, key string) (*query.Result, bool) {
	if c.client == nil {
		return nil, false
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var result query.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	return &result, true
}

// Set stores a query result in the cache.
func (c *Cache) Set(ctx context.Context, key string, result *query.Result) error {
	if c.client == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	return c.client.Set(ctx, key, data, c.ttl).Err()
}

// Invalidate removes a key from the cache.
func (c *Cache) Invalidate(ctx context.Context, key string) error {
	if c.client == nil {
		return nil
	}
	return c.client.Del(ctx, key).Err()
}

// Close closes the Redis connection.
func (c *Cache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}
