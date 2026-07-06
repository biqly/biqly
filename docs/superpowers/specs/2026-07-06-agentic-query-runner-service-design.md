# Agentic Query Runner Service — Design

Date: 2026-07-06
Status: Approved in conversation; implementation plan pending
Supersedes: the runtime-placement decision in
`docs/superpowers/specs/2026-07-06-agentic-runtime-design.md`

## Goal

Add a separately deployed, internal-only `cmd/agent` service that runs a bounded LLM
`plan → tool → observe` loop for the read-only BI query lifecycle while preserving the
existing public `/api/ai/*` contract and legacy pipeline during a staged rollout.

The first release is an **Agentic Query Runner**, not a general autonomous agent platform.
It accepts a text question, resolves governed semantic context, optionally asks for
clarification, produces and validates a query plan, executes a read-only query through the
Query service, explains the result, and persists every decision and tool step.

This program also fixes the existing conversation snapshot replay bug and provides a
reversible, evidence-based repair path for duplicates already persisted in production.

## Existing capabilities to reuse

The following shipped capabilities remain the foundation:

- asynchronous AI jobs over NATS JetStream;
- persistent `agent_runs` and `agent_steps`;
- clarification policy and clarification resume behavior;
- semantic models, metrics, dimensions, joins, PII policy, row filters, and query compiler;
- query validation and read-only execution;
- memory, saved queries, glossary, feedback, eval regression, and run trace UI;
- Catalog, Query, AI, Auth, API/BFF, worker, NATS, Dragonfly, PostgreSQL, and OTEL
  deployments;
- existing public `/api/ai/query/*`, `/api/ai/jobs/*`, and conversation APIs.

This design moves the agentic orchestration boundary into a service. It does not rebuild the
existing BI pipeline or create a general tool virtual machine.

## Locked decisions

1. **Runtime shape:** one supervisor LLM agent with typed tools. There are no specialist
   agents and no agent-per-service network.
2. **Service boundary:** `cmd/agent` is an internal Kubernetes service. Frontend and MCP keep
   calling the API/BFF.
3. **Scope:** text question → semantic interpretation → SQL → table result → short
   explanation. Dashboard creation, charts as tools, PDF export, write-back, data correction,
   schema migration, and agent-to-agent work are excluded.
4. **Safety:** the agent cannot access customer databases directly. Query execution always
   passes through Query service validation and policy enforcement.
5. **Rollout:** shadow → workspace allowlisted beta → default agent with controlled legacy
   fallback.
6. **Loop bounds:** maximum six tool steps and maximum two clarification rounds per run.
7. **Evaluation:** shadow comparison is outside the agent tool registry. The agent cannot
   invoke or alter its own evaluator.
8. **Conversation repair:** fix future writes with stable client identities and transactional
   idempotency; repair historical rows only when an ordered-prefix replay chain is proven.
9. **Deletion:** repair first archives and soft-deletes. Physical purge is a separate,
   explicitly invoked operation after observation.

## Program decomposition

The work is delivered in three independently verifiable slices:

1. **Conversation Idempotency and Repair**
   - stop snapshot replay from creating new message rows;
   - add dry-run, report, archive, soft-delete, restore, and later purge workflows.
2. **Agentic Query Runner Runtime**
   - add the bounded planner, policy engine, typed tools, event contracts, feature router,
     shadow evaluator, and `cmd/agent`.
3. **Production Integration**
   - add image build, Helm subchart, NetworkPolicies, local developer loop, observability,
     CI gates, staged rollout configuration, and operational documentation.

Each slice must leave the legacy query path working and must pass its focused tests before the
next slice begins.

## Request routing and rollout

### Public boundary

Frontend and MCP continue using the current API/BFF endpoints. The API/BFF retains:

- authentication and workspace/user identity;
- datasource and semantic-model authorization;
- rate limits and spend limits;
- request normalization;
- public response compatibility;
- AI job creation and polling semantics.

No Gateway `HTTPRoute` is created for Agent.

### Normalized execution request

The API/BFF normalizes both legacy and agent execution into the same internal request:

```json
{
  "schema_version": "agent-job.v1",
  "job_id": "job_...",
  "request_id": "req_...",
  "workspace_id": "workspace_...",
  "user_id": "user_...",
  "datasource_id": "datasource_...",
  "semantic_model_id": "model_...",
  "conversation_id": "conv_...",
  "question": "What were monthly sales this year?",
  "locale": "en",
  "mode": "shadow",
  "permissions": {
    "datasource_access": true
  },
  "limits": {
    "max_steps": 6,
    "max_clarification_rounds": 2,
    "timeout_seconds": 45,
    "max_rows": 1000
  }
}
```

The payload never carries credentials or unrestricted SQL.

### Routing stages

#### Stage 1 — Shadow

- The legacy pipeline remains authoritative and its answer is returned.
- Agent receives the same normalized request and runs independently.
- Shadow Agent cannot cause a second customer query execution. Its Query Execute tool runs in
  explain/dry-run mode or uses the already captured legacy result where comparison requires
  rows.
- The evaluator compares SQL/LogicalQuery equivalence, result similarity, latency,
  clarification behavior, policy decisions, confidence, and failure class.
- Agent output and comparison are persisted but not shown as the answer.

#### Stage 2 — Allowlisted beta

- Agent is authoritative only for explicitly allowlisted workspaces.
- A failure may fall back to legacy only before Query Execute starts.
- Once Query Execute starts, the same request is never submitted to the legacy path.
- Non-allowlisted workspaces remain on legacy.

#### Stage 3 — Default Agent

- Agent is authoritative by default.
- Legacy remains a pre-execution fallback and emergency feature-flag rollback path.
- Disabling Agent routes new jobs to legacy without stopping Agent pods or mutating active
  runs.

The rollout mode and allowlist live in dynamic AI runtime configuration. Environment variables
are deployment fallbacks, not the primary control plane.

## Agent runtime

### State machine

```text
queued
  → running
  → planning
  → policy_check
  → tool_running
  → observing
  → planning | waiting_clarification | completing
  → completed | failed | cancelled
```

Every transition is persisted. A run resumes from persisted state after clarification or
consumer redelivery.

### Planner

The planner receives:

- the normalized user request;
- approved semantic context;
- bounded memory recall;
- the current run state and prior observations;
- a machine-readable tool catalog;
- remaining step, clarification, time, token, and cost budgets.

The planner can only return:

- a typed tool proposal;
- a clarification request;
- a final response proposal;
- a terminal failure.

Free-form SQL or network destinations are never treated as executable instructions.

### Bounds

Default runtime limits:

```text
max_steps = 6
max_clarification_rounds = 2
timeout_seconds = 45
max_rows = 1000
retry_budget_per_tool = 1
dangerous_sql = reject
every_step_persisted = true
```

The effective limit is the minimum of system policy, workspace policy, and request limit.

### Confidence and clarification

- High confidence: execute the governed plan.
- Medium confidence: execute only when a safe default exists; state the assumption in the
  answer.
- Low confidence: request clarification.
- Missing metric, tied metric candidates, ambiguous segment, materially different
  interpretations, or absence of a safe default force clarification.
- A third clarification attempt terminates with a bounded explanation instead of looping.

## Policy Engine

Every planner proposal passes through the Policy Engine before tool execution. The Policy
Engine is deterministic and cannot be bypassed by prompt output.

It verifies:

- workspace, user, datasource, model, table, and column access;
- tenant isolation;
- semantic join-path validity;
- row-filter injection;
- hidden-column exclusion;
- PII masking;
- read-only SQL AST;
- statement count and forbidden constructs;
- maximum rows and query timeout;
- query cost/dry-run policy where supported;
- prompt-injection and tool-argument policy;
- per-tool retry, token, cost, and duration budgets;
- deployment-mode egress restrictions.

Denied proposals are persisted as policy-denied steps. The planner may produce one corrected
proposal if budget remains; it cannot weaken the policy.

## Tool registry

All tools have versioned, typed request and response schemas, explicit timeouts, and stable
error classes.

### Catalog Tool

Returns accessible schema, table, column, relationship, metric, and dimension metadata. It
prefers approved semantic models over raw datasource introspection.

It never returns hidden objects or credentials.

### Semantic Model Tool

Maps the question onto governed metrics, dimensions, filters, grains, and join paths. It
returns a LogicalQuery-compatible semantic plan plus confidence and assumptions.

### Query Compile Tool

Compiles and validates the semantic plan through Query service. It returns sanitized SQL,
parameters, validation metadata, and estimated execution policy. It does not execute SQL.

### Query Execute Tool

Executes only a previously approved compile result through Query service. Query service
enforces read-only access, single statement, tenant filters, PII masking, hidden-column
blocking, row limit, timeout, and dialect validation.

Agent has no customer-database egress.

### Memory Tool

Recalls only approved items:

- confirmed metric mappings;
- user clarification answers;
- accepted saved queries;
- successful SQL/result exemplars approved for reuse;
- workspace glossary entries.

Raw conversation text is not automatically promoted to durable memory.

### Final Response Builder

Builds the user-facing answer from the governed result, assumptions, caveats, confidence, and
persisted run trace. It cannot introduce new data or execute another tool.

## NATS contracts

Subjects:

```text
biqly.ai.jobs
biqly.ai.agent.jobs
biqly.ai.agent.steps
biqly.ai.agent.results
biqly.ai.agent.errors
```

Event schema versions:

```text
agent-job.v1
agent-step.v1
agent-result.v1
agent-error.v1
```

Delivery is at-least-once. Idempotency keys:

- jobs: `job_id`;
- runs: `run_id`;
- steps: `(run_id, seq)`;
- terminal result/error: `(run_id, terminal_version)`.

Redelivery resumes persisted state and does not repeat a completed tool step. Event consumers
reject unsupported major schema versions and record a permanent contract error.

The existing AI jobs API remains the frontend source of truth. Step events support live trace
streaming, while persisted run and job rows support polling and reload.

## Shadow evaluator

The evaluator is not an Agent tool. It consumes persisted legacy and shadow outcomes and
records:

- semantic-plan and LogicalQuery equivalence;
- normalized SQL equivalence;
- result-set similarity;
- latency and LLM token/cost delta;
- clarification agreement;
- policy denial and unsafe-proposal counts;
- missing or excessive join paths;
- PII and row-filter policy agreement;
- failure taxonomy.

Promotion from shadow to beta requires an explicit report and approval. No automatic metric
threshold changes the rollout mode.

## Conversation idempotency

### Message identity

`ai_conversation_messages` keeps its backend-generated primary key and gains client identity:

```text
id                    backend UUID primary key
conversation_id       parent conversation
remote_id             client-generated stable message identity
ordinal               position in the client conversation snapshot
role                  user | assistant | tool
content               message content
ai_response           structured response
result_summary        structured summary
created_at            original creation time
updated_at            last permitted update time
deleted_at            nullable soft-delete timestamp
deleted_by_repair_run_id
```

Required constraint:

```sql
UNIQUE (conversation_id, remote_id)
```

Frontend generates `remote_id` before optimistic insertion and preserves remote IDs returned by
the API. Backend `id` remains internal and is not used as the replay identity.

### Conversation version

`ai_conversations` gains a monotonically increasing `snapshot_version`. A write supplies its
expected version. A stale version returns `409 Conflict` with the current version; it never
blindly overwrites newer conversation state.

### Request identity

Conversation writes include an `Idempotency-Key`. The backend stores:

```text
conversation_write_requests
  idempotency_key
  user_id
  conversation_id
  payload_hash
  status
  response_status
  response_body
  created_at
  completed_at
```

Reusing the same key with the same payload returns the stored result. Reusing it with a
different payload returns `409 Conflict`.

### Atomic write

One database transaction:

1. verifies user/workspace ownership;
2. locks the conversation row;
3. validates `snapshot_version`;
4. reserves or reads the idempotency request;
5. upserts conversation metadata;
6. upserts messages by `(conversation_id, remote_id)`;
7. increments the conversation version;
8. stores the response;
9. commits.

Any failure rolls back the whole snapshot.

### Message immutability

- Same `remote_id` and same canonical payload hash: no-op.
- User drafts may change only through a distinct edit operation.
- Final user, assistant, and tool messages are immutable.
- Same `remote_id` with different immutable content returns `409 Conflict`.
- The backend never silently rewrites a finalized assistant/tool response.

## Historical conversation repair

### Repair data

The repair migration adds:

```text
conversation_repair_runs
  id
  mode
  status
  candidate_count
  repaired_count
  skipped_count
  canonical_hash
  report_json
  created_at
  completed_at

conversation_message_repair_archive
  repair_run_id
  original_message_id
  conversation_id
  remote_id
  role
  content
  content_hash
  ordinal
  created_at
  full_row_json
  archived_at
```

Active reads exclude `deleted_at IS NOT NULL`.

### Commands

```text
conversation-repair detect --dry-run
conversation-repair report --conversation-id <id>
conversation-repair archive --run-id <id>
conversation-repair apply --run-id <id>
conversation-repair restore --run-id <id>
conversation-repair purge --run-id <id>
```

`detect` is read-only. `apply` requires the dry-run identifier and canonical hash. `purge` is
never part of automatic deployment.

### Batch inference

Legacy rows lack request and remote IDs. The repair command:

1. orders rows by `created_at` and `id`;
2. groups rapid sequential inserts into candidate POST batches using a fixed 250 ms boundary;
3. assigns ordinal within each candidate batch;
4. computes a canonical message hash from role, normalized content, canonical JSON response,
   result summary, tool name/arguments where present, ordinal, and run/model provenance;
5. verifies that every earlier batch is an exact ordered prefix of the next batch;
6. requires a chain of at least two batches and a unique longest final batch.

The final batch is the canonical snapshot. Earlier proven-prefix batches are replay copies.

### Automatic apply requirements

All conditions must hold:

- batch boundaries are unambiguous;
- batches form a monotonic exact ordered-prefix chain;
- role and ordinal match;
- final batch covers every prior batch;
- no partial POST or error trace is detected;
- no differing tool/result/run/model provenance exists;
- no candidate row is pinned, starred, exported, or directly referenced by feedback;
- the dry-run canonical hash still matches under `SELECT ... FOR UPDATE`;
- archive rows are written in the same transaction;
- conversation did not change after detection.

If any condition fails, the conversation is reported and not changed.

### Apply and rollback

`apply` archives the full original rows and sets `deleted_at` plus
`deleted_by_repair_run_id`. It does not physically delete rows.

`restore` clears the soft-delete markers after verifying the archive. Physical purge is a
separate manual operation after observation and backup validation.

## `cmd/agent` service

### Responsibilities

- consume `biqly.ai.agent.jobs`;
- create/resume Agent runs;
- execute the bounded planner and policy loop;
- call internal tools;
- publish step/result/error events;
- update the existing AI job store;
- expose health, readiness, metrics, and internal diagnostic endpoints;
- shut down gracefully without losing an acknowledged job.

### Internal HTTP

Port: `8084`.

Endpoints:

```text
GET  /healthz
GET  /readyz
GET  /metrics
GET  /internal/agent/runs/{id}
POST /internal/agent/runs/{id}/cancel
```

Internal endpoints require the existing internal peer token. There is no public route.

### Configuration

Environment fallback keys:

```text
BI_AGENT_ENABLED
BI_AGENT_MODE
BI_AGENT_MAX_STEPS
BI_AGENT_MAX_CLARIFICATION_ROUNDS
BI_AGENT_TIMEOUT_SECONDS
BI_AGENT_MAX_ROWS
BI_AGENT_NATS_SUBJECT
BI_AGENT_NATS_STEP_SUBJECT
BI_AGENT_NATS_RESULT_SUBJECT
BI_AGENT_NATS_ERROR_SUBJECT
BI_AGENT_WORKSPACE_ALLOWLIST
BI_AGENT_LEGACY_FALLBACK_ENABLED
```

Provider, database, Redis, Auth, internal-token, service URL, tracing, deployment-mode, and
logging configuration reuse existing keys.

## Kubernetes and Helm

### Subchart

Create `deploy/helm/biqly/charts/agent` and add it to the umbrella chart behind
`agent.enabled`.

Resources:

- `Deployment`;
- internal `ClusterIP` Service on port 8084;
- ConfigMap;
- helpers;
- HPA;
- PodDisruptionBudget when replicas exceed one;
- chart tests for health connectivity.

The workload follows existing Biqly hardening:

- pinned SHA image in production;
- non-root UID/GID 65532;
- read-only root filesystem;
- no privilege escalation;
- all Linux capabilities dropped;
- runtime-default seccomp;
- service account token not mounted;
- resource requests and limits;
- readiness/liveness probes;
- graceful termination;
- config and secret checksum rollout annotations;
- OTEL service name `biqly-agent`.

Initial resources:

```yaml
requests:
  cpu: 250m
  memory: 256Mi
limits:
  cpu: "2"
  memory: 1Gi
```

HPA starts with two replicas in production and scales on CPU plus available custom Agent queue
metrics when configured.

### NetworkPolicy matrix

Agent ingress:

| Source | Destination | Port | Purpose |
| --- | --- | --- | --- |
| API/BFF pods | Agent | TCP/8084 | internal run diagnostics/cancel |
| approved monitoring path | Agent | TCP/8084 | metrics scrape |

Agent egress:

| Destination | Port | Purpose |
| --- | --- | --- |
| kube-dns | UDP/TCP 53 | service discovery |
| NATS | TCP 4222 | jobs and events |
| shared PostgreSQL | TCP 5432 | run/job/conversation state |
| Dragonfly | TCP 6379 | bounded cache/budgets |
| Catalog | TCP 8080 | catalog tool |
| Query | TCP 8081 | compile and execute tools |
| AI | TCP 8082 | bounded semantic/generation operations |
| Auth | TCP 8889 | internal identity/preferences when required |
| OTEL collector | TCP 4317/4318 | traces/log export |
| approved LLM endpoints | TCP 443 | supervisor inference in cloud/private modes |

Agent receives no direct customer-datasource CIDR egress.

The existing Catalog, Query, AI, Auth, shared PostgreSQL, OTEL, DNS, NATS, and Dragonfly
policies are updated symmetrically to admit Agent only on the listed ports. In airgapped mode,
external HTTPS is denied and provider endpoints must be private/in-cluster.

### Values and deployment

Update:

- umbrella `Chart.yaml`, lock/dependencies, `values.yaml`, `values-dev.yaml`, and
  `values-prod.yaml`;
- shared PostgreSQL component selectors;
- Agent internal and external CiliumNetworkPolicies;
- image tag bump script;
- migration ordering so schema exists before Agent starts.

Existing uncommitted Query route changes are preserved and are not part of this program.

## CI/CD

### Build

- `ci.yml` already builds all `cmd/*` targets; `cmd/agent` joins automatically.
- Add `.github/workflows/build-agent.yml`.
- Build `ghcr.io/biqly/agent` for linux/arm64 with immutable `sha-<commit>` and default-branch
  `latest`.
- Trigger on `cmd/agent/**`, Agent runtime packages, shared relevant packages, Go module files,
  Dockerfile, and workflow changes.
- Add Agent to `scripts/helm-bump-tags.sh`.

### Required gates

- Agent planner, state machine, policy, tool, retry, cancellation, and event-contract tests;
- race-enabled Go suite;
- conversation idempotency transaction tests;
- repair detector property tests and archive/restore integration tests;
- stub-provider Agent eval regression;
- shadow comparator golden cases;
- no test may require live NATS, Redis, or PostgreSQL in the default unit suite;
- Docker build;
- Helm dependency update, lint, dev/prod render;
- assertions that Agent has no public route or customer-datasource egress;
- existing `make verify-main`.

## Local development

- Add `agent` to the default `WATCH_SVCS`.
- Support `make watch SVC="api agent auth"` and `make debug-watch SVC=agent`.
- Add `.env.dev.example` Agent settings and localhost service URLs.
- Host Agent uses the Dockerized PostgreSQL, Dragonfly, and NATS from `make dev-up`.
- Add Agent to the full containerized Compose profile.
- Document the loop in `docs/agents/local-dev.md`.

## Observability

### Tracing

Propagate trace and request IDs across:

```text
API request → NATS publish → Agent run → policy → tool call → Query execution → result event
```

Every Agent step and tool span includes `run_id`, `job_id`, `workspace_id`, tool name,
attempt, status, and duration. Questions, SQL, credentials, result rows, and sensitive tool
arguments are not emitted as unrestricted metric labels or logs.

### Metrics

- Agent runs by terminal state;
- run and step duration;
- tool call duration/errors/retries;
- policy denials by bounded reason;
- clarification count;
- loop exhaustion;
- legacy fallback count;
- shadow divergence by bounded category;
- queue pending/redelivery;
- LLM token and cost totals;
- cancellation and timeout counts.

### Alerts

- sustained Agent failure or timeout rate;
- queue backlog and redelivery growth;
- policy-denial spike;
- shadow divergence regression;
- missing result events;
- abnormal token/cost growth;
- no ready Agent replicas during beta/default mode.

## Security and failure behavior

- Agent tools are allowlisted and schema-validated.
- Internal peer authentication is mandatory.
- Workspace and user identity are never inferred from model output.
- Policy failures fail closed.
- External provider failures follow bounded retry and deployment-mode policy.
- NATS redelivery is idempotent.
- Cancellation is persisted and checked between tool steps.
- Graceful shutdown stops accepting new work and finishes or safely releases the active
  message.
- A terminal Agent result is immutable.
- Legacy fallback never runs after Query Execute begins.

## Success criteria

### Conversation idempotency

- Posting the same snapshot ten times creates one row per `remote_id`.
- A stale snapshot returns `409` without changing stored state.
- A retried idempotency key returns the stored response.
- Existing proven replay chains can be dry-run, archived, soft-deleted, restored, and
  reported.
- Ambiguous histories are not modified.

### Agent runtime

- Shadow mode never changes the answer returned to the user.
- Agent runs are fully reconstructable from persisted runs and steps.
- Every tool proposal passes through Policy Engine.
- Agent cannot execute writes or connect directly to a customer datasource.
- Clarification and loop limits terminate deterministically.
- NATS redelivery does not repeat completed steps.

### Production integration

- Agent image is built and tagged in CI.
- Helm lint and dev/prod rendering pass.
- NetworkPolicy rendering grants only the documented flows.
- Agent is dark-deployable before routing traffic.
- Shadow reports support an explicit beta promotion decision.
- `make verify-main` passes.

## Out of scope

- specialist or peer AI agents;
- general service task distribution;
- arbitrary tool/plugin loading;
- chart, dashboard, PDF, email, or write-back tools;
- DDL or schema migration initiated by Agent;
- direct Agent access to customer databases;
- public Agent endpoint;
- automatic rollout promotion;
- immediate hard deletion of repaired conversation rows.
