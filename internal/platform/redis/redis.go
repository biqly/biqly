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
func (c *Cache) Key(datasourceID, modelID string, lq query.LogicalQuery, userScope string) string {
	data := struct {
		DatasourceID string               `json:"d"`
		ModelID      string               `json:"m"`
		Query        query.LogicalQuery   `json:"q"`
		UserScope    string               `json:"u"`
	}{datasourceID, modelID, lq, userScope}

	hash := sha256.Sum256([]byte(fmt.Sprintf("%+v", data)))
	return fmt.Sprintf("bi:query:%x", hash)
}

// Get retrieves a cached result.
func (c *Cache) Get(ctx context.Context, key string) (*query.QueryResult, bool) {
	if c.client == nil {
		return nil, false
	}

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}

	var result query.QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	return &result, true
}

// Set stores a query result in the cache.
func (c *Cache) Set(ctx context.Context, key string, result *query.QueryResult) error {
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
