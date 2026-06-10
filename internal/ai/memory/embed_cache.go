package memory

import (
	"container/list"
	"sync"
	"time"
)

// questionEmbedCacheTTL bounds reuse of an incoming-question embedding. The
// win comes from multi-round clarification sessions re-embedding the same
// question within seconds; a short TTL keeps memory flat and bounds staleness.
// The embedding model is part of the key, so a model switch never serves a
// vector from the previous model.
const (
	questionEmbedCacheTTL  = 5 * time.Minute
	questionEmbedCacheSize = 512
)

// questionEmbedCache is shared across recall calls within one process.
var questionEmbedCache = newEmbedCache(questionEmbedCacheSize, questionEmbedCacheTTL)

type embedCacheEntry struct {
	key       string
	vec       []float32
	expiresAt time.Time
}

// embedCache is a TTL-bounded LRU keyed by embedding model + question text.
type embedCache struct {
	mu      sync.Mutex
	maxSize int
	ttl     time.Duration
	now     func() time.Time
	order   *list.List // front = most recently used
	entries map[string]*list.Element
}

func newEmbedCache(maxSize int, ttl time.Duration) *embedCache {
	return &embedCache{
		maxSize: maxSize,
		ttl:     ttl,
		now:     time.Now,
		order:   list.New(),
		entries: make(map[string]*list.Element, maxSize),
	}
}

func embedCacheKey(model, question string) string {
	return model + "\x00" + question
}

func (c *embedCache) get(model, question string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[embedCacheKey(model, question)]
	if !ok {
		return nil, false
	}
	entry, ok := el.Value.(*embedCacheEntry)
	if !ok {
		return nil, false
	}
	if c.now().After(entry.expiresAt) {
		c.order.Remove(el)
		delete(c.entries, entry.key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return entry.vec, true
}

func (c *embedCache) put(model, question string, vec []float32) {
	if len(vec) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key := embedCacheKey(model, question)
	if el, ok := c.entries[key]; ok {
		if entry, ok := el.Value.(*embedCacheEntry); ok {
			entry.vec = vec
			entry.expiresAt = c.now().Add(c.ttl)
			c.order.MoveToFront(el)
			return
		}
	}
	if c.order.Len() >= c.maxSize {
		if oldest := c.order.Back(); oldest != nil {
			c.order.Remove(oldest)
			if entry, ok := oldest.Value.(*embedCacheEntry); ok {
				delete(c.entries, entry.key)
			}
		}
	}
	c.entries[key] = c.order.PushFront(&embedCacheEntry{
		key:       key,
		vec:       vec,
		expiresAt: c.now().Add(c.ttl),
	})
}
