# Biqly Backend Code Review TODO

## Critical - Must Fix Before Production

- [x] `security/readonly.go:52` — ReadOnlyChecker semicolon bypass: `SELECT 1; DROP TABLE x;` passes validation. Count statement fragments instead of suffix check.
- [x] `security/row_injection.go:58-60` — Unsupported row filter operators silently fall back to `eq`. A `neq` filter becomes `column = value`, exposing restricted rows. Return error for unknown operators.
- [x] `security/readonly.go:41` — CTE queries (`WITH ... SELECT`) rejected because they don't start with `SELECT`. Allow `WITH` and `EXPLAIN` prefixes.
- [x] `security/readonly.go:15-19` — Missing dangerous functions: `xp_cmdshell`, `OPENROWSET`, `BULK INSERT`, `pg_read_file`, `LOAD_FILE`, `dblink`, `lo_import`.
- [x] `http/router.go` — No authentication middleware on `/api/*` routes. All endpoints are unauthenticated.
- [x] `http/router.go:36-43` — CORS `AllowedOrigins: ["https://*", "http://*"]` with `AllowCredentials: true`. Restrict to known frontend domains.
- [x] `cmd/api/main.go:46` — DSN with credentials logged in plaintext. Redact or hash before logging.
- [x] `security/permissions.go:30-31` — Nil policy grants full access. Fail closed instead of default-open.

## Performance - Should Fix Soon

- [x] `core/query_service.go:130-134` — New `*sql.DB` pool opened and closed per query execution. Implement pool cache keyed by datasource ID.
- [x] `http/metrics.go:16,203-205` — Single global `sync.Mutex` for all metrics, held during Prometheus serialization. Replace with `atomic.Int64` counters.
- [x] `http/metrics.go:214` — `runtime.ReadMemStats` forces STW GC pause on every `/metrics` scrape. Remove forced GC.
- [x] `ai/service.go:446-467` — Multi-candidate LLM generation runs N requests serially. Use `errgroup` for parallelism.
- [x] `ai/prompt.go:250` — Go `text/template` parsed from scratch on every prompt build. Parse once and cache.
- [x] `semantic/repository.go:316-338` — `GetFullModel` executes 4 sequential queries (hot path). Use JOINs or `errgroup`.
- [x] `ai/table_router.go:229-240` — `ListTables`, `ListColumns`, `ListRelations` called sequentially. Run concurrently with `errgroup`.
- [x] `ai/prompt.go:104-206` — 11 `bytes.Buffer` allocations per `Build` call. Use existing `promptBuilderPool` or single buffer.
- [x] `ai/table_router.go:1805-1808` — `isNumericType` allocates fresh slice per call. Use package-level map.

## Database

- [x] `app/dependencies.go:87-98` — Metadata DB pool missing `ConnMaxLifetime` and `ConnMaxIdleTime`. Connections never recycled. Add `db.SetConnMaxLifetime(30*time.Minute)` and `db.SetConnMaxIdleTime(10*time.Minute)`.
- [x] `datasource/sql_pool.go:53-54` — External datasource pools missing `ConnMaxLifetime`/`ConnMaxIdleTime`.
- [x] `app/dependencies.go` — `internal/platform/db/pool.go` provides properly configured pool but is dead code. Use it or delete it.
- [x] `metadata/embeddings.go:148-188` — `upsertEntityEmbedding` does SELECT + UPDATE without transaction. Race condition on concurrent writes.
- [x] `semantic/repository.go:44,51,...` — `id::text = $1` forces text cast on UUID columns, bypasses index. Pass UUID directly.
- [x] `metadata/repository.go:730-733` — `ILIKE '%' || term || '%'` full table scan. Consider pg_trgm GIN index for large metadata tables.
- [x] `metadata/repository.go:136` — `DeleteDatasource` does not cascade-delete related records. Orphaned rows accumulate.
- [x] `http/handlers/metadata.go:113-124` — Table description + label updated in separate transactions. Partial success possible.

## Architecture

- [ ] `http/router.go:30` — Global `AIRequestTimeout` (~630s) applied to all routes including `/health` and `/ready`. Non-AI routes should have shorter timeout.
- [ ] `app/dependencies.go:255-260` — `Close()` only closes metadata DB. AI provider HTTP clients, embedder, datasource pools never closed.
- [ ] `app/dependencies.go` — `NewDependencies` is a monolithic 200-line constructor. Hard to test without real Postgres. Accept interfaces for key deps.
- [ ] `compiler.go:74-119` — `CompileWithPermissions` does regex surgery on assembled SQL to inject WHERE. Can match WHERE inside CTEs. Inject filters during compilation instead.
- [ ] `calendar_grain_filter.go:194` — `dateTruncCompareExpr` hardcodes `::timestamptz` PostgreSQL cast. Breaks for MySQL, SQL Server, ClickHouse.

## Security (Non-Critical)

- [ ] `http/metrics.go` — `/metrics` endpoint has no auth. Exposes operational data (query volumes, error rates).
- [ ] `handlers/helpers.go:78` — `decodeJSON` has no request body size limit. Client can send arbitrarily large payloads.
- [ ] `readiness.go:62` — Internal error details (hostnames, ports) exposed in readiness JSON response.
- [ ] `readiness.go:72` — `http.DefaultClient` with no redirect limit on upstream health checks. SSRF risk.
- [ ] `embed_metadata.go:222-264` — All table/column metadata sent to external embedding API without opt-out for sensitive schemas.
- [ ] `logger/logger.go:36` — Log file permissions `0644` (world-readable). Logs may contain sensitive metadata.
- [ ] `config.go:181` — Default DSN with `sslmode=disable`. Should not ship to production.
- [ ] `handlers/internal_auth_middleware.go:43` — Falls back to raw `Authorization` header without `Bearer` prefix.
- [ ] `semantic/publish.go:326-329` — Calculated expression DML detection uses string matching, easily bypassable. Needs AST-based validation.
- [ ] `http/handlers/ai_jobs.go:203-237` — `AdminListStale` / `AdminCancelAllStale` have no admin auth. Any client can cancel all jobs.

## Refactoring

- [ ] `http/ai_proxy.go` + `http/query_proxy.go` — 95% identical code. Extract `newUpstreamProxy(prefix, targetURL, serviceName)` factory.
- [ ] `http/handlers/metadata.go` — All error paths use `writeError` (no logging). Should use `writeInternalError` for 500+ errors.
- [ ] `http/handlers/query.go:104` — History listing failure uses `writeError` with no logging. Use `writeInternalError`.
- [ ] `ai/table_router.go` — `tokenSet(question)` recomputed in multiple functions. Compute once and pass as parameter.
- [ ] `ai/table_router.go:644-817` — `appendEntityResolverTables` is 173 lines with nested BFS. Consider extraction into sub-functions.

## Testing

- [x] Test: ReadOnlyChecker rejects `SELECT 1; DROP TABLE x;`
- [x] Test: ReadOnlyChecker accepts `WITH cte AS (...) SELECT ...`
- [x] Test: Row filter unsupported operators return errors, not wrong SQL
- [x] Test: Concurrent embedding writes on same entity  <!-- fixed via SELECT FOR UPDATE + tx; not unit-tested (needs live PG) -->

- [ ] Test: Metrics mutex contention under concurrent load (benchmark)
- [ ] Test: Template parsing overhead with cached vs uncached (benchmark)
- [ ] Test: Cache key determinism for `redis.Key` with map-containing structs
- [x] Test: Permission nil-safety (nil policy should deny access after fix)
- [ ] Test: `dateTruncCompareExpr` with MySQL/SQLServer/ClickHouse dialects
- [ ] Test: Admin job endpoints reject unauthenticated requests (after auth added)
- [ ] Test: Connection pool open/close overhead vs cached pool (benchmark)

## Optional / Future

- [ ] `config.go:88-167` — `AIConfig` has ~80 fields. Extract sub-configs (`AIEmbeddingConfig`, `AITranslationConfig`, `AIRoutingConfig`).
- [ ] `config.go:444-453` — `getEnvAsInt` / `getEnvAsFloat` silently swallow parse errors. Log warning on failure.
- [ ] `ai/retry_helpers.go:22` — No jitter in exponential backoff. Retry storms possible under load.
- [ ] `platform/redis/redis.go:54` — `fmt.Sprintf("%+v", data)` for cache key is fragile with maps (non-deterministic ordering).
- [ ] `query/executor.go:80-82` — Row limit truncation is silent. Add `Truncated bool` to result so caller can distinguish.
- [ ] `core/query_service.go:93-94` vs `104-105` — `RepairMisnamedCalendarGrainDimensions` and `EnsureGroupBySelected` called twice on `Compile -> CompileWithContext` path.
- [ ] Log rotation for file-based logging (`platform/logger/logger.go:36`).
