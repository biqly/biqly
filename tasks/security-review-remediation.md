# Backend Security Review — Remediation Plan

Full backend scan (2026-07-03). Findings verified by tracing code paths. Fix in priority order; run `gofmt -w`, `make lint-go`, `make test-go` before commit.

## P0 — Critical

- [ ] **#1 Window-fn SQL injection** — `internal/query/validator.go` `validateWindowSelect`: wire `pkgsemantic.ValidateExprStrict` on `w.Expr` (mirror `internal/semantic/publish_validation.go`). Reject non-whitelisted function names before compile.
- [ ] **#2 `AdminForceLogout` no authz** — `internal/auth/handlers/handler_account.go` + `service_sessions.go`: require super-admin (mirror `GenerateMFABypassCode`).
- [ ] **#3 `RestoreAccount` no authz** — same group; require super-admin.
- [ ] **#4 Query engine cross-tenant IDOR** — `internal/http/query_router.go` + `router.go`: wrap `/query/compile|run|explain` with datasource-access check keyed on body `datasource_id`.
- [ ] **#5 Semantic/composite/drift routes unauthorized** — `internal/http/catalog_router.go`: add datasource-access middleware resolving datasource from model id.
- [ ] **#6 Poison-msg loss** — `internal/queue/nats.go` `handleAIJobFailure`: only `Term()` after successful DLQ publish; else `Nak()`.

## P1 — High

- [ ] Workspace metadata/member/datasource IDOR — `internal/auth/handlers/handler_rbac.go` GET routes: add membership/permission guard.
- [ ] Password change/reset doesn't revoke sessions — `service_profile.go`, `service_password.go`: call `RevokeAllUserSessions`.
- [ ] Internal-token hardcoded fallback — `internal/auth/config.go`: fail closed in production.
- [ ] `TableSchemas`/`DefaultSchema` unvalidated — `internal/query/validator.go`: validate against model schemas.
- [ ] `POST /api/datasources` + `test-connection` ungated — `internal/http/catalog_router.go`: add permission guard.
- [ ] `/ai/jobs/stale` full dump — `internal/http/handlers/ai_jobs.go`: require session scoping / move behind admin.
- [ ] PII sample to LLM unmasked — `internal/ai/describe.go`/`sample.go`: exclude/mask PII columns.
- [ ] Dashboard global (NULL-workspace) mutation — `internal/dashboard/repository.go`: drop `OR workspace_id IS NULL` from Update/Delete.
- [ ] Job stuck "running" on shutdown — `internal/http/handlers/ai_job_service.go`: use `context.WithoutCancel` for failure bookkeeping.

## P2 — Moderate

- [ ] `ReadOnlyChecker` missing `INTO`/`OUTFILE`/`DUMPFILE` — `internal/security/readonly.go`.
- [ ] `"cancelled"` vs stdlib `context.Canceled` — `internal/http/handlers/ai_job_service.go`: use `errors.Is`.
- [ ] Mail Subject CRLF injection — `internal/mail/templates.go`/`mime.go`: strip control chars from header values.
- [ ] LIKE metachar escaping — `internal/query/compiler_filter.go`: escape `%`/`_` + `ESCAPE`.
- [ ] Glossary Update/Delete datasource scoping — `internal/metadata/business_glossary.go`.

## Review section — fixes applied 2026-07-03

Verification: `go build ./...`, `make lint-go` (0 issues), `deadcode` (clean for new symbols),
`go test -race` on auth/query/queue/http/dashboard/mail/security/ai/metadata — all pass.

### Fixed
- **P0 #1** window-fn injection → fail-closed at compile sinks: `functionCallSQL` rejects
  functions outside `pkgsemantic.AllowedFunctions`; `unaryExprSQL` rejects unknown operators
  (`internal/query/expr_compiler.go`). Covers all ExprNode paths, not just window.
- **P0 #2/#3** force-logout/restore → super-admin check in `AdminForceLogout`/`RestoreAccount`
  (`service_sessions.go`, `service_account_lifecycle.go`), 403 mapping in handler.
- **P0 #4** query IDOR → `RequireDatasourceAccess` on `/query/compile|run|explain`; body probe
  now reads nested `logical_query.datasource_id` (`query_router.go`, `router.go`, `permission.go`).
- **P0 #5** semantic/composite/drift → new `RequireResolvedDatasourceAccess` middleware resolves
  model/composite/drift-report id → datasource; create/generate gated on body datasource_id
  (`catalog_router.go`, `permission.go`, `drift/repository.go`).
- **P0 #6** poison-msg loss → Nak (not Term) when DLQ publish fails (`queue/nats.go` + test).
- **P1** workspace IDOR → `requireWorkspaceMembership` on the 3 GET routes (`handler_rbac.go`).
- **P1** password change/reset → `RevokeAllUserSessions` after update (`service_profile.go`, `service_password.go`).
- **P1** internal-token fallback → fail closed in production (`auth/config.go`).
- **P1** `TableSchemas`/`DefaultSchema` → validated against model base/join schemas (`validator.go`).
- **P1** datasources create/test-connection → gated on `datasource:create` (`catalog_router.go`).
- **P1** `/ai/jobs/stale` → require `client_session_id`; unscoped view stays admin-only (`ai_jobs.go`).
- **P1** PII sample to LLM → exclude PII-typed columns from describe sample (`ai/describe.go`).
- **P1** dashboard global mutation → drop `OR workspace_id IS NULL` from Update/Delete (`dashboard/repository.go`).
- **P1** job stuck "running" → failure bookkeeping uses `context.WithoutCancel`; treat
  `context.Canceled/DeadlineExceeded` as redeliver-not-fail (`ai_job_service.go`).
- **P2** `INTO`/OUTFILE/DUMPFILE added to read-only blocklist (`security/readonly.go`).
- **P2** mail Subject CRLF injection → `sanitizeHeaderValue` strips control chars (`mail/mime.go`).
- **P2** glossary Update/Delete → resolved datasource access; create gated on body datasource_id (`ai_router.go`, `business_glossary.go`).

### Deferred (documented, not fixed — reason)
- **P2** LIKE metachar escaping — needs per-dialect `ESCAPE` clause; cross-dialect risk, correctness not security.
- **P2** i18n cache refresh holds mutex over DB round-trip — refactor to atomic snapshot swap.
- **P1(rel)** AckWait(30m) < job deadline(35m) — raise AckWait or add `msg.InProgress()` heartbeat.
- Full resume-after-shutdown for in-flight jobs needs a `running→queued` reclaim (repo method or
  AckWait-driven stale reclaim); current fix stops spurious failure records but a job interrupted
  mid-flight still requires the stale-jobs path.
- Proxy-mode (`Services.AIURL`/`CatalogURL` set) datasource guarding for glossary/examples — the
  in-process path is guarded; proxy edge (`aiProxyDatasourceGuardedPaths`) can't do resolved-id checks
  without repo access. Not exercised by the current single-monolith deployment.
