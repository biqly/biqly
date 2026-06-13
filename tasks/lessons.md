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

### Frontend Formatting Gate Before Commit

Run `npx prettier --check` (or `npm --prefix frontend run format:check`) on every touched frontend file, alongside `eslint` and `tsc`. ESLint + tsc passing does NOT catch Prettier drift — CI runs `format:check` as a separate gate (part of `make check-frontend`) that fails the build independently.

**Why it bites**: editing a file (e.g. adding an import line) can reflow neighboring code into a shape Prettier considers non-canonical (a multi-line import that now fits on one line, etc.). The functional edit is fine and lint/tsc stay green, but `format:check` flags the file. Real example: adding a `useAuth` import to `useAIJobs.tsx` left a `../types/ai` type import in multi-line form that Prettier wanted collapsed.

**Rule**: After editing any frontend file, run `prettier --write` (or `--check`) on it before commit. Do not rely on eslint/tsc to surface formatting issues.

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

### AI Eval: Two Tiers

- **PR / pre-commit (stub)**: `make eval-regression` — deterministic stub provider, 1.00 pass threshold; catches harness/compiler regressions only. Not part of `make precommit`; run when eval code or golden cases change.
- **Nightly (live LLM)**: `make eval-live` / `.github/workflows/eval-nightly.yml` — real provider + baseline drift report; requires API secrets. Never gate local commits on this.

### Stale Item Cleanup

Close items explicitly with rationale when investigation shows no current issue:

- "Auth claims `map[string]any`" → already on typed struct.
- "Provider store `map[string]any`" → already on typed paths.
- "i18n/mail `map[string]any`" → accepted cold-path usage.

**Rule**: Don't leave stale items open. Close with a one-line decision so the next person doesn't re-investigate.

### Verify-Before-Build (todo.md drifts behind code)

`tasks/todo.md` lags the codebase. Multiple "X is missing / not wired" items turned out to be already implemented and shipped:

- "enrich-context frontend UI yok" → already lived in `GlossaryEnrichPanel.tsx` + `Glossary.tsx` (commit `64ef4642`).
- "publish-time confirmed-query deaktivasyon mekanizması yok" → `DeactivateConfirmedQueriesExceptHash` already called from `(*SemanticHandler).PublishModel`.

**Rule**: Before implementing ANY todo.md item, confirm the current state first — `gograph_query`/`gograph_context` for Go symbols, `rg` for frontend. If it already exists, surface that and pivot to improve/harden/test instead of rebuilding from scratch. Never trust a todo's "yok / missing" framing at face value.

### Migration Naming Convention

```text
migrations/
  040a_add_expression_ast_json.up.sql
  040b_add_expression_ast_json.down.sql
  042a_add_ab_experiments.up.sql
  042b_add_ab_experiments.down.sql
```

`NNNa` = up, `NNNb` = down. Paired. Sequential numbering.

### Configuration Documentation Alignment

When adding or modifying any `BI_*` environment variables in `internal/config/config.go`, you must immediately update `docs/configuration.md` to document the key, its default value, Helm status, runtime override status, used-in path, and notes.

**Rule**: The Go test `TestConfigDocSync` in `internal/config` automatically parses `config.go` and asserts that every environment variable key is documented in `docs/configuration.md`. Ensure this test passes (`go test ./internal/config/...`) before staging or committing any changes.

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
- Styling: Tailwind util'leri + modül sabitleri (`exprAst*Class`, `exprModeToggleClass`) in `ExpressionBuilder.tsx` (2026-06-13; `expressionBuilder.css` silindi).

### Tailwind CSS Integration (2026-06-13)

Kademeli vanilla CSS → Tailwind v4 migrasyonu. Detaylı checklist: `tasks/todo.md` → **Tailwind CSS Entegrasyonu**.

**Kurulum:** `tailwindcss` + `@tailwindcss/vite` plugin; `frontend/src/index.css` başında `@import 'tailwindcss';`.

**Runtime tema (kritik):** Tokenlar `:root` / `[data-theme]` ile runtime'da değişir. `@theme inline { --color-card: var(--bg-card); ... }` kullan — util'ler `var(--token)` emit eder. **Hardcode renkli Tailwind util (`bg-zinc-900` vb.) kullanma**; light/dark kırılır.

**Tercih edilen util isimleri:** `bg-canvas`, `bg-card`, `text-foreground`, `text-foreground-muted`, `border-border`, `text-accent`, `shadow-card`, `font-mono`. Off-grid spacing için arbitrary: `p-[0.35rem]`, `text-[0.8125rem]`.

**Yeni / taşınan component'ler:** Tailwind util'leri tercih et. Mevcut `.btn`/`.card`/`.input` global sınıfları `index.css`'te kalır (170+ `.btn` referansı); yeni kodda `ui/Button`, `ui/FormField` kullan.

**Silinen CSS dosyası:** Component'teki `import './x.css'` satırını da kaldır — aksi halde build kırılır (`Modeling.tsx` + `drift.css` örneği).

**Doğrulama:** Her grup sonrası `make check-frontend` + görsel light/dark kontrol.

### Shared List/Table Building Blocks (use in NEW code — do not hand-roll)

- Paginated server list state: `usePaginatedList` (`frontend/src/hooks/usePaginatedList.ts`); client-side slicing: `useClientPagination`. Never hand-roll `currentPage/totalItems/loading/error` blocks.
- Loading/error/empty composition: `ui/DataState`; tables: `ui/DataTable` + `ColumnDef`; sorting: `useSortState` + `utils/sorting`; selection sets: `useRowSelection`.
- Buttons: `ui/Button` (emits `btn btn-*` classes) instead of raw `className="btn btn-…"` strings. Text inputs with labels: `ui/FormField`. (`admin-btn-*` family is separate, not covered.)
- Locale-aware dates: `formatDateTime`/`formatDateOnly` from `utils/formatters` — never bare `toLocaleString()` without a language tag.
- New components take NO `t`/`locale` props — call `useT()`/`useLocale()` directly.

### Component Testing

- Vitest for unit/component tests.
- Pure-logic tests for non-trivial frontend behavior (e.g., PII reviewed-row strategy saves).
- `npm --prefix frontend run build` must pass before commit.
- Lint threshold: `--max-warnings 1500` (existing warnings are known).

---

## Lint-Enforced Coding Rules

These rules are enforced by `make lint-go` (golangci-lint via `.golangci.yml`) and `make lint-frontend` (ESLint via `frontend/eslint.config.js`). Code must comply at write-time — do not rely on CI to catch violations. The rules below explain *why* each linter is enabled and *how* to write code that passes.

### Go — golangci-lint Rules (`.golangci.yml`)

#### Error Handling

- **errcheck** (`check-type-assertions: true`): never silently discard error returns. Every `func() (..., error)` call must handle the error. Type assertions like `v.(string)` must be checked with the two-value form `v, ok := x.(string)` — unchecked assertions panic at runtime.
- **errorlint**: use `errors.Is()` for sentinel comparison, `errors.As()` / `errors.AsType[T]()` for type matching, and `%w` verb in `fmt.Errorf` for wrapping. Never compare errors with `==` or `!=` — wrapped errors break equality.
- **nilerr**: do not return a nil error when returning a nil value that should accompany a non-nil error (or vice versa). If you assign `err` and then return `nil, nil`, this linter catches the logic bug.
- **nilnesserr**: catches contradictions where a nil-checked variable is used in a way that assumes non-nil, or where a nil pointer is dereferenced after an error check.
- **errname**: error types must be named `ErrFoo` (sentinel values) or `FooError` (custom error types). Follow Go naming convention for error variables.
- **errchkjson**: types that marshal to JSON must handle errors from `json.Marshal` / `json.Unmarshal`. Do not discard the error from `json.Marshal` — even structs can fail if they contain unsupported types.
- **nilnil**: do not return `(nil, nil)` — it forces callers to check both values. Return a sentinel error or a typed zero-value instead.

#### Function Design

- **funlen** (`lines: 120, statements: 70`): functions longer than 120 lines or 70 statements must be split. Extract helpers, reduce nesting, or break into smaller functions. This is a hard limit — if a function exceeds it, refactor before commit.
- **gocyclo** (`min-complexity: 30`): cyclomatic complexity above 30 is blocked. Reduce branching with early returns, guard clauses, and extracted helpers. If a single function needs 30+ independent paths, it is doing too much.
- **gocognit** (`min-complexity: 30`): cognitive complexity above 30 is blocked. Unlike cyclomatic complexity, this penalizes nesting. Deep nesting (`if` inside `if` inside `for` inside `if`) accumulates points fast — flatten with early returns.
- **nestif** (`min-complexity: 5`): nesting depth beyond 5 levels is blocked. Every nested `if`/`for`/`select` adds complexity. Flatten by returning early, extracting methods, or using guard clauses.
- **maintidx** (`under: 10`): maintainability index below 10 is blocked. This combines Halstead volume, cyclomatic complexity, and line count. Low scores mean the function is hard to understand and maintain.
- **unparam**: function parameters that are never used (or always receive the same value) must be removed. Dead parameters add noise and confusion.
- **recvcheck**: receiver type (pointer vs value) must be consistent across all methods of a type. Do not mix pointer and value receivers on the same struct.

#### Resource Management

- **bodyclose**: HTTP response bodies must be closed with `defer resp.Body.Close()`. Unclosed bodies leak connections and file descriptors.
- **rowserrcheck**: `database/sql.Rows` must check `rows.Err()` after `rows.Next()` loop. The loop may exit early on error — `rows.Err()` is the only way to detect it.
- **sqlclosecheck**: `sql.Rows`, `sql.Stmt`, and `sql.Conn` must be closed with `defer Close()`. Opening without closing leaks database resources.
- **noctx**: every HTTP request must carry a `context.Context`. Use `http.NewRequestWithContext(ctx, ...)` — never `http.NewRequest(...)` without context. Context enables cancellation, timeouts, and tracing.
- **contextcheck**: `context.Context` must be passed through the call chain. Do not create a new background context mid-chain — propagate the parent context for cancellation to work.

#### Performance & Allocation

- **prealloc**: pre-allocate slices with `make([]T, 0, n)` when the size is known or can be estimated. Without pre-allocation, `append` doubles the slice and copies on every growth.
- **wastedassign**: assignments that are never used (assigned and then overwritten without being read) must be removed. Wasted assignments confuse readers and may indicate a logic bug.
- **ineffassign**: values assigned but never read (the assignment itself is ineffective) must be removed. Unlike `wastedassign`, the value may be used in a later branch that is never taken.
- **makezero**: `append` to a nil slice after `make([]T, 0)` is fine, but `make([]T, n)` followed by `append` without index overwrites existing zero-values. Use `make([]T, 0, n)` when you intend to `append`.
- **perfsprint**: use `fmt.Sprintf` only when formatting is needed. For simple conversions, use `strconv.Itoa`, `strconv.AppendInt`, `fmt.Sprint` without format verbs. `fmt.Sprintf` with no verbs is slower than `fmt.Sprint`.
- **mirror**: identifies code that can be simplified by using standard library functions. If `strings.Contains(s, x)` replaces a manual loop, use the stdlib version.
- **fatcontext**: `context.Context` should not be stored in structs or wrapped in tight loops. Passing a fat context (one that accumulates values) through many layers degrades performance.

#### Security (Go)

- **gosec**: flags common security issues — hardcoded credentials, weak crypto (`crypto/md5`, `crypto/sha1`), SQL injection patterns, file path traversal, command injection. Treat every finding as a blocker unless explicitly accepted.
- **bidichk**: detects bidirectional Unicode text that can hide malicious code in source files. Prevents trojan-source attacks.
- **durationcheck**: detects cases where `time.Duration` is multiplied by another `time.Duration`, which almost always indicates a bug (duration² is nonsensical).

#### Code Quality (Go)

- **govet**: Go vet checks for common mistakes — unreachable code, wrong lock usage, incorrect `Printf` format strings, shadowed returns. Must be zero.
- **staticcheck**: broad set of Go correctness and performance checks. Includes `SA*` (analysis), `S*` (style), `ST*` (static analysis), `QF*` (quick fixes). Treat as mandatory.
- **unused**: exported and unexported identifiers that are never referenced must be removed. Dead code is maintenance burden.
- **ineffassign**: ineffective assignments (value written but never read) must be removed.
- **misspell**: catches spelling mistakes in comments and strings. Fix before commit.
- **dupword**: detects duplicate words in comments and strings (e.g., "the the"). Fix before commit.
- **gocritic** (enabled: `badLock`, `badRegexp`, `evalOrder`, `httpNoBody`, `nilValReturn`, `sloppyReassign`, `truncateCmp`, `weakCond`, `importShadow`): catches subtle logic errors — incorrect mutex usage, malformed regexps, evaluation order issues, comparison truncation, weak conditions, import shadowing. `exitAfterDefer` is disabled because `cmd/` packages legitimately call `os.Exit` after `defer`.
- **copyloopvar**: ensures loop variables are captured correctly in closures. Since Go 1.22 loop variables are per-iteration, but this linter catches remaining patterns where the address of a loop variable is taken.
- **revive** (rules: `blank-imports`, `context-as-argument`, `error-return`, `error-strings`, `var-naming`, `unused-parameter`, `unused-receiver`): enforces Go naming and API conventions. Context must be first argument. Errors must be returned as the last value. Error strings must not be capitalized or end with punctuation. Unused parameters and receivers must be named `_`.
- **whitespace**: enforces consistent blank line usage — no double blank lines, blank lines at start/end of blocks. Keeps code visually clean.
- **exhaustive**: `switch` statements on enums must cover all cases or have a `default`. Missing enum cases are bugs.
- **nolintlint**: `//nolint` directives must name the specific linter being suppressed (e.g., `//nolint:errcheck`). Bare `//nolint` is banned — it hides all findings, not just the intended one.
- **predeclared**: do not shadow Go predeclared identifiers (`len`, `cap`, `new`, `make`, `true`, `false`, `nil`, etc.). Shadowing builtins causes confusion.
- **loggercheck** + **sloglint**: structured logging rules. Use `slog` with key-value pairs, not `fmt.Print*`. `slog.Info("msg", "key", value)` — keys must be string constants, values must match the logging context. Never use `fmt.Println` / `fmt.Printf` for logging in application code.
- **forbidigo** (forbidden: `fmt.Print*`, `panic`): application code must use structured logging, not `fmt.Print*`. `panic` is banned in application code — return errors instead. Exception: `cmd/` packages may use `fmt.Print*` for CLI output.
- **usestdlibvars**: use standard library variables instead of string literals. E.g., `http.MethodGet` instead of `"GET"`, `os.Stdout` instead of explicit file opens. Reduces typo risk.
- **unconvert**: remove unnecessary type conversions. `int(x)` where `x` is already `int` is noise.
- **reassign**: detects reassignment of imported package variables or loop variables that may cause subtle bugs.
- **forcetypeassert**: type assertions `x.(T)` must use the two-value form `x, ok := y.(T)`. One-value assertions panic if the type doesn't match.
- **musttag**: exported struct fields that are marshaled to JSON/XML/YAML must have struct tags. Missing tags cause silent data loss or incorrect serialization.
- **gomoddirectives**: `go.mod` directives (`replace`, `retract`, `exclude`) must not be used without explicit justification. Local `replace` directives are especially dangerous in published modules.
- **dupl** (`threshold: 100`): code blocks longer than 100 tokens that are duplicated must be extracted into a shared function. Tests (`_test.go`) are exempt because test duplication is acceptable for clarity.

### Frontend — ESLint Rules (`frontend/eslint.config.js`)

#### TypeScript Type Safety

- **no-explicit-any**: never use `any` as a type. Use unknown, generic parameters, or a specific type. `any` opts out of all type checking — it is the TypeScript equivalent of a segfault waiting to happen.
- **no-unsafe-call / no-unsafe-assignment / no-unsafe-member-access / no-unsafe-argument / no-unsafe-return**: values typed as `any` must not flow through the program. If a third-party library returns `any`, cast it to a specific type at the boundary immediately. Every unsafe usage is a potential runtime error.
- **no-floating-promises**: every Promise must be awaited, returned, or explicitly voided with `void promise`. Unhandled promises cause silent failures — errors in `.then()` chains disappear.
- **no-misused-promises**: do not pass async functions where synchronous callbacks are expected (e.g., `addEventListener`, `Array.prototype.sort`). The return value (a Promise) is ignored, and errors are lost.
- **await-thenable**: only `await` values that are thenable (Promises). Awaiting a non-Promise is usually a bug or unnecessary.
- **no-unnecessary-condition**: conditions that are always true or always false (based on type analysis) must be removed. They indicate a type narrowing error or dead code.
- **prefer-nullish-coalescing**: use `??` instead of `||` for defaults. `||` treats `0`, `""`, and `false` as falsy — `??` only treats `null` and `undefined` as nullish. Prevents bugs with empty strings and zero values.
- **no-redundant-type-constituents**: remove redundant types from unions. `string | string` is `string`. `T | T` is `T`.
- **no-base-to-string**: do not call `.toString()` on types that have no meaningful string representation (e.g., plain objects, arrays). Use `JSON.stringify` or explicit formatting instead.
- **no-empty-function**: empty function bodies are not allowed unless the function is intentionally a no-op. If it is a no-op, add a comment explaining why.
- **ban-ts-comment**: `@ts-ignore` and `@ts-expect-error` are banned. Fix the type error instead of suppressing it. `@ts-expect-error` is acceptable only in test files for type-level assertions.
- **prefer-for-of**: use `for...of` instead of indexed `for` loops when the index is not needed. More readable and works with any iterable.
- **consistent-type-imports** (`prefer: type-imports`): use `import type { X }` for type-only imports. Type imports are erased at compile time and do not affect the runtime bundle. Mixing value and type imports in the same statement is fine, but pure type imports must use the `type` keyword.
- **no-unused-vars** (`argsIgnorePattern: ^_`): unused variables are errors. Prefix intentionally unused parameters with `_` (e.g., `(_event: MouseEvent)`).

#### React Hooks

- **react-hooks/rules-of-hooks**: hooks must be called at the top level of a React function component or custom hook. Never inside loops, conditions, or nested functions. The call order must be stable across renders.
- **react-hooks/set-state-in-effect**: do not call `setState` directly in a `useEffect` body that runs on every render. Use a dependency array or move the state update to an event handler.
- **react-hooks/refs**: refs must be used correctly — do not read a ref during rendering (it causes stale UI), only in effects and event handlers.
- **react-hooks/immutability**: state values must not be mutated. Always create new objects/arrays for state updates. Mutation breaks React's change detection.
- **react-hooks/purity**: render functions must be pure — no side effects during rendering. Move side effects to `useEffect` or event handlers.
- **react-refresh/only-export-components** (`allowConstantExport: true`): files with React components should only export components and constants. Exporting non-component functions alongside components breaks hot module replacement.

#### Code Quality (Frontend)

- **complexity** (`max: 20`): cyclomatic complexity above 20 per function is blocked. Break into smaller functions.
- **max-depth** (`max: 4`): nesting deeper than 4 levels is blocked. Flatten with early returns, extracted functions, or guard clauses.
- **no-console** (`allow: ['warn', 'error']`): `console.log` is banned in production code. Use `console.warn` / `console.error` for warnings/errors, or a proper logging utility. Debug `console.log` statements must be removed before commit.
- **no-debugger**: `debugger` statements are banned. Remove before commit.
- **eqeqeq** (`always`, `null: ignore`): always use `===` / `!==` for equality checks. Exception: `x == null` is allowed because it checks both `null` and `undefined` (the idiomatic TypeScript pattern).
- **curly** (`all`): all control flow statements (`if`, `else`, `for`, `while`, `do`) must use curly braces. Single-line bodies must still use braces — prevents bugs when adding lines later.

#### Security (Frontend)

- **detect-non-literal-regexp**: flags `new RegExp(userInput)` — user-controlled regex patterns can cause ReDoS (regular expression denial of service). Validate or sanitize user input before using in regex.
- **detect-unsafe-regex**: flags regex patterns that are vulnerable to catastrophic backtracking (e.g., `(a+)+`). These can hang the browser.
- **detect-eval-with-expression**: `eval()`, `new Function()`, and similar are banned. They execute arbitrary code and are XSS vectors.
- **detect-object-injection** (`off`): disabled because bracket notation `obj[key]` is common and safe with typed objects. This rule produces too many false positives in TypeScript codebases.

#### Import Sorting

- **simple-import-sort/imports + exports**: imports and exports must be sorted consistently. Groups: (1) side-effects, (2) external packages, (3) internal aliases, (4) relative imports, (5) type imports. Within each group, alphabetize. Consistent import ordering reduces merge conflicts and improves readability.

#### Accessibility

- **jsx-a11y/alt-text**: all `<img>` elements must have an `alt` prop. Decorative images use `alt=""`. Meaningful images describe the content.
- **jsx-a11y/anchor-is-valid**: `<a>` elements must have a valid `href`. `href="#"` is banned — use a button for non-navigational actions, or a real URL for navigation.
- **jsx-a11y/no-autofocus**: `autoFocus` attribute is banned. It steals focus from assistive technologies and screen readers. Manage focus programmatically with `useRef` + `.focus()` when needed.

#### Test Relaxations

- In `**/*.test.{ts,tsx}` and `**/test/**`: `no-explicit-any` and `no-non-null-assertion` are relaxed. Test code often needs `any` for partial mocks and `!` for asserting test preconditions. Production code has no such exemptions.

## Naming Conventions — Best Practices

### Go Naming (golangci-lint enforced: `revive` var-naming, `errname`, `recvcheck`)

Sources: [Effective Go — Names](https://go.dev/doc/effective_go#names), [Google Go Style — Naming](https://google.github.io/styleguide/go/decisions#names).

#### General (Go)

- **MixedCaps**: Go uses `camelCase` (unexported) and `PascalCase` (exported). No snake_case (use `MaxSize`, not `MAX_SIZE`).
- **No underscores**: Do not use underscores in package, function, variable, or constant names. Exceptions: `_test.go` files and name matching for cgo/syscall interfaces.
- **Single-letter variables**: Only use them for short-lived loop indices (`i`, `j`), method receivers, or standard abbreviations (`r` for `io.Reader`, `w` for `io.Writer`, `ctx` for `context.Context`, `err` for `error`). For scopes wider than 10 lines, descriptive names are required.

#### Receiver

- Receiver names must be short (1-2 characters), usually abbreviations of the type name, and consistent across all methods of the type.
- **CORRECT**: `func (s *Service)`, `func (r *Repository)`, `func (h *AuthHandler)`, `func (m *JWTManager)`
- **INCORRECT**: `func (this *Service)`, `func (self *Scanner)`, `func (svc *Service)`

#### Functions

- **No `Get` prefix**: Go convention avoids `Get` prefixes. Prefer `User` over `GetUser`, `Count` over `GetCount`, and `PublicKey` over `GetPublicKey`.
  - Exception: HTTP GET concept (REST handler names like `GetMe` are acceptable as endpoint definitions, but not regular methods).
  - Use `Compute`, `Fetch`, or `Resolve` for expensive computations or remote network calls.
- **Exported functions**: `PascalCase`. Do not repeat the package name (`datasource.Config`, not `datasource.DatasourceConfig`).
- **Unexported functions**: `camelCase`. Helper functions should be descriptive (`composePostgresDSN`, `introspectStep`).

#### Interfaces

- Single-method interfaces are named by method name + `-er` suffix: `Reader`, `Writer`, `Stringer`, `Scanner`, `Embedder`.
- Multi-method interfaces should have a descriptive noun: `Driver`, `ResponseCache`, `ModelLoader`, `TemplateStore`.
- Test-only small internal interfaces (package-private): `scanner`, `rowsScanner`, `rowScanner` — lowercase and focused.

#### Constants (Go)

- `MixedCaps` (exported: `PascalCase`, unexported: `camelCase`). Do not use `UPPER_SNAKE_CASE` or `kPrefix`.
- **CORRECT**: `const MaxRetries = 3`, `const defaultTimeout = 30 * time.Second`
- **INCORRECT**: `const MAX_RETRIES = 3`, `const kMaxRetries = 3`
- Name constants by their role/meaning, not their value: `const Twelve = 12` is bad practice.

#### Error Variables

- Sentinel errors: `Err` prefix + PascalCase. `ErrNotFound`, `ErrUnauthorized`, `ErrCircuitOpen`.
- **CORRECT**: `var ErrInvalidCredentials = errors.New("ldap: invalid credentials")`
- **INCORRECT**: `var NotFound = errors.New(...)`, `var errInvalid = errors.New(...)`
- Error types: `Error` suffix. `TimeoutError`, `ServiceError`, `PathError`.

#### Initialisms / Acronyms

- Standard abbreviations must be consistently cased: `URL`, `ID`, `HTTP`, `JSON`, `SQL`, `API`, `DB`, `DSN`, `JWT`, `PII`, `RBAC`, `MFA`, `OTEL`, `NATS`, `CORS`, `CSRF`, `OAuth`, `LDAP`, `SMTP`, `TOTP`, `DNS`, `TCP`, `IP`, `SSH`, `SSL`, `TLS`, `CPU`, `RAM`, `OS`.
- All letters must share the same casing: `URL` or `url`, never `Url`. `XMLAPI` $\rightarrow$ exported, `xmlAPI` $\rightarrow$ unexported.
- `ID` is always `ID` (capitalized), never `Id`. E.g., `userID` not `userId`, and `APIKey` not `ApiKey`.

#### Repetition / Stutter

- Avoid repeating package names in type names: `datasource.Info` instead of `datasource.DatasourceInfo`. `config.Config` is an acceptable exception (standard pattern).
- Variable names should not stutter their type: `var users int` instead of `var userCount int` (if count context is clear).

### TypeScript / React Naming (ESLint enforced: `@typescript-eslint/naming-convention`)

Sources: [Google TypeScript Style Guide — Naming](https://google.github.io/styleguide/tsguide.html#naming).

#### General (TypeScript/React)

- **`UpperCamelCase`**: class, interface, type, enum, decorator, type parameter, React component.
- **`lowerCamelCase`**: variable, parameter, function, method, property, module alias.
- **`CONSTANT_CASE`**: global constant values, enum values. Only for module-level or `static readonly` class fields.
- **No `I` prefix for interfaces**: `AuthProvider`, not `IAuthProvider` (or use a purpose-built name like `AuthStorage`).

#### React Specific

- **Components**: `UpperCamelCase` — `QueryHistory`, `DatasourceForm`, `ExpressionBuilder`.
- **Handlers**: `handle` + event name — `handleSubmit`, `handleClick`, `handleKeyDown`. DOM event props use `on` prefix: `onClick`, `onChange`, `onSubmit`.
- **Boolean states**: `is` / `has` / `should` / `can` prefix — `isLoading`, `hasPermission`, `shouldRefresh`, `isOpen`, `canEdit`.
- **useState names**: consistent `[value, setValue]` pairs — `[open, setOpen]`, `[loading, setLoading]`, `[error, setError]`. Keep it concise.
- **Custom hooks**: `use` prefix — `useAuth`, `useT`, `useAIJobs`, `useLocale`.

#### Abbreviations

- Treat abbreviations as regular words: `loadHttpUrl` (good), `loadHTTPURL` (bad). Exception: platform names (`XMLHttpRequest`).
- Do not use confusing abbreviations: `n`, `nErr`, `cstmrId`, `wgcConnections`. Common abbreviations are acceptable: `url`, `dns`, `id`, `http`.
- **`ID` over `Id`**: `customerID` instead of `customerId` (Google TS style) — though local `camelCase` might normalize `id` as lowercase. Consistency is key.

#### Constants (TypeScript/React)

- Module-level constants: `CONSTANT_CASE`. `CHART_COLORS`, `AI_QUERY_TIMEOUT_MS`, `STORAGE_KEY`.
- Inside functions: `lowerCamelCase`. `const maxRetries = 3` (do not use uppercase for local function-scoped constants).

## Anti-Patterns to Avoid

1. **Don't add `fmt.Sprintf` in hot-path predicate builders** — use direct string concatenation or `strings.Builder`.
2. **Don't pool values that escape into return values** — unsafe, caller may hold reference after pool recycle.
3. **Don't use `jwt.MapClaims` when typed claims structs exist** — already migrated.
4. **Don't break backward compatibility on read paths** — fail-open with warning log.
5. **Don't leave duplicated handler logic** — extract shared helper on first sight.
6. **Don't optimize without benchmark numbers** — every "maybe faster" hypothesis tested was either same speed or slower.
7. **Don't commit with lint errors** — `make lint-go` / `make lint-frontend` / `make test-go` must all pass cleanly (zero errors).
8. **Don't hardcode Tailwind colors for themed UI** — use `@theme inline` token bridge (`bg-card`, `text-foreground`, …). Legacy BEM CSS in `frontend/src/styles/` may remain until migrated; don't broad-rewrite without an explicit task.
9. **Don't trust eslint+tsc to catch Prettier drift** — run `prettier --check`/`--write` on touched frontend files; CI's `format:check` is a separate gate.
