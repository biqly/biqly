package datasource

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/maphash"
	"log/slog"
	"strconv"
	"sync"

	"golang.org/x/sync/singleflight"
)

var poolKeySeed = maphash.MakeSeed()

// PoolCache reuses *sql.DB pools across query executions so we don't pay the
// open/close cost on every query. Pools are keyed by datasource ID + a hash
// of the resolved DSN so a rotated credential automatically yields a fresh
// pool instead of serving the old one.
//
// Callers MUST NOT call Close() on pools returned by Get — the cache owns
// the lifecycle and closes them in PoolCache.Close.
type PoolCache struct {
	mu    sync.RWMutex
	pools map[string]*sql.DB
	sf    singleflight.Group
}

// NewPoolCache constructs an empty cache.
func NewPoolCache() *PoolCache {
	return &PoolCache{pools: make(map[string]*sql.DB)}
}

// Get returns a pooled *sql.DB for the (datasourceID, dsn) pair, opening a
// new pool via driver.Open the first time it is requested.
//
// Concurrent callers for the same key block on a single open and share the
// resulting handle.
func (p *PoolCache) Get(ctx context.Context, driver Driver, datasourceID, dsn string) (*sql.DB, error) {
	if p == nil {
		return nil, errors.New("pool cache is nil")
	}
	if driver == nil {
		return nil, errors.New("pool cache: nil driver")
	}
	key := poolKey(datasourceID, dsn)

	p.mu.RLock()
	if db, ok := p.pools[key]; ok {
		p.mu.RUnlock()
		return db, nil
	}
	p.mu.RUnlock()

	result, err, _ := p.sf.Do(key, func() (any, error) {
		db, err := driver.Open(ctx, dsn)
		if err != nil {
			return nil, err
		}
		p.mu.Lock()
		p.pools[key] = db
		p.mu.Unlock()
		return db, nil
	})
	if err != nil {
		return nil, err
	}
	db, ok := result.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("pool cache: unexpected type %T", result)
	}
	return db, nil
}

// Invalidate removes and closes all cached pools whose key prefix matches
// datasourceID. Use when the datasource is deleted, its DSN rotated, or
// connection settings changed.
func (p *PoolCache) Invalidate(datasourceID string) {
	if p == nil || datasourceID == "" {
		return
	}
	prefix := datasourceID + "|"
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, db := range p.pools {
		if key == datasourceID || hasKeyPrefix(key, prefix) {
			delete(p.pools, key)
			if err := db.Close(); err != nil {
				slog.Warn("pool cache: close pool failed", "datasource_id", datasourceID, "error", err)
			}
		}
	}
}

// Close drains and closes every cached pool. Safe to call multiple times.
func (p *PoolCache) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	pools := p.pools
	p.pools = make(map[string]*sql.DB)
	p.mu.Unlock()

	var errs []error
	for _, db := range pools {
		if err := db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("pool cache close: %d pool(s) failed: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// poolKey hashes the DSN portion of the key so cache lookups don't keep raw
// credentials in long-lived map keys. The datasource ID is kept in clear so
// Invalidate(datasourceID) can scan by prefix.
func poolKey(datasourceID, dsn string) string {
	var h maphash.Hash
	h.SetSeed(poolKeySeed)
	_, _ = h.WriteString(dsn)
	return datasourceID + "|" + strconv.FormatUint(h.Sum64(), 16)
}

func hasKeyPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
