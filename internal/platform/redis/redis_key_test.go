package rediscache

import (
	"testing"

	"github.com/biqly/biqly/internal/query"
)

// TestKeyDeterministicWithMapValue verifies the cache key stays stable when
// LogicalQuery contains map-valued filter values. The earlier implementation
// used fmt.Sprintf("%+v", …) which iterated map keys in random order — two
// identical queries could hash to different keys, breaking cache hits.
func TestKeyDeterministicWithMapValue(t *testing.T) {
	cache := NewCacheDisabled()

	mkLQ := func() query.LogicalQuery {
		return query.LogicalQuery{
			Version:      "v1",
			DatasourceID: "ds-1",
			ModelID:      "m-1",
			Select:       []query.SelectItem{{Type: "metric", Name: "count"}},
			// Map-valued filter — exact case the old %+v formatter would
			// shuffle between runs.
			Filters: []query.Filter{
				{
					Field:    "metadata",
					Operator: "eq",
					Value: map[string]any{
						"a": 1,
						"b": 2,
						"c": 3,
						"d": 4,
						"e": 5,
					},
				},
			},
			Limit: 100,
		}
	}

	// Re-build the LogicalQuery each iteration so internal map state is fresh.
	first := cache.Key("ds-1", "m-1", mkLQ(), "tenant-a")
	for i := 0; i < 50; i++ {
		got := cache.Key("ds-1", "m-1", mkLQ(), "tenant-a")
		if got != first {
			t.Fatalf("key drifted on iteration %d: %s vs %s", i, got, first)
		}
	}
}

// TestKeyDistinguishesUserScope confirms two requests for the same query
// from different users hash to different cache keys (so user A doesn't see
// rows row-level-filtered for user B).
func TestKeyDistinguishesUserScope(t *testing.T) {
	cache := NewCacheDisabled()
	lq := query.LogicalQuery{
		Version:      "v1",
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Select:       []query.SelectItem{{Type: "metric", Name: "count"}},
	}
	a := cache.Key("ds-1", "m-1", lq, "tenant-a")
	b := cache.Key("ds-1", "m-1", lq, "tenant-b")
	if a == b {
		t.Fatalf("cache keys must differ across user scopes, both = %s", a)
	}
}

// TestKeyStableAcrossEquivalentSelects covers the inverse: two structurally
// equal LogicalQuery values built independently still hash to the same key.
func TestKeyStableAcrossEquivalentSelects(t *testing.T) {
	cache := NewCacheDisabled()
	a := query.LogicalQuery{
		Version:      "v1",
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Select:       []query.SelectItem{{Type: "metric", Name: "count"}},
		OrderBy:      []query.OrderBy{{Field: "count", Direction: "desc"}},
		Limit:        50,
	}
	b := query.LogicalQuery{
		Version:      "v1",
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Select:       []query.SelectItem{{Type: "metric", Name: "count"}},
		OrderBy:      []query.OrderBy{{Field: "count", Direction: "desc"}},
		Limit:        50,
	}
	if cache.Key("ds-1", "m-1", a, "u") != cache.Key("ds-1", "m-1", b, "u") {
		t.Fatal("structurally equal queries should hash equally")
	}
}
