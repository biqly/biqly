package redisclient

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/rueidis"
	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeycompat"
)

type benchConfig struct {
	addr       string
	password   string
	db         int
	poolSize   int
	keyCount   int
	valueBytes int
	batchSize  int
	prefix     string
}

func loadBenchConfig(tb testing.TB) benchConfig {
	tb.Helper()

	addr := os.Getenv("REDIS_BENCH_ADDR")
	if addr == "" {
		tb.Skip("set REDIS_BENCH_ADDR to run Redis client benchmarks")
	}

	return benchConfig{
		addr:       addr,
		password:   os.Getenv("REDIS_BENCH_PASSWORD"),
		db:         envInt(tb, "REDIS_BENCH_DB", 0),
		poolSize:   envInt(tb, "REDIS_BENCH_POOL_SIZE", 32),
		keyCount:   envInt(tb, "REDIS_BENCH_KEYS", 1000),
		valueBytes: envInt(tb, "REDIS_BENCH_VALUE_BYTES", 256),
		batchSize:  envInt(tb, "REDIS_BENCH_BATCH", 16),
		prefix:     fmt.Sprintf("biqly:redisbench:%d", time.Now().UnixNano()),
	}
}

func envInt(tb testing.TB, key string, fallback int) int {
	tb.Helper()

	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		tb.Fatalf("%s must be a positive integer", key)
	}
	return n
}

func benchKeys(cfg benchConfig) []string {
	keys := make([]string, cfg.keyCount)
	for i := range cfg.keyCount {
		keys[i] = fmt.Sprintf("%s:%d", cfg.prefix, i)
	}
	return keys
}

func benchValue(cfg benchConfig) string {
	return strings.Repeat("x", cfg.valueBytes)
}

func newGoRedisClient(tb testing.TB, cfg benchConfig) *redis.Client {
	tb.Helper()

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.addr,
		Password:     cfg.password,
		DB:           cfg.db,
		PoolSize:     cfg.poolSize,
		MinIdleConns: max(1, cfg.poolSize/4),
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		tb.Fatalf("go-redis ping: %v", err)
	}
	tb.Cleanup(func() { _ = client.Close() })
	return client
}

func newRueidisClient(tb testing.TB, cfg benchConfig) rueidis.Client {
	tb.Helper()

	client, err := rueidis.NewClient(rueidis.ClientOption{
		InitAddress:       []string{cfg.addr},
		Password:          cfg.password,
		SelectDB:          cfg.db,
		PipelineMultiplex: pipelineMultiplex(cfg.poolSize),
		BlockingPoolSize:  cfg.poolSize,
	})
	if err != nil {
		tb.Fatalf("rueidis client: %v", err)
	}
	if err := client.Do(context.Background(), client.B().Ping().Build()).Error(); err != nil {
		tb.Fatalf("rueidis ping: %v", err)
	}
	tb.Cleanup(client.Close)
	return client
}

func pipelineMultiplex(poolSize int) int {
	multiplex := 0
	connections := 1
	for connections < poolSize {
		multiplex++
		connections <<= 1
	}
	return multiplex
}

func seedGoRedis(tb testing.TB, client *redis.Client, keys []string, value string) {
	tb.Helper()

	ctx := context.Background()
	pipe := client.Pipeline()
	for _, key := range keys {
		pipe.Set(ctx, key, value, time.Minute)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		tb.Fatalf("seed go-redis keys: %v", err)
	}
	tb.Cleanup(func() { _ = client.Del(context.Background(), keys...).Err() })
}

func seedRueidis(tb testing.TB, client rueidis.Client, keys []string, value string) {
	tb.Helper()

	ctx := context.Background()
	cmds := make(rueidis.Commands, 0, len(keys))
	for _, key := range keys {
		cmds = append(cmds, client.B().Set().Key(key).Value(value).ExSeconds(60).Build())
	}
	for _, resp := range client.DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			tb.Fatalf("seed rueidis keys: %v", err)
		}
	}
	tb.Cleanup(func() {
		cmds := make(rueidis.Commands, 0, len(keys))
		for _, key := range keys {
			cmds = append(cmds, client.B().Del().Key(key).Build())
		}
		_ = client.DoMulti(context.Background(), cmds...)
	})
}

func measureP99(b *testing.B, run func(context.Context, int) error) {
	b.Helper()

	ctx := context.Background()
	samples := make([]int64, 0, 10_000)
	i := 0
	b.ResetTimer()
	for b.Loop() {
		start := time.Now()
		if err := run(ctx, i); err != nil {
			b.Fatal(err)
		}
		if len(samples) < cap(samples) {
			samples = append(samples, time.Since(start).Nanoseconds())
		}
		i++
	}
	b.StopTimer()

	if len(samples) == 0 {
		return
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	idx := min(len(samples)-1, (len(samples)*99)/100)
	b.ReportMetric(float64(samples[idx]), "p99_ns/op")
}

func BenchmarkGoRedisGET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newGoRedisClient(b, cfg)
	keys := benchKeys(cfg)
	seedGoRedis(b, client, keys, benchValue(cfg))

	measureP99(b, func(ctx context.Context, i int) error {
		return client.Get(ctx, keys[i%len(keys)]).Err()
	})
}

func BenchmarkRueidisGET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newRueidisClient(b, cfg)
	keys := benchKeys(cfg)
	seedRueidis(b, client, keys, benchValue(cfg))

	measureP99(b, func(ctx context.Context, i int) error {
		return client.Do(ctx, client.B().Get().Key(keys[i%len(keys)]).Build()).Error()
	})
}

func BenchmarkGoRedisSET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newGoRedisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedGoRedis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		return client.Set(ctx, keys[i%len(keys)], value, time.Minute).Err()
	})
}

func BenchmarkRueidisSET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newRueidisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedRueidis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		return client.Do(ctx, client.B().Set().Key(keys[i%len(keys)]).Value(value).ExSeconds(60).Build()).Error()
	})
}

func BenchmarkGoRedisMSET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newGoRedisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedGoRedis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		values := make([]any, 0, cfg.batchSize*2)
		for offset := range cfg.batchSize {
			values = append(values, keys[(i+offset)%len(keys)], value)
		}
		return client.MSet(ctx, values...).Err()
	})
}

func BenchmarkRueidisMSET(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newRueidisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedRueidis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		cmd := client.B().Mset().KeyValue()
		for offset := range cfg.batchSize {
			cmd = cmd.KeyValue(keys[(i+offset)%len(keys)], value)
		}
		return client.Do(ctx, cmd.Build()).Error()
	})
}

func BenchmarkGoRedisPipelineSetGet(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newGoRedisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedGoRedis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		pipe := client.Pipeline()
		for offset := range cfg.batchSize {
			key := keys[(i+offset)%len(keys)]
			pipe.Set(ctx, key, value, time.Minute)
			pipe.Get(ctx, key)
		}
		_, err := pipe.Exec(ctx)
		return err
	})
}

func BenchmarkRueidisDoMultiSetGet(b *testing.B) {
	cfg := loadBenchConfig(b)
	client := newRueidisClient(b, cfg)
	keys := benchKeys(cfg)
	value := benchValue(cfg)
	seedRueidis(b, client, keys, value)

	measureP99(b, func(ctx context.Context, i int) error {
		cmds := make(rueidis.Commands, 0, cfg.batchSize*2)
		for offset := range cfg.batchSize {
			key := keys[(i+offset)%len(keys)]
			cmds = append(cmds,
				client.B().Set().Key(key).Value(value).ExSeconds(60).Build(),
				client.B().Get().Key(key).Build(),
			)
		}
		for _, resp := range client.DoMulti(ctx, cmds...) {
			if err := resp.Error(); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestValkeyCompatAPISurface(t *testing.T) {
	cfg := loadBenchConfig(t)
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.addr},
		Password:    cfg.password,
		SelectDB:    cfg.db,
	})
	if err != nil {
		t.Fatalf("valkey client: %v", err)
	}
	t.Cleanup(client.Close)

	adapter := valkeycompat.NewAdapter(client)
	ctx := context.Background()
	key := cfg.prefix + ":valkeycompat"
	t.Cleanup(func() { _ = client.Do(context.Background(), client.B().Del().Key(key).Build()).Error() })

	if err := adapter.Set(ctx, key, "value", time.Minute).Err(); err != nil {
		t.Fatalf("valkey compat set: %v", err)
	}
	if got, err := adapter.Get(ctx, key).Result(); err != nil || got != "value" {
		t.Fatalf("valkey compat get = %q, %v", got, err)
	}
	_ = adapter.Cache(time.Minute)
	_, _ = adapter.Pipelined(ctx, func(pipe valkeycompat.Pipeliner) error {
		pipe.Set(ctx, key, "value", time.Minute)
		pipe.Get(ctx, key)
		return nil
	})
}
