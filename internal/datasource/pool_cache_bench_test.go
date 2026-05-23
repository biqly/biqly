package datasource

import (
	"context"
	"testing"
)

// BenchmarkPoolCacheGet measures the steady-state Get cost when the pool
// is already cached (production hot path: same datasource queried over
// and over). Should be on the order of a map lookup + a mutex round-trip.
//
// Run:
//
//	go test -bench=BenchmarkPool -benchmem ./internal/datasource/...
func BenchmarkPoolCacheGet(b *testing.B) {
	cache := NewPoolCache()
	d := &stubDriver{}

	// Prime the cache.
	if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-1"); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = cache.Close() })

	b.ResetTimer()
	for range b.N {
		if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-1"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFreshOpenPerCall simulates the pre-cache behavior: each
// "query" calls driver.Open and immediately closes the resulting pool.
// The delta against BenchmarkPoolCacheGet shows what the cache saves per
// query execution.
func BenchmarkFreshOpenPerCall(b *testing.B) {
	d := &stubDriver{}
	b.ResetTimer()
	for range b.N {
		db, err := d.Open(context.Background(), "dsn-1")
		if err != nil {
			b.Fatal(err)
		}
		_ = db.Close()
	}
}

// BenchmarkPoolCacheGetParallel covers concurrent callers competing for
// the same cached handle. The single Mutex in PoolCache should not
// saturate at typical web concurrency levels.
func BenchmarkPoolCacheGetParallel(b *testing.B) {
	cache := NewPoolCache()
	d := &stubDriver{}
	if _, err := cache.Get(context.Background(), d, "ds-1", "dsn-1"); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = cache.Close() })

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			if _, err := cache.Get(ctx, d, "ds-1", "dsn-1"); err != nil {
				b.Fatal(err)
			}
		}
	})
}
