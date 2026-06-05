# Todo list

## Internal Query Compile Duplicate Cleanup

Success criteria:

- `Compile` and `DryRun` share the duplicated compile/metrics/error path through one helper.
- Existing response shapes and fingerprints remain unchanged.
- Edited Go file diagnostics, focused handler tests, and whitespace checks pass.

- [x] Extract shared internal compile helper.
- [x] Replace duplicated `Compile` and `DryRun` handler bodies with helper calls.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## Internal Query Compile Duplicate Cleanup Results

Resolved:

1. Added `compileLogicalQuery` to centralize internal query compile timing, metrics, service-error handling, and defensive nil-result handling.
2. Replaced the duplicated compile blocks in `Compile` and `DryRun` while keeping their response DTOs unchanged.
3. Added defensive nil-result handling in `Run` to satisfy diagnostics and avoid panics if a runner returns `nil, nil`.

Verification:

- `get_errors` on `internal_query.go`: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check -- internal/http/handlers/internal_query.go tasks/todo.md`
- Duplicate check: `RecordQueryCompile` appears only once, inside `compileLogicalQuery`.

Notes:

- Full `git diff --check` was attempted and reports unrelated existing whitespace issues in `internal/ai/context_user.go`, `internal/http/handlers/history.go`, and `internal/metadata/ai_prompt_templates.go`.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Slice

Success criteria:

- `internal/ai/abtest` defines experiment, variant, metric, and status types.
- Traffic allocation is deterministic per `user_id + experiment_id`.
- Variant traffic percentages are validated before allocation.
- Allocation handles boundary buckets and zero-traffic variants safely.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing unit tests for deterministic traffic allocation and validation.
- [x] Implement Phase 1 A/B experiment data model types.
- [x] Implement deterministic traffic allocation helper.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 1 Experiment Data Model Results

Resolved:

1. Added `internal/ai/abtest` with `ExperimentStatus`, `Experiment`, `Variant`, and `ExperimentMetrics`.
2. Added deterministic `SelectVariantForUser(userID, experimentID, variants)` allocation using a stable hash bucket from `user_id + experiment_id`.
3. Added `ValidateVariantsForAllocation` to enforce 100% total traffic, 0-100 per variant, and exactly one control variant.
4. Added tests for deterministic assignment, cumulative traffic boundaries, zero-traffic variants, and validation failures.
5. Updated `PROMPT_AB_TEST_PLAN.md` Phase 1 checklist.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest ./internal/ai/prompt -count=1`
- `GOCACHE=/private/tmp/biqly-gocache golangci-lint run ./internal/ai/abtest`
- `git diff --check`

Notes:

- `golangci-lint` completed with `0 issues`; sandbox blocked writes to `~/Library/Caches/golangci-lint`, producing cache warnings only.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Slice

Success criteria:

- `migrations/042a_add_ab_experiments.up.sql` creates `ab_experiments` and `ab_variants`.
- `ai_query_history` gains nullable `ab_experiment_id` and `ab_variant_id` columns.
- `migrations/042b_add_ab_experiments.down.sql` reverses the history columns and experiment tables safely.
- Focused Go migration-shape tests and `git diff --check` pass.
- Existing DB apply is attempted and either verified or recorded with the exact local blocker.

- [x] Add a failing migration-shape test for the Phase 2 schema contract.
- [x] Create the `042a` up migration for experiment tables and history context columns.
- [x] Create the `042b` down migration.
- [x] Run focused Go tests for `cmd/migrate` and `internal/ai/abtest`.
- [ ] Verify migration runs cleanly against an existing metadata DB.

## PROMPT_AB_TEST_PLAN Phase 2 Database Schema Results

Resolved:

1. Added `migrations/042a_add_ab_experiments.up.sql` with `ab_experiments`, `ab_variants`, `idx_ab_exp_status`, and nullable `ai_query_history` experiment context columns.
2. Added `migrations/042b_add_ab_experiments.down.sql` to drop history context columns before dropping dependent A/B tables.
3. Added `cmd/migrate/ab_experiments_migration_test.go` to pin the Phase 2 migration contract.
4. Updated `PROMPT_AB_TEST_PLAN.md` Phase 2 migration-file checklist.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1` failed because `migrations/042a_add_ab_experiments.up.sql` did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run TestABExperimentsMigrationFiles -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`

Blocked:

- Existing DB apply could not be completed locally on 2026-06-05: Docker daemon is not running, `postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable` refused connections, and the installed `libpq` `initdb` cannot start a temporary server because the matching `postgres` binary is missing.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Slice

Success criteria:

- `internal/ai/abtest/repository.go` exposes CRUD methods for experiments and variants.
- Repository create/update paths validate traffic allocation, one control variant, and prompt template version existence.
- Running-experiment lookup filters by template name, locale, and running status.
- Focused repository tests and `git diff --check` pass.

- [x] Add failing repository tests for create validation and running-experiment lookup.
- [x] Implement the minimal repository layer.
- [x] Update `PROMPT_AB_TEST_PLAN.md` Phase 3 checklist once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## PROMPT_AB_TEST_PLAN Phase 3 Repository Layer Results

Resolved:

1. Added `internal/ai/abtest/repository.go` with experiment CRUD, variant CRUD, and running-experiment lookup by template/locale/status.
2. Added validation before running an experiment so variant traffic sums to 100 and exactly one control variant exists.
3. Added prompt-template version existence validation before adding/updating variants and while validating running experiments.
4. Added mock-runner repository tests for invalid running allocation, missing template version, and running-experiment lookup.

Verification:

- RED: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1` failed because the repository API did not exist.
- GREEN: `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -run 'TestRepository' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai/abtest -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate ./internal/ai/abtest -count=1`
- `git diff --check`

## HTTP Handler Log Context Duplicate Cleanup

Success criteria:

- Request/user/workspace log context args are appended through one helper.
- `writeServiceError` and `writeInternalAPIError` preserve existing structured log fields.
- Focused handler tests, diagnostics, duplicate check, and whitespace checks pass.

- [x] Extract shared log context args helper.
- [x] Replace duplicated log context blocks in service/internal error writers.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## HTTP Handler Log Context Duplicate Cleanup Results

Resolved:

1. Added `appendRequestLogArgs` to append `request_id`, `user_id`, and `workspace_id` consistently for handler structured logs.
2. Replaced duplicated log context blocks in `writeServiceError` and `writeInternalAPIError`.
3. Removed now-unused `bimw` and `requestid` imports from `internal.go`.

Verification:

- `get_errors` on `helpers.go` and `internal.go`: no errors found.
- Duplicate check: `requestid.FromContext(ctx)` appears only once in handlers, inside `appendRequestLogArgs`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## AI Run Datasource Process Options Duplicate Cleanup

Success criteria:

- Local datasource run process options are built in one helper.
- Sync and async AI run paths preserve existing SQL validation, target dialect, few-shot, and sample-data behavior.
- Focused handler tests, diagnostics, and whitespace checks pass.

- [x] Extract shared local run process options helper.
- [x] Replace duplicated sync and async run process option blocks.
- [x] Run diagnostics, focused Go tests, duplicate check, and whitespace checks.

## AI Run Datasource Process Options Duplicate Cleanup Results

Resolved:

1. Added `localRunProcessOptions` to build local datasource SQL dry-run, target dialect, few-shot, and sample-data process options once.
2. Replaced the duplicated sync `processAndObserve` and async `executeAIQueryPhase` option-building blocks.
3. Added defensive datasource nil checks so local run execution fails cleanly instead of dereferencing an invalid resolved datasource.
4. Kept QueryClient run handling explicit before local datasource execution.

Verification:

- `get_errors` on `ai_job_exec.go`: no errors found.
- `get_errors` on `ai.go`: no refactor-related errors; existing unrelated IDE warnings remain for `routing` name/package collisions and pre-existing nil-analysis warnings around `observeAIRequest`.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- Duplicate check: local datasource process-options fragment appears only once in `localRunProcessOptions`.
- `git diff --check`

## AI History Permission Duplicate Cleanup

Success criteria:

- Shared AI history/detail view permission logic is implemented once.
- `AIHistory`, `AIHistoryDetail`, and AI usage breakdown preserve existing behavior.
- Focused handler package tests and diff checks pass.

- [x] Extract shared AI view-detail permission helper.
- [x] Replace duplicated handler blocks with the helper.
- [x] Run diagnostics, focused Go tests, and `git diff --check`.

## AI History Permission Duplicate Cleanup Results

Resolved:

1. Added shared `canViewAIHistoryDetails` helper for the AI view-detail permission check.
2. Replaced duplicated permission blocks in `AIHistory`, `AIHistoryDetail`, and `GetAIUsageBreakdown`.
3. Preserved existing behavior for auth-disabled legacy mode, super admins, empty user IDs, and permission-service errors.

Verification:

- `get_errors` on edited Go files: no errors found.
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Slice

Success criteria:

- Backend has a `POST /api/semantic/models/{id}/compile-expression` endpoint to validate and compile expression ASTs/strings to dialect-specific SQL.
- A recursive `ExpressionBuilder` React component is created and supports visual AST building, whitelisted function calls, and case expressions.
- Text mode in the expression builder supports raw string inputs and real-time backend compilation.
- Metric creation and editing support the new `ExpressionBuilder` and persist ASTs.
- Calculated dimension editing in the modeling panel uses the expression builder.
- All frontend and backend tests pass cleanly, and the frontend build succeeds.

- [x] Implement backend `POST /api/semantic/models/{id}/compile-expression` handler and route.
- [x] Add unit tests for the backend compilation endpoint.
- [x] Create `ExpressionBuilder.tsx` component and `expressionBuilder.css`.
- [x] Integrate `ExpressionBuilder` into `AddMetricModal.tsx` for creation and editing.
- [x] Add calculated dimension editing to `ModelingPalette.tsx` using the `ExpressionBuilder`.
- [x] Verify everything via frontend tests/build and backend pre-commit checks.

## EXPR_AST_PLAN Phase 8 Frontend Expression Builder Results

Verified:

1. Backend route `POST /api/semantic/models/{id}/compile-expression` is wired in `internal/http/catalog_router.go`.
2. Handler `CompileExpression` accepts expression strings or AST payloads and returns compiled SQL.
3. Backend endpoint coverage exists in `internal/http/handlers/semantic_expr_api_test.go`.
4. `ExpressionBuilder.tsx` and `expressionBuilder.css` exist and support text/visual modes, whitelisted functions, binary/unary nodes, references, and CASE expressions.
5. `AddMetricModal.tsx` uses `ExpressionBuilder` for metric expressions and persists AST payloads.
6. `ModelingPalette.tsx` opens dimension editing, and `EditDimensionModal.tsx` uses `ExpressionBuilder` for calculated dimensions.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run TestCompileExpression -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `GOCACHE=/private/tmp/biqly-gocache make lint-go`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `npm --prefix frontend run lint`
- `git diff --check`

Notes:

- `npm --prefix frontend run lint` completed with 0 errors and existing warnings under the configured `--max-warnings 1500` threshold.
- `GOCACHE=/private/tmp/biqly-gocache make lint-go` completed with 0 issues; sandbox prevented writing golangci-lint cache facts under `~/Library/Caches`, producing cache warnings only.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Slice

Success criteria:

- Command `go run ./cmd/migrate backfill` parses existing expression strings to JSON ASTs and updates the database.
- `ExprDependencies` extracts all dependencies recursively from an expression AST.
- Publish validation detects circular dependencies between metrics and dimensions, returning validation errors.
- Lineage endpoint `GET /api/semantic/models/{id}/lineage` returns the dependency graph of metrics and dimensions.
- `ValidateExprStrict` recursively validates function names, arity, column/metric/dimension existence, maximum depth, and nested aggregations.
- Compile-time safety net runs `ReadOnlyChecker` on compiled expressions.
- Focused Go tests pass cleanly.

- [x] Add backfill command to `cmd/migrate/main.go` and test it.
- [x] Implement `ExprDependencies` in `pkg/semantic/expr_lineage.go` and add unit tests.
- [x] Detect circular dependencies in publish validation.
- [x] Add GET `/api/semantic/models/{id}/lineage` endpoint in HTTP handlers and router.
- [x] Implement `ValidateExprStrict` in `pkg/semantic/expr_validation.go` and wire it into publish validation.
- [x] Add compile-time safety net in expression compiler.
- [x] Run focused tests, verify all checklist items, and update `EXPR_AST_PLAN.md`.

## EXPR_AST_PLAN Phase 5.3, 6, & 7 Backfill, Lineage, and Hardening Results

Resolved:

1. Added database backfill command `backfill` to `cmd/migrate/main.go` to parse legacy string expressions to JSON AST format and save them, plus wrote integration tests.
2. Implemented `ExprDependencies` to recursively extract references (columns, metrics, dimensions) from expression ASTs.
3. Added circular dependency detection using DFS during model publish.
4. Added HTTP endpoint `GET /api/semantic/models/{id}/lineage` to return nodes and edges for the lineage dependency graph.
5. Implemented strict AST validation in `pkg/semantic/expr_validation.go` (checking function whitelist, arity, column/metric/dimension existences, max depth of 10, and blocking nested aggregates) and integrated it into the publish phase.
6. Embedded compile-time safety net calling `ReadOnlyChecker.Check` to ensure compiled expression SQL only performs read-only operations.

Verification:

- Run `make precommit` (tests pass cleanly, zero lint/formatting findings).

## EXPR_AST_PLAN Phase 5.2 API Changes Slice

Success criteria:

- Dimension create/update requests accept `calculated_expression` strings and `calculated_expr` JSON AST payloads.
- Metric create/update requests accept legacy `expression` strings and `expr` JSON AST payloads.
- String expressions are parsed server-side when an expression parser is registered.
- Invalid expression JSON or string payloads return a bad-request error before repository writes.
- API response JSON includes AST fields when present.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing handler tests for expression AST request/response behavior.
- [x] Update semantic handler DTO mapping for dimension and metric expression ASTs.
- [x] Wire create/update handlers through shared request mapping helpers.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.2 once verified.
- [x] Run focused Go tests and frontend checks, then document results.

## EXPR_AST_PLAN Phase 5.2 API Changes Results

Resolved:

1. Dimension create/update requests now accept `calculated_expression` and `calculated_expr`.
2. Metric create/update requests now accept `expression` and `expr`.
3. Request mapping parses provided JSON AST payloads and parses string expressions server-side when `ExpressionParser` is registered.
4. Invalid AST or string expressions return `400 Bad Request` before repository writes.
5. API response structs already serialize `calculated_expr` and `expr`; frontend semantic types now model those response fields.
6. Modeling rename/reactivate payloads preserve AST fields so full update calls do not drop them.
7. Updated `EXPR_AST_PLAN.md` Phase 5.2.

Left open intentionally:

- Existing-row JSON backfill remains the open Phase 5.1/5.3 item.
- Full visual expression editor work remains a later frontend task.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers -run 'TestDimensionFromRequest|TestMetricFromRequest|TestExpressionAPI|TestCreateDimensionRejectsInvalidExpressionASTBeforeRepoWrite|TestSemanticExpressionAPIResponse' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/http/handlers ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `npm --prefix frontend run test -- entityActions`
- `npm --prefix frontend run build`

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Slice

Success criteria:

- Migrations add expression AST JSONB columns for dimensions and metrics.
- Repository write paths persist AST JSON when AST fields are present.
- Repository read paths prefer JSON AST columns and fall back to string parsing.
- Existing string expression fields remain backward-compatible.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing tests for expression AST JSON encode/decode helpers.
- [x] Add migration for dimension/metric AST JSON columns.
- [x] Update repository dimension/metric insert, update, select, and scan paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 5.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 5.1 Dual-Format Storage Results

Resolved:

1. Added migration `040a_add_expression_ast_json.up.sql` for `semantic_dimensions.calculated_expr_json` and `semantic_metrics.expr_json`.
2. Added matching down migration `040b_add_expression_ast_json.down.sql`.
3. Added `calculated_expression` with `IF NOT EXISTS` because the Go model already had the field but existing migrations did not create the column.
4. Added AST JSON encode/decode helpers for repository storage.
5. Updated dimension/metric create, bulk insert, update, select, and scan paths to write/read AST JSON.
6. Repository scan prefers JSON AST when present; Phase 4.4 hydration still parses legacy string expressions when JSON is missing or invalid.
7. Updated `EXPR_AST_PLAN.md` Phase 5.1 migration/repository items.

Left open intentionally:

- Existing-row backfill is still open. The plan says actual parsing should be done in Go, so this belongs to Phase 5.3 one-shot migration tooling rather than SQL-only migration.
- API accept/return behavior remains Phase 5.2.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestExpressionASTStorage|TestDecodeExprNodeJSON|TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Slice

Success criteria:

- Semantic repository load path populates parsed AST fields for dimension and metric expressions when a parser is registered.
- Parse failures leave AST fields nil for read-time fail-open compatibility.
- `internal/query` registers the parser alongside the existing validator hook.
- Publish validation remains covered by existing validation path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing semantic hydration tests for parser success and fail-open parse errors.
- [x] Add parser hook and repository hydration helper.
- [x] Register `ParseExpression` from `internal/query`.
- [x] Wire hydration into full model and published snapshot load paths.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.4 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.4 Load-Time Parsing Results

Resolved:

1. Added `ExpressionParser` hook in `internal/semantic` so semantic loading can parse expressions without importing `internal/query`.
2. Registered `ParseExpression` from `internal/query` alongside the existing calculated-expression validator hook.
3. Added `hydrateExpressionASTs` to populate `Dimension.CalculatedExpr` and `Metric.Expr` when the parser is available.
4. Wired hydration into `GetFullModel` and decoded published snapshots.
5. Kept read-time compatibility fail-open: parse errors log a warning and leave AST fields nil.
6. Added focused hydration tests for parser success and parse-error nil behavior.
7. Updated `EXPR_AST_PLAN.md` Phase 4.4.

Left open intentionally:

- Parse failures are fail-open at read time per the plan. Strict fail-closed behavior remains a Phase 7 compile-time safety-net task.
- JSON AST storage/backfill is still Phase 5.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/semantic -run 'TestHydrateExpressionASTs' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/semantic ./pkg/semantic ./pkg/logicalquery -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.3 Window AST Slice

Success criteria:

- `WindowSpec` can carry an AST expression while keeping `Expression` for backward compatibility.
- `buildWindowExpr` compiles `WindowSpec.Expr` via `CompileExpr`.
- AST window aggregate expressions are not re-quoted as identifiers.
- Existing window metric/ranking behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for `WindowSpec.Expr`.
- [x] Add `Expr` to `pkg/logicalquery.WindowSpec`.
- [x] Update `buildWindowExpr` to compile AST window expressions.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.3 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.3 Window AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/logicalquery.WindowSpec` while keeping `Expression` for backward compatibility.
2. Updated `buildWindowExpr` to compile `WindowSpec.Expr` through `CompileExpr`.
3. Reused the AST-aware aggregate wrapper so compiled window expressions are not re-quoted as identifiers.
4. Preserved existing metric-backed and ranking window behavior.
5. Added `TestCompiler_WindowFunctionUsesASTExpression`.
6. Updated `EXPR_AST_PLAN.md` Phase 4.3.

Left open intentionally:

- Legacy `WindowSpec.Expression` remains on the existing string/bracket-resolution path until load-time parsing and storage migration are complete.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_WindowFunctionUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_Window' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/logicalquery ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## Align SQL Golden Files Across Dialects

- [x] Modify `internal/query/golden_test.go` to add unified `TestGoldenAcrossDialects` and supporting fixtures
- [x] Run `UPDATE_GOLDEN=true go test -v ./internal/query -run TestGoldenAcrossDialects` to generate/align golden files
- [x] Run `make precommit` (lint and test) to ensure all tests and linters pass cleanly
- [x] Verify git diff and ensure all files are correctly created and aligned

## Align SQL Golden Files Across Dialects Results

Resolved:

1. Replaced single-dialect legacy golden test cases with a unified, table-driven test suite `TestGoldenAcrossDialects` in `internal/query/golden_test.go`.
2. Expanded test coverage to generate/test all 12 query scenarios for all 3 dialects (Postgres, MySQL, and SQL Server), checking a total of 36 compile targets.
3. Automatically generated the missing 10 SQL golden files for `mysql` and `sqlserver` databases under `testdata/sql/` using `UPDATE_GOLDEN=true`.
4. Fixed linter errors (gocritic, gosec) and ran `make lint-go` and `make test-go` successfully (zero errors).

## EXPR_AST_PLAN Phase 4.2 Metric AST Slice

Success criteria:

- `pkg/semantic.Metric` can carry a parsed `Expr` AST while keeping `Expression` for backward compatibility.
- Metric SELECT, HAVING/filters, ORDER BY, and bracket-token references use `CompileExpr` when `Expr` is present.
- Legacy string metric expressions remain on the existing compatibility path.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler test for metric `Expr` in SELECT/HAVING/ORDER BY.
- [x] Add `Expr` to `pkg/semantic.Metric`.
- [x] Update metric expression helpers/call sites to prefer AST.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.2 Metric AST Results

Resolved:

1. Added `Expr ExprNode` to `pkg/semantic.Metric` while keeping `Expression` for storage/backward compatibility.
2. Updated metric expression helpers and metric call sites to prefer `Metric.Expr` when available.
3. Added an AST-aware aggregate wrapper so compiled metric expressions are not re-quoted as identifiers.
4. Kept legacy string metric expressions on the existing `dialect.Aggregate` / bracket-resolution path.
5. Added `TestCompiler_MetricUsesASTExpression` for metric SELECT, filter expression, and ORDER BY alias behavior.
6. Updated `EXPR_AST_PLAN.md` Phase 4.2.

Left open intentionally:

- `resolveBracketExpressions` remains as the legacy fallback until load-time parsing and storage migration are complete.
- Window expressions remain raw-string based and are tracked separately in Phase 4.3.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_MetricUsesASTExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Slice

Success criteria:

- `pkg/semantic.Dimension` can carry a parsed `CalculatedExpr` AST while keeping `CalculatedExpression` for backward compatibility.
- `Compiler.dimensionSQL` prefers `CalculatedExpr` and compiles through `CompileExpr`.
- Legacy `CalculatedExpression` strings are parsed on the fly before compilation for migration safety.
- Existing calculated dimension SELECT, GROUP BY, and WHERE behavior remains covered.
- Focused Go tests and `git diff --check` pass.

- [x] Add failing compiler tests for `CalculatedExpr` and parsed fallback output.
- [x] Add `CalculatedExpr` to `pkg/semantic.Dimension`.
- [x] Update `dimensionSQL` to compile AST first and parse legacy strings.
- [x] Update `EXPR_AST_PLAN.md` Phase 4.1 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 4.1 Calculated Dimension AST Results

Resolved:

1. Added `CalculatedExpr ExprNode` to `pkg/semantic.Dimension` while keeping `CalculatedExpression` for storage/backward compatibility.
2. Updated `Compiler.dimensionSQL` to prefer `CalculatedExpr` and compile through `CompileExpr`.
3. Added migration fallback parsing for legacy `CalculatedExpression` strings before compiling them with `CompileExpr`.
4. Added `TestCompiler_CalculatedDimensionUsesAST` and updated calculated-dimension assertions to expect dialect-quoted AST SQL.
5. Updated the Postgres calculated-dimension golden file for the new AST output.
6. Updated `EXPR_AST_PLAN.md` Phase 4.1.

Left open intentionally:

- If a legacy calculated expression cannot be parsed, `dimensionSQL` still preserves the old raw-string behavior because the current function signature has no error return. Phase 7 compile-time safety should close that fail-closed path.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompiler_CalculatedDimension' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Function Mapping Slice

Success criteria:

- `CompileExpr` uses an explicit dialect function mapping table for approved scalar functions.
- `DATE_TRUNC` compiles through dialect-owned date truncation helpers instead of raw generic function output.
- Existing emitter behavior remains unchanged for the dialect matrix already covered.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing DATE_TRUNC/function mapping tests.
- [x] Implement explicit dialect function mapping and DATE_TRUNC transform.
- [x] Update `EXPR_AST_PLAN.md` Phase 3.2 once verified.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Function Mapping Results

Resolved:

1. Added an explicit `dialectFunctions` mapping table for AST function names.
2. Kept ClickHouse scalar function casing dialect-specific through the mapping table.
3. Added `DATE_TRUNC` special handling that compiles literal part plus column ref through each dialect's `DateTrunc` helper.
4. Added dialect-matrix coverage for `DATE_TRUNC('month', created_at)`.
5. Updated `EXPR_AST_PLAN.md` Phase 3.2.

Left open intentionally:

- `DATE_TRUNC` currently handles the safe planned shape: literal grain plus column ref. Broader expression arguments can be added when integration needs them.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects/date_trunc|TestCompileExprAcrossDialects' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 3 Emitter Slice

Success criteria:

- `CompileExpr` emits dialect-aware SQL for literals, column refs, binary/unary expressions, function calls, CASE, and concat overrides.
- Column refs go through `SchemaResolver.QualifyColumn`.
- Zero-value dialect structs are normalized to the initialized dialect defaults before quoting.
- Metric/Dimension model lookup and DATE_TRUNC argument transforms remain open for later context-aware integration.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing dialect matrix tests for AST-to-SQL emitter behavior.
- [x] Implement standalone emitter core in `internal/query/expr_compiler.go`.
- [x] Normalize zero-value dialect structs to existing initialized dialect defaults.
- [x] Update `EXPR_AST_PLAN.md` for completed Phase 3 emitter-core items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 3 Emitter Results

Resolved:

1. Added `internal/query/expr_compiler.go` with `CompileExpr` for canonical `pkg/semantic.ExprNode` values.
2. Added SQL emission for literals, column refs, binary/unary operators, function calls, and CASE expressions.
3. Routed column refs through `SchemaResolver.QualifyColumn`.
4. Added dialect concat overrides: PostgreSQL `||`, MySQL `CONCAT`, SQL Server `+`, ClickHouse `concat`.
5. Normalized zero-value dialect structs to the existing initialized dialect globals before quoting.
6. Added dialect-matrix expected SQL tests in `internal/query/expr_compiler_test.go`.
7. Updated `EXPR_AST_PLAN.md` for completed emitter-core items.

Left open intentionally:

- `MetricRefExpr` and `DimensionRefExpr` still need model-aware lookup/integration before the broad per-node-type checklist can close.
- DATE_TRUNC-style argument transforms are still open in the function mapping section.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestCompileExprAcrossDialects|TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 2 Parser Slice

Success criteria:

- `ParseExpression` returns canonical `pkg/semantic.ExprNode` values.
- Existing expression validation behavior remains intact.
- Bracket references, bare identifiers, qualified columns, functions, and CASE expressions have AST tests.
- Focused query/semantic tests and `git diff --check` pass.

- [x] Add failing parser AST tests for Phase 2.2/2.3 cases.
- [x] Refactor expression parser output from internal node types to canonical semantic AST.
- [x] Rename parser implementation to `internal/query/expression_parse.go`.
- [x] Keep `ValidateExpression` on the new parser path.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 2 Parser Results

Resolved:

1. Added `ParseExpression(expr string) (pkg/semantic.ExprNode, error)` as the public parser entry point.
2. Renamed the parser implementation to `internal/query/expression_parse.go`.
3. Refactored parser output from internal AST node structs to canonical `pkg/semantic` AST nodes.
4. Removed the old internal node types (`IdentifierNode`, `BinaryOpNode`, `UnaryOpNode`, `FunctionCallNode`, `CaseNode`, etc.).
5. Kept `ValidateExpression` on the same lexer/parser security path by delegating to `ParseExpression`.
6. Added AST assertions for bracket metric refs, bare column refs, qualified column refs, function calls, and CASE expressions.
7. Updated `EXPR_AST_PLAN.md` for completed Phase 2.1, Phase 2.2, and the new Phase 2.3 AST cases.

Left open intentionally:

- The broad existing validation table is still validation-focused, so `Migrate existing expression_parser_test.go tests to new AST types` remains open.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run 'TestParseExpressionProducesSemanticAST|TestValidateExpression' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## EXPR_AST_PLAN Phase 1 Slice

Success criteria:

- `pkg/semantic` has sealed expression AST node types from `EXPR_AST_PLAN.md` Phase 1.1.
- AST nodes JSON marshal with a `type` discriminator and nested nodes unmarshal back to concrete node types.
- Unknown AST node types are rejected.
- Allowed expression function whitelist is available.
- Focused package tests and `git diff --check` pass.

- [x] Add failing AST JSON/whitelist tests in `pkg/semantic`.
- [x] Implement canonical AST types and JSON handling in `pkg/semantic/expr.go`.
- [x] Mark completed Phase 1 checklist items in `EXPR_AST_PLAN.md`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## EXPR_AST_PLAN Phase 1 Results

Resolved:

1. Added `pkg/semantic/expr.go` with sealed expression AST node types for literals, column/metric/dimension refs, binary/unary operations, function calls, and CASE expressions.
2. Added dialect-neutral binary/unary operator constants.
3. Added `AllowedFunctions` whitelist with arity values from the plan.
4. Added discriminator-based JSON marshal/unmarshal support plus `UnmarshalExprNode` for nested interface decoding.
5. Added `pkg/semantic/expr_test.go` covering node round trips, nested expressions, unknown type rejection, and whitelist entries.
6. Updated `EXPR_AST_PLAN.md` for completed Phase 1.1, Phase 1.2, and the duplicate Phase 9 AST unit-test item.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic -run 'TestExprNode|TestUnmarshalExprNode|TestAllowedFunctions' -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./pkg/semantic ./internal/semantic ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Auth Claims Stale Slice

Success criteria:

- The `handler_rbac.go:23` JWT claims item is closed only if source inspection proves the map-claims hot path is gone.
- No benchmark file is added when the compared current path no longer exists.
- Focused auth/middleware tests and `git diff --check` pass.

- [x] Inspect current `handler_rbac.go:23` and JWT claim parsing code.
- [x] Verify auth JWT paths use typed claims instead of `map[string]any` / `MapClaims`.
- [x] Update `GO_PERF_TODO.md` and document the stale-item decision.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Auth Claims Stale Results

Resolved:

1. Verified `internal/auth/handlers/handler_rbac.go:23` is currently `type RBACHandler struct`, not JWT claim decoding.
2. Verified auth token code uses typed `auth.JWTClaims` in `internal/auth/jwt.go`.
3. Verified monolith HTTP JWT middleware uses typed `JWTClaims` plus `jwt.NewParser(...).ParseWithClaims`.
4. No `jwt.MapClaims` current hot path was found under `internal/auth` or `internal/http/middleware`.

Decision:

- Close the stale `handler_rbac.go:23` JWT claims `map[string]any` benchmark item. There is no current map-claims path to benchmark.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/auth ./internal/auth/handlers ./internal/http/middleware -count=1`
- `git diff --check`

## GO_PERF_TODO Readonly Builder Benchmark Slice

Success criteria:

- `security/readonly.go` builder pool idea is measured before any production change.
- Benchmark compares current stack-local `strings.Builder` with a test-only pooled builder alternative.
- `GO_PERF_TODO.md` is updated only from measured evidence.
- Focused security tests/benchmarks and `git diff --check` pass.

- [x] Add focused readonly builder benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs pooled builder.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Readonly Builder Benchmark Results

Resolved:

1. Added `BenchmarkStripSQLLiteralsAndComments` for short, commented, and long SQL inputs.
2. Compared current stack-local `strings.Builder` with a test-only pooled builder variant.
3. Measured current short SQL at roughly 119-123 ns/op, 96 B/op, 1 alloc/op; pooled builder was slower at roughly 137-142 ns/op with the same allocation profile.
4. Measured current commented SQL at roughly 137-143 ns/op, 128 B/op, 1 alloc/op; pooled builder was slower at roughly 157-159 ns/op with the same allocation profile.
5. Measured current long SQL at roughly 3350-3736 ns/op, 3200 B/op, 1 alloc/op; pooled builder was slower at roughly 3772-4001 ns/op and did not reduce allocations.

Decision:

- Keep production `stripSQLLiteralsAndComments` as-is. Builder pooling is not justified.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -run '^$' -bench '^BenchmarkStripSQLLiteralsAndComments$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/security -count=1`
- `git diff --check`

## GO_PERF_TODO Filter Benchmark Slice

Success criteria:

- `compiler_filter.go` `filterHandler` / typed slice concern is measured with Go benchmarks before any production refactor.
- Benchmarks cover current dispatch/direct paths and typed `[]string` / `[]int` helper alternatives.
- `GO_PERF_TODO.md` is updated only from measured results.
- Focused Go tests/benchmarks and `git diff --check` pass.

- [x] Add focused compiler filter benchmarks.
- [x] Run benchmark with `-benchmem` and compare current vs direct/typed alternatives.
- [x] Update `GO_PERF_TODO.md` and this review section from measured evidence.
- [x] Run focused Go tests and `git diff --check`.

## GO_PERF_TODO Filter Benchmark Results

Resolved:

1. Added `BenchmarkCompilerFilterHandler` for current dispatch, direct method calls, and typed `[]string` / `[]int` helper alternatives.
2. Measured `current_dispatch_eq_string_slice` at roughly 281-300 ns/op, 584 B/op, 16 allocs/op; direct/typed alternatives had the same alloc profile and only small timing noise.
3. Measured `current_dispatch_in_any_strings` at roughly 160-240 ns/op, 264 B/op, 8 allocs/op; typed `[]string` helper was worse at 193-205 ns/op, 328 B/op, 12 allocs/op.
4. Measured `current_dispatch_in_any_ints` at roughly 161-177 ns/op, 264 B/op, 8 allocs/op; typed `[]int` helper kept the same allocation profile and did not show a stable improvement.

Decision:

- Keep production `filterHandler` as-is. The benchmark does not justify a generics refactor.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -run '^$' -bench '^BenchmarkCompilerFilterHandler$' -benchmem -count=5`
- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query -count=1`
- `git diff --check`

## GO_PERF_TODO Escape/Decision Slice

Success criteria:

- Escape-analysis subitems are closed only with fresh `go build -gcflags='-m -m'` evidence.
- Existing "do not add" / "small map is okay" items are closed as explicit no-code decisions only after source inspection.
- No speculative pool/dependency changes are introduced.
- Focused Go tests and `git diff --check` pass after tracker edits.

- [x] Inspect compiler args/dimMap and readonly builder source for no-code decision items.
- [x] Run escape-analysis checks for compiler, executor, and readonly builder subitems.
- [x] Update `GO_PERF_TODO.md` for only evidence-backed closed items.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Escape/Decision Results

Resolved:

1. Closed `query/compiler.go` `args []any` as an explicit no-code decision; the checklist already notes pool overhead is higher than one compile-time allocation.
2. Closed `compiler.go:110` `dimMap` as an explicit no-code decision; it is a small per-row-filter lookup map scoped to `buildRowFilterPreds`.
3. Closed `query/executor.go` `columns []ResultColumn` as a no-pool decision. Escape analysis shows `columns` flows into returned `Result`, so pooling would be unsafe unless the result API changes.
4. Checked compiler `[]string` escape state with `go build -gcflags='-m -m' ./internal/query`; remaining slice escapes are in returned SQL/error-building paths, not a standalone safe pooling target.
5. Checked executor `Result` escape state; `&Result`, `Columns`, `Rows`, and each returned row slice escape because they are returned to callers.
6. Checked readonly builder state with `go build -gcflags='-m -m' ./internal/security`; `stripSQLLiteralsAndComments` already calls `Grow(len(sql))`, and Builder write paths inline through `abi.NoEscape`.

Left open intentionally:

- `security/readonly.go` builder pooling still needs before/after benchmark evidence.
- `executor.go` per-row copy allocation remains open; reducing it would need a benchmark-backed result-layout/API change, not a small pool tweak.
- JSON library, JWT claim, `filterHandler` generics, and pprof baseline items remain benchmark/profile work.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## GO_PERF_TODO Continuation

Success criteria:

- Stale/verification-only GO perf items are closed only with code or command evidence.
- Benchmark/dependency/profile items remain open unless measured in this slice.
- Focused Go tests and diff checks still pass after tracker/code edits.

- [x] Verify `provider_store.go` no longer uses `map[string]any` config.
- [x] Run compiler escape-analysis command from `GO_PERF_TODO.md` and record the result.
- [x] Close verified non-code GO perf items and leave benchmark-only items open.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Continuation Results

Resolved:

1. Verified `internal/ai/provider_store.go` is already on typed provider config paths and closed the stale `map[string]any` item.
2. Ran the compiler escape-analysis command from `GO_PERF_TODO.md` and closed the command-level checklist item.
3. Closed cold-path `map[string]any` items for i18n bundles, mail rendering/sending, and auth audit metadata as accepted non-hot-path uses.
4. Closed `internal/http/handlers/helpers.go` as a low-priority error/wrapper path; it has no `fmt.Sprintf`, and remaining `fmt.Errorf` calls wrap auth client errors.

Left open intentionally:

- `filterHandler` generics, JWT claim structs, JSON backend alternatives, pprof profiling, pool decisions, and row-copy allocation items still need benchmark/profile evidence before code changes.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/ai ./internal/query ./internal/core ./internal/security ./internal/http/handlers ./internal/http -count=1`
- `git diff --check`

## GO_PERF_TODO Slice

Success criteria:

- PII masking config can resolve access/type/strategy from a single `ColumnInfo` map.
- Existing `PIIMaskingConfig` map fields remain backward-compatible for current tests/callers.
- `GO_PERF_TODO.md` reflects only verified code changes; benchmark-only items remain open unless measured.
- Focused Go tests pass.

- [x] Add failing coverage for `PIIMaskingConfig.ColumnInfo` lookup.
- [x] Implement single-map PII lookup with compatibility fallback.
- [x] Mark already-implemented compiler select concat escape item as verified in `GO_PERF_TODO.md`.
- [x] Remove verified `fmt.Sprintf` predicate builders in `compiler_filter.go` and `row_injection.go`.
- [x] Run focused Go tests and `git diff --check`, then document results.

## GO_PERF_TODO Slice Results

Resolved:

1. Added `PIIMaskingConfig.ColumnInfo map[string]PIIColumnInfo` and made compiler PII lookup prefer the single map while preserving legacy split-map fallback.
2. Populated `ColumnInfo` in `PIIPolicyService.MaskingConfig`.
3. Removed `fmt.Sprintf` predicate builders from `internal/query/compiler_filter.go` and `internal/security/row_injection.go`.
4. Marked verified GO perf items for PII single-map lookup, executor scan slice pooling, compiler select concat, compiler filter predicates, and row injection predicates.

Left open intentionally:

- JSON library alternatives, pprof, escape-analysis, and pool-decision items still require measurement/benchmark baselines.
- `filterHandler` generics and typed provider/auth config items need separate benchmark-backed slices.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/core ./internal/security -count=1`
- `GOCACHE=/private/tmp/biqly-gocache go test ./... -count=1`
- `git diff --check`

## PII Review Findings Fix Plan

Success criteria:

- Backend rejects or ignores invalid masking strategy values consistently.
- Full masking no longer changes GROUP BY/ORDER BY semantics or permits filters that hidden values would block.
- Frontend lets editable users update masking strategy on reviewed columns while avoiding confusing strategy controls for admin/raw access.
- Focused backend/frontend tests prove the review findings are fixed.

- [x] Verify current failures with focused tests for full-strategy GROUP BY/ORDER BY, filter blocking, and invalid strategy values.
- [x] Fix backend masking semantics with minimal changes and shared strategy constants.
- [x] Add defensive PII-type checks when building column strategy maps.
- [x] Fix `PIIDetectionPanel.tsx` so reviewed columns can update strategy and admin/raw access does not show a misleading strategy dropdown.
- [x] Run focused backend/frontend tests, then update this tracker with results.

## PII Review Findings Results

Resolved:

1. Backend strategy values now normalize through shared constants. Unknown non-empty strategies fail closed to full/hidden behavior.
2. The compiler resolves strategy into one effective access level before SELECT, WHERE, GROUP BY, and ORDER BY decisions.
3. Hidden/full-masked columns are rejected in filters, GROUP BY, and ORDER BY instead of producing constant predicates or aggregates.
4. Column strategy maps are built only for PII-annotated columns.
5. Reviewed PII rows can save strategy changes without dismissing/re-scanning, and raw-access users see a localized note that raw roles still see raw values.
6. Added focused backend regressions and a frontend pure-logic test for reviewed-row strategy saves.

Verification:

- `GOCACHE=/private/tmp/biqly-gocache go test ./internal/query ./internal/security/pii ./internal/core -count=1`
- `npm --prefix frontend run test`
- `npm --prefix frontend run build`
- `git diff --check`

## Previous PII Strategy Work

- [x] Frontend: Add Turkish and English translation keys for `strategy_partial` and `strategy_full`
- [x] Frontend: Implement `pendingStrategy` state and masking strategy dropdown in `PIIDetectionPanel.tsx`
- [x] Frontend: Pass `pii_masking_strategy` in `handleConfirm` to the backend API in `PIIDetectionPanel.tsx`
- [x] Backend: Add `ColumnStrategies` mapping in `PIIMaskingConfig` in `internal/query/pii_masking.go`
- [x] Backend: Update `dimensionOutputSQL` in `internal/query/pii_masking.go` to handle "full" strategy
- [x] Backend: Populate `ColumnStrategies` from database columns in `internal/core/pii_policy.go`
- [x] Verification: Run frontend build and tests
- [x] Verification: Run backend tests

## Previous Review

All tasks are completed successfully:

1. **Frontend**: Translation keys added for English and Turkish. Masking strategy dropdown rendered for unreviewed columns in the PII Detection Panel. Local edits are stored in `pendingStrategy` state and transmitted via `updateColumnPII` upon clicking Confirm.
2. **Backend**: Added column-level strategy mapping in `PIIMaskingConfig` and parsed `PIIMaskingStrategy` from database records. Integrated this strategy in the query compiler: columns with `"full"` strategy resolve to `pii.HiddenLiteral` (full mask) instead of the partial masking expression.
3. **Verification**: Frontend build and tests pass successfully. Backend linter and unit tests (including a new compiler test for strategy overrides) pass successfully.
