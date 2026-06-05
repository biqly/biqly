# Lessons Learned

Patterns, corrections, and decisions captured from implementation work. Review at session start.

---

## Code Patterns

### Duplicate Code Extraction

When you see the same logic repeated in 2+ handlers, extract a shared helper immediately. Repeated patterns found and fixed:

- **Log context**: `appendRequestLogArgs` — extracted from `writeServiceError` / `writeInternalAPIError` so `request_id`, `user_id`, `workspace_id` are appended once.
- **AI datasource process options**: `localRunProcessOptions` — extracted from sync `processAndObserve` and async `executeAIQueryPhase` to build dry-run, dialect, few-shot, sample-data options once.
- **AI history permission**: `canViewAIHistoryDetails` — extracted from `AIHistory`, `AIHistoryDetail`, `GetAIUsageBreakdown` to check view-detail permission once.
- **PII masking lookup**: Consolidated split-map (`piiColumnsByTable`, `piiColumnsByName`) into single `ColumnInfo map[string]PIIColumnInfo` with legacy fallback.

**Rule**: If you copy-paste more than 5 lines between handlers, stop and extract.

### Dead Code Gate Before Commit

Run `deadcode -test $(go list ./... | grep -v '/frontend')` before commits that touch Go code. The `/frontend` exclusion is required because `frontend/node_modules` can include third-party Go packages that are not part of this repository's Go surface.

**Rule**: Treat deadcode findings as blockers to triage before commit, but do not blindly delete every reported function. Check exported APIs, alternate build tags, reflection/linkname paths, generated code, tests, and planned integration points before removal.

**Default cleanup strategy**: Clean up only genuinely dead internal code under `internal/` that is unused in production and tests. Preserve public SDK APIs under `pkg/` and test-only helpers unless a focused review proves they are obsolete.

### Go Formatting Gate Before Commit

Run `gofmt -w` on every touched `.go` file before linting or testing. Formatting drift is a blocker, even when the code compiles.

### Go Min/Max Modernization

Use Go's built-in `min` / `max` for simple clamps and two-value comparisons, such as `chunk = min(chunk, len(encoded))`. Keep explicit `if` statements when branches have side effects, extra logic, or clearer domain meaning.

### Fail-Open at Read Time, Fail-Closed at Write Time

Expression AST parsing follows this pattern:

- **Read path** (`GetFullModel`): parse errors log a warning and leave AST nil. Existing string expression still works.
- **Write path** (publish, API create/update): invalid AST or unparsable string returns `400 Bad Request` immediately.

**Rule**: Backward compatibility = fail-open reads, fail-closed writes. Never break existing data on read.

### Dual-Format Storage Migration

When adding a new structured field alongside a legacy string field:

1. Add new column with `IF NOT EXISTS` (Go model may already have the field).
2. Repository scan prefers new format when present; falls back to legacy.
3. Add load-time hydration to parse legacy → new format on read.
4. Backfill existing rows in a separate one-shot migration tool (not SQL-only).
5. Keep legacy field populated during transition.

Applied in `semantic_dimensions.calculated_expr_json` / `semantic_metrics.expr_json`.

### Deterministic Hash-Based Traffic Split

For A/B tests, use `hash(userID + experimentID) % 100` → cumulative bucket selection. Properties:

- Same user always sees same variant.
- No database lookup needed per request.
- Add in-memory cache with 30s TTL for running experiments.

---

## Performance Decisions

### Benchmark Before Optimizing — No Speculative Changes

Multiple items closed as explicit no-code decisions after benchmarking:

| Item | Finding | Decision |
| ------ | --------- | ---------- |
| `strings.Builder` pool for `stripSQLLiteralsAndComments` | Pooled builder slower (137 vs 123 ns/op) | Keep as-is |
| `filterHandler` generics refactor | Typed helpers same alloc profile, no stable improvement | Keep as-is |
| `args []any` pool in compiler | Pool overhead > one compile-time allocation | Keep as-is |
| `dimMap` pool in compiler | Small per-row-filter map, scoped to function | Keep as-is |
| `columns []ResultColumn` pool in executor | Escapes into returned `Result`, pooling unsafe | Keep as-is |
| Auth JWT `map[string]any` | Already on typed `JWTClaims` struct | No-op, stale item |
| `map[string]any` in provider store | Already on typed provider config paths | No-op, stale item |

**Rule**: Never refactor for performance without benchmark evidence. `go test -bench -benchmem -count=5` is the standard. If pooled/generic variant is not measurably faster, keep the simpler code. New or touched Go benchmarks should use `b.Loop()` instead of manual `for range b.N` / `for i := 0; i < b.N; i++` loops.

### Escape Analysis Workflow

1. `go build -gcflags='-m -m' ./internal/query` to find escapes.
2. If value escapes into return value → pooling is unsafe (the caller owns it).
3. If value is stack-local → pooling *might* help, but benchmark first.
4. `strings.Builder` with `Grow(len(sql))` already inlines through `abi.NoEscape`.

**Rule**: Escape analysis tells you *where* allocations happen. It doesn't tell you *whether* pooling is faster.

### Pre-Allocate Known-Size Slices

Applied in PII masking:

- `ColumnInfo map[string]PIIColumnInfo` single lookup instead of split-map double lookup.
- Removed `fmt.Sprintf` predicate builders in `compiler_filter.go` and `row_injection.go`.

---

## Architecture Decisions

### Expression AST: Sealed Interface with JSON Discriminator

`pkg/semantic/expr.go` uses:

- Sealed interface pattern: `ExprNode` interface with unexported method.
- JSON `"type"` discriminator for marshal/unmarshal.
- `AllowedFunctions` whitelist with arity values.
- Max depth 10 for AST validation.

This prevents external packages from implementing new node types without updating the whitelist.

### PII Masking: Resolve Strategy → Access Level Before Compilation

Previous bug: strategy ("full"/"masked"/"hidden") and access ("raw"/"masked"/"hidden") were checked independently, causing semantic conflicts (e.g., strategy="full" + access="masked" produced wrong SQL).

Fix: Compiler resolves `(strategy, access)` → single effective access level before any SQL generation. Hidden/full-masked columns rejected in filters, GROUP BY, ORDER BY — no constant predicates.

### Cross-Dialect SQL Testing

Golden test pattern:

1. Single table-driven test `TestGoldenAcrossDialects` with 12 scenarios × 3 dialects = 36 targets.
2. `UPDATE_GOLDEN=true` to auto-generate missing `.sql` files.
3. Each dialect has its own `testdata/sql/{dialect}/` directory.

---

## Workflow & Process

### Plan Verification Loop

1. Write plan to `tasks/todo.md` with `[]` items.
2. Run RED test first (failing test against non-existent API).
3. Implement minimum code to make GREEN.
4. Run focused tests + `git diff --check`.
5. Document results in `### Results` section.

This was consistently followed across EXPR_AST (9 phases), GO_PERF (6 slices), PII (2 slices), AB_TEST (3 phases), and duplicate cleanup (3 refactorings).

### Stale Item Cleanup

Close items explicitly with rationale when investigation shows no current issue:

- "Auth claims `map[string]any`" → already on typed struct.
- "Provider store `map[string]any`" → already on typed paths.
- "i18n/mail `map[string]any`" → accepted cold-path usage.

**Rule**: Don't leave stale items open. Close with a one-line decision so the next person doesn't re-investigate.

### Migration Naming Convention

```text
migrations/
  040a_add_expression_ast_json.up.sql
  040b_add_expression_ast_json.down.sql
  042a_add_ab_experiments.up.sql
  042b_add_ab_experiments.down.sql
```

`NNNa` = up, `NNNb` = down. Paired. Sequential numbering.

### Local DB Dependency

Docker daemon must be running for migration testing. When Docker is unavailable:

- Write the migration file.
- Write a migration-contract test in `cmd/migrate/`.
- Mark as "blocked on Docker" and verify in CI.

---

## Frontend Conventions

### ExpressionBuilder Pattern

- `ExpressionBuilder.tsx` supports text mode (raw input + backend compile) and visual mode (recursive AST builder).
- Whitelisted functions rendered from backend `AllowedFunctions`.
- CSS in `frontend/src/styles/expressionBuilder.css` (BEM naming).
- No Tailwind — vanilla CSS only.

### Component Testing

- Vitest for unit/component tests.
- Pure-logic tests for non-trivial frontend behavior (e.g., PII reviewed-row strategy saves).
- `npm --prefix frontend run build` must pass before commit.
- Lint threshold: `--max-warnings 1500` (existing warnings are known).

---

## Anti-Patterns to Avoid

1. **Don't add `fmt.Sprintf` in hot-path predicate builders** — use direct string concatenation or `strings.Builder`.
2. **Don't pool values that escape into return values** — unsafe, caller may hold reference after pool recycle.
3. **Don't use `jwt.MapClaims` when typed claims structs exist** — already migrated.
4. **Don't break backward compatibility on read paths** — fail-open with warning log.
5. **Don't leave duplicated handler logic** — extract shared helper on first sight.
6. **Don't optimize without benchmark numbers** — every "maybe faster" hypothesis tested was either same speed or slower.
7. **Don't commit with lint errors** — `make lint-go` / `make lint-frontend` / `make test-go` must all pass cleanly (zero errors).
8. **Don't use Tailwind CSS** — vanilla CSS with BEM naming in `frontend/src/styles/`.
