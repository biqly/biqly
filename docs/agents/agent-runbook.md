# Agentic query runner — operations runbook

Operational reference for `cmd/agent` (the NATS-driven planner/tool-execution
pipeline in `internal/agent/`) and `cmd/conversation-repair` (the CLI that
cleans up replay-chain duplicate messages the agent's rollout depends on
being fixed first). See `docs/superpowers/plans/2026-07-06-agentic-query-runner-service.md`
for the design; this doc is the day-2 reference once it's deployed.

## Service ownership

`cmd/agent` is a standalone deployable (chart `deploy/helm/biqly/charts/agent` in the biqly-gitops repo,
`agent.enabled: false` by default in every values file). It owns exactly one
thing: the bounded planner/tool loop in `internal/agent/runtime.go`. It has
no public HTTPRoute and no direct customer-database egress — only
`GET /healthz`, `GET /readyz`, `GET /metrics`, and the internal-token-gated
`GET /internal/agent/runs/{id}` / `POST /internal/agent/runs/{id}/cancel`.
It talks to catalog/AI/query over HTTP like any other client (see
`internal/app/agent_dependencies.go`), never in-process.

## Operational controls

All of the following are `BI_AGENT_*` env vars (see `docs/configuration.md`'s
"Agentic Query Runner" table for the full list with defaults) — none are
runtime-DB-overridable yet, unlike ambiguity/PII/memory settings. Changing
any of them means a Helm value change + rollout, not an admin-panel toggle:

- **Mode**: `BI_AGENT_MODE` — `shadow` (compute for comparison, never shown
  to the user) or `active` (the agent's result reaches the user). Always
  start a rollout in `shadow`.
- **Allowlist**: `BI_AGENT_WORKSPACE_ALLOWLIST` — comma-separated workspace
  IDs; empty means all workspaces. Narrow this before widening `BI_AGENT_MODE`.
- **Fallback boundary**: `BI_AGENT_LEGACY_FALLBACK_ENABLED` (default `true`)
  governs `internal/http/handlers/ai_job_service.go`'s `shouldFallbackToLegacy`
  — but as of this writing that function is **not yet called by `Enqueue`**
  (a deliberate Task 10 deferral: no consumer existed yet for agent-routed
  jobs, and wiring it early risked stranding jobs). Until it is wired in,
  `BI_AGENT_ENABLED`/`BI_AGENT_MODE` are the only real traffic controls —
  don't assume the fallback env var does anything in production yet.
- **Bounds**: `BI_AGENT_MAX_STEPS` (≤6), `BI_AGENT_MAX_CLARIFICATION_ROUNDS`
  (≤2), `BI_AGENT_TIMEOUT` (≤45s), `BI_AGENT_MAX_ROWS` (≤1000) — these are
  hard validated ceilings in `internal/config/config.go`, not soft defaults.
- **Airgapped behavior**: `BI_DEPLOYMENT_MODE=airgapped` denies any tool in
  `RunContext.ExternalEgressTools`. Today that map is always empty — every
  agent tool calls an in-cluster BI service over HTTP, not a third party
  directly — so airgapped mode does not currently deny any agent tool call.
  LLM/embedding egress is still enforced inside the AI service itself
  (`providerpkg.SetAirgapped`), not re-checked in the agent's policy engine.

## Metrics

`GET /metrics` on the agent pod exposes, alongside the standard process
metrics, the bounded-label agent metrics added in
`internal/platform/observability/tier2_metrics.go`:

| Metric | Labels | What it tells you |
| --- | --- | --- |
| `biqly_agent_runs_total` | `outcome` (`completed`/`failed`) | Run-level success rate |
| `biqly_agent_terminal_failures_total` | `reason` | Why runs failed (loop exhaustion, timeout, tool_error, planner_error, …) |
| `biqly_agent_step_duration_seconds` | `tool` | Per-tool dispatch latency (policy eval + tool call) |
| `biqly_agent_policy_denials_total` | `reason` | Which policy rule fired and how often |
| `biqly_agent_clarification_rounds_histogram` | — | Distribution of clarification rounds reached |
| `biqly_agent_shadow_comparisons_total` | `category` | Shadow-mode divergence from the legacy pipeline — the primary promotion gate |
| `biqly_agent_queue_redeliveries_total` | — | NATS redeliveries / crash-recovery resumes |
| `biqly_agent_planner_tokens_total` | `kind` (`prompt`/`completion`) | Planner LLM token spend |

Every label is bounded via `observability.BoundLabel` against a fixed value
set (unknown values fall back to a documented bucket like `"other"` or
`"tool_error"`) — never the run's question, SQL, credentials, or result rows.

## Alerts

biqly-gitops `deploy/helm/biqly/templates/prometheus-rules.yaml`'s `biqly.agent` group:
`BiqlyAgentTerminalFailureRateHigh` (>10% failed runs over 10m, warning),
`BiqlyAgentSecurityPolicyDenialSpike` (any `prompt_injection_suspected` or
`identity_mismatch` denial in 5m, critical), `BiqlyAgentShadowDivergenceHigh`
(>30% non-match shadow comparisons over 15m, warning — check before
promoting out of shadow mode), `BiqlyAgentQueueRedeliverySpike` (sustained
redeliveries over 10m, warning — usually means the process is crashing
mid-run). `BiqlyServiceDown` (generic, `biqly.infrastructure`) already
covers the agent pod via its `up{job=~"biqly-.*"}` selector.

## Conversation replay repair (`cmd/conversation-repair`)

A prerequisite for the agent's persistent-run tracking: historical
conversations could accumulate duplicate messages from repost/replay races.
The CLI (`BI_METADATA_DB_DSN` required) is read-only by default; every
mutation requires an explicit `--run-id`:

```sh
conversation-repair detect --dry-run                       # scan all conversations, report only
conversation-repair report  --conversation-id <id>          # inspect one conversation's replay chain
conversation-repair archive --run-id <id>                   # show archive status for a prior detect run
conversation-repair apply   --run-id <id>                   # soft-delete the proven duplicate rows
conversation-repair restore --run-id <id>                   # clear soft-delete markers (undo apply)
conversation-repair purge   --run-id <id> --confirm-purge <id>   # physically delete archived rows — irreversible
```

**Restore procedure**: `apply` only sets soft-delete markers — the rows
still exist. Run `conversation-repair restore --run-id <id>` to clear those
markers and bring the rows back exactly as they were, with no data loss.
Only `purge` is destructive, and it requires `--confirm-purge` to echo the
same `--run-id` back — a deliberate double-confirmation for the one command
that cannot be undone. Always run `detect --dry-run` and `report` first and
review the JSON output before ever calling `apply`, and never call `purge`
without a verified `restore`-tested rollback plan.
