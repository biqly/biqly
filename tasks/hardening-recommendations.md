# Hardening Recommendations — Progress

Follow-up to the backend/frontend reviews. Status of the 7 systemic recommendations.

- **#1 Read-only DB guarantee** — DONE. Postgres/MySQL queries run inside a `ReadOnly` tx
  (`internal/query/executor.go`, driver `SupportsReadOnlyTx()`); DB rejects writes that pass the
  regex checker. Analytical engines keep prior behavior. Tests added.
- **#2 Internal-route isolation** — VERIFIED gap, fix deferred (infra). Only auth/mail have ingress
  NetworkPolicies; no namespace default-deny → monolith `/internal/*` is L4-open cluster-wide.
  Mitigated at L7 (internal token now fails closed in prod). Full fix = per-service NetworkPolicies
  with correct egress + default-deny; risky without canary, do deliberately.
- **#3 Per-workspace AI spend cap** — DONE. Redis-backed `SpendLimiter`
  (`BI_AI_WORKSPACE_DAILY_TOKEN_BUDGET`, 0=off); 429 on exceed; fails open on Redis error. Tests + docs.
- **#4 Tenant-isolation tests** — DONE. Regression tests for readonly-INTO, window-fn injection,
  TableSchemas override, nested datasource_id extraction, RequireResolvedDatasourceAccess,
  dashboard global-mutation.
- **#5 LLM PII egress** — DONE. Central `ai.ExcludePIIColumns` withholds PII column *values* from
  both describe and the NL→SQL sample-row prompt (a live leak). Test added.
- **#6 Deploy / DR** — ANALYSIS (below).
- **#7 Alerting** — DONE (rules). Added `BiqlyServiceDown` (up==0 backstop), `BiqlyAuthFailedLoginSpike`,
  `BiqlyAISpendLimitRejections` (+ new counter). Secret-rotation runbook still open.

## #6 — Deploy / DR findings (read-only analysis, nothing changed)

### Confirmed
1. **No backup/DR for the primary Postgres, anywhere in the repo.** Live `biqly` namespace has no
   Postgres pod (only ai/auth/catalog/query/worker/mail/frontend/dragonfly/nats/otel) — Postgres is
   external/self-managed, reached via `postgresql-vip` (a LoadBalancer Service). There is **no
   CloudNativePG `Cluster`, no `ScheduledBackup`, no `barmanObjectStore`, no PITR** config in the
   chart. Backup, retention, and — critically — **restore-testing of the metadata/auth/mail data is
   unverified and out of this repo's control.** This is the highest-priority DR gap.
   - Action: confirm where the external Postgres is backed up, its RPO/retention, and that a restore
     has actually been exercised. If it's self-managed with no automation, add scheduled logical/PITR
     backups and a documented, tested restore.

2. **Migrations are applied as a Helm `pre-install,pre-upgrade` hook** (`templates/migrate-job.yaml`,
   weight -10) — they run *before* new pods roll. During a rolling upgrade the **old** pods keep
   serving against the already-migrated schema, so every migration must be backward-compatible
   (expand/contract). Down migrations exist (91 up / 91 down = reversible), but nothing enforces
   backward-compatibility: a destructive migration (DROP/RENAME column) would break the still-running
   old pods during the rollout window → errors/partial downtime, and the pre-hook nature means a
   failed migration blocks the deploy (good) but a *successful-but-incompatible* one causes silent
   breakage.
   - Action: adopt an explicit expand/contract convention (add nullable → backfill → switch reads →
     drop in a later release), and consider a CI check that flags destructive DDL in a single release.

3. **Rolling deploy with no canary / auto-rollback / GitOps** (known; `helm upgrade --install`
   manual/CI). Combined with #2 above (job-polling transient failures) this means a bad image has no
   automatic safety net.
   - Action: a canary/analysis step (the repo already ships Argo Rollouts `analysis_templates.yaml`
     for AI — extend the pattern) or at least an automated post-deploy health gate + `helm rollback`.

### Not blocking, noted
- `deadcode`/lint gates are strong; CI mirrors prod gate (`make verify-main`). Migration reversibility
  is in good shape structurally; the risk is process (expand/contract discipline), not missing downs.
