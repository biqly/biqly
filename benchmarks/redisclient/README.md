# Redis Client Benchmark

Runnable benchmark harness for the `tasks/todo.md` Redis client migration evaluation.

It compares the currently used `go-redis/v9` client with `rueidis` for this repo's common Redis shapes:

- single `GET`
- single `SET`
- batched `MSET`
- pipelined `SET`/`GET` batches

`valkey-go` is included as a compile-time API compatibility check because its compat adapter is close to `go-redis`, but production migration should still wait for measured benchmark wins.

## Run

Start Redis, Dragonfly, or Valkey locally, then:

```sh
cd benchmarks/redisclient
REDIS_BENCH_ADDR=127.0.0.1:6379 go test -run TestValkeyCompatAPISurface -bench . -benchtime=10s -count=5
```

Optional environment:

- `REDIS_BENCH_PASSWORD`
- `REDIS_BENCH_DB` (default `0`)
- `REDIS_BENCH_POOL_SIZE` (default `32`; mapped to `go-redis` `PoolSize` and the nearest rueidis `PipelineMultiplex` connection count)
- `REDIS_BENCH_KEYS` (default `1000`)
- `REDIS_BENCH_VALUE_BYTES` (default `256`)
- `REDIS_BENCH_BATCH` (default `16`)

The benchmark reports Go's normal `ns/op` plus a bounded `p99_ns/op` sample for latency comparison. Treat the result as deployment-specific; run it against the same Dragonfly/Redis class and network path used by staging before choosing a migration.
