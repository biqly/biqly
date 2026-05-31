package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/redis/go-redis/v9"
)

// ResponseCache defines the interface for caching AI query responses.
type ResponseCache interface {
	Get(ctx context.Context, fingerprint string) (*AIResponse, error)
	Put(ctx context.Context, fingerprint string, resp *AIResponse, ttl time.Duration) error
	InvalidateModel(ctx context.Context, modelID string) error
	Close() error
}

// globalCache is used by semantic.OnModelPublish to invalidate cache items.
var globalCache ResponseCache

func init() {
	semantic.OnModelPublish = func(ctx context.Context, modelID string) {
		if globalCache != nil {
			if err := globalCache.InvalidateModel(ctx, modelID); err != nil {
				slog.Error("failed to invalidate LLM response cache", "model_id", modelID, "error", err)
			} else {
				slog.Info("invalidated LLM response cache for model", "model_id", modelID)
			}
		}
	}
}

// RedisResponseCache is a Redis-backed implementation of ResponseCache.
type RedisResponseCache struct {
	client *redis.Client
}

// NewRedisResponseCache creates a new RedisResponseCache and registers it globally.
func NewRedisResponseCache(client *redis.Client) *RedisResponseCache {
	cache := &RedisResponseCache{client: client}
	globalCache = cache
	return cache
}

func (r *RedisResponseCache) Get(ctx context.Context, fingerprint string) (*AIResponse, error) {
	if r.client == nil {
		return nil, nil
	}
	val, err := r.client.Get(ctx, fingerprint).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var resp AIResponse
	if err := json.Unmarshal(val, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (r *RedisResponseCache) Put(ctx context.Context, fingerprint string, resp *AIResponse, ttl time.Duration) error {
	if r.client == nil {
		return nil
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, fingerprint, data, ttl).Err()
}

func (r *RedisResponseCache) InvalidateModel(ctx context.Context, modelID string) error {
	if r.client == nil {
		return nil
	}
	pattern := fmt.Sprintf("bi:ai:cache:%s:*", modelID)
	var cursor uint64
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *RedisResponseCache) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

// GenerateCacheKey generates a deterministic cache key.
func GenerateCacheKey(question string, modelID string, deniedFields []string) string {
	sortedDenied := make([]string, len(deniedFields))
	copy(sortedDenied, deniedFields)
	sort.Strings(sortedDenied)
	deniedJoined := strings.Join(sortedDenied, ",")

	input := fmt.Sprintf("%s:%s:%s", question, modelID, deniedJoined)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("bi:ai:cache:%s:%x", modelID, hash)
}
