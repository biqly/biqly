package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
)

// runtimeOverridesTTL bounds cross-replica staleness after an admin PUT; the
// writing replica invalidates immediately, others converge within the TTL.
const runtimeOverridesTTL = 30 * time.Second

// runtimeOverrides memoizes one ai_runtime_config domain row. T is the
// pointer-field overrides struct for the domain; nil fields fall back to the
// environment-derived defaults. The zero value is ready to use.
type runtimeOverrides[T any] struct {
	mu      sync.Mutex
	cached  T
	expires time.Time
}

// load returns the cached DB overrides for key, refreshing once per TTL
// window. Errors degrade to "no overrides" so query handling never fails on
// the config path.
func (s *runtimeOverrides[T]) load(ctx context.Context, repo *metadata.Repository, key string) T {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Now().Before(s.expires) {
		return s.cached
	}
	s.cached = fetchRuntimeOverrides[T](ctx, repo, key)
	s.expires = time.Now().Add(runtimeOverridesTTL)
	return s.cached
}

func (s *runtimeOverrides[T]) invalidate() {
	s.mu.Lock()
	s.expires = time.Time{}
	s.mu.Unlock()
}

// fetchRuntimeOverrides reads and decodes one ai_runtime_config row without
// caching. A missing row, decode failure, or DB error all yield the zero
// overrides (environment defaults apply).
func fetchRuntimeOverrides[T any](ctx context.Context, repo *metadata.Repository, key string) T {
	var ov T
	if repo == nil {
		return ov
	}
	raw, err := repo.GetAIRuntimeConfig(ctx, key)
	switch {
	case err == nil:
		if jsonErr := sonic.Unmarshal(raw, &ov); jsonErr != nil {
			slog.WarnContext(ctx, "decode runtime config", "key", key, "error", jsonErr)
			var zero T
			return zero
		}
	case errors.Is(err, sql.ErrNoRows):
		// Key unset — environment defaults apply.
	default:
		slog.WarnContext(ctx, "load runtime config", "key", key, "error", err)
	}
	return ov
}
