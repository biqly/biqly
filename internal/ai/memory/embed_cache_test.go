package memory

import (
	"context"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingEmbedder struct {
	model string
	calls *int
}

func (c countingEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	*c.calls++
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, 0}
	}
	return out, nil
}

func (c countingEmbedder) Model() string { return c.model }

func TestRecallFewShotUsesEmbedCache(t *testing.T) {
	calls := 0
	embedder := countingEmbedder{model: "cache-test-model", calls: &calls}
	candidates := []metadata.ConfirmedQueryRow{
		{NLQuery: "orders by region", SQLQuery: `{}`, QuestionEmbedding: []float32{1, 0}},
	}
	question := "embed cache unique question"

	out, _ := RecallFewShot(context.Background(), embedder, candidates, question, 1)
	require.Len(t, out, 1)
	require.Equal(t, 1, calls)

	out, _ = RecallFewShot(context.Background(), embedder, candidates, question, 1)
	require.Len(t, out, 1)
	assert.Equal(t, 1, calls, "second recall of the same question must not re-embed")
}

func TestEmbedCacheTTLExpiry(t *testing.T) {
	cache := newEmbedCache(8, time.Minute)
	current := time.Unix(1_000_000, 0)
	cache.now = func() time.Time { return current }

	cache.put("m", "q", []float32{1})
	if _, ok := cache.get("m", "q"); !ok {
		t.Fatal("expected cache hit within TTL")
	}

	current = current.Add(time.Minute + time.Second)
	if _, ok := cache.get("m", "q"); ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
	if _, ok := cache.entries[embedCacheKey("m", "q")]; ok {
		t.Fatal("expired entry must be removed from the cache")
	}
}

func TestEmbedCacheModelScoped(t *testing.T) {
	cache := newEmbedCache(8, time.Minute)
	cache.put("model-a", "q", []float32{1})
	if _, ok := cache.get("model-b", "q"); ok {
		t.Fatal("embedding from another model must not be served")
	}
}

func TestEmbedCacheEvictsOldest(t *testing.T) {
	cache := newEmbedCache(2, time.Minute)
	cache.put("m", "q1", []float32{1})
	cache.put("m", "q2", []float32{2})
	// Touch q1 so q2 becomes the least recently used entry.
	if _, ok := cache.get("m", "q1"); !ok {
		t.Fatal("expected q1 hit")
	}
	cache.put("m", "q3", []float32{3})

	if _, ok := cache.get("m", "q2"); ok {
		t.Fatal("least recently used entry q2 should have been evicted")
	}
	if _, ok := cache.get("m", "q1"); !ok {
		t.Fatal("recently used entry q1 should survive eviction")
	}
	if _, ok := cache.get("m", "q3"); !ok {
		t.Fatal("newly inserted entry q3 should be present")
	}
}

func TestEmbedCacheIgnoresEmptyVector(t *testing.T) {
	cache := newEmbedCache(2, time.Minute)
	cache.put("m", "q", nil)
	if _, ok := cache.get("m", "q"); ok {
		t.Fatal("empty vectors must not be cached")
	}
}
