# Agentic Query Runner Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop conversation snapshot duplication, safely repair existing replay rows, and ship an internal-only Agentic Query Runner that executes a bounded, policy-gated BI query loop behind shadow/beta/default feature routing.

**Architecture:** Preserve the public `/api/ai/*` contract. The API/BFF normalizes and routes jobs to either the legacy NATS subject or the new Agent subject. `cmd/agent` consumes Agent jobs, persists a resumable run, proposes typed tool calls, sends every proposal through a deterministic Policy Engine, calls existing Catalog/AI/Query services, and publishes persisted step/result/error events. Conversation writes become transactionally idempotent through client `remote_id`, snapshot version, and request idempotency.

**Tech Stack:** Go 1.26, PostgreSQL, NATS JetStream, React 19/TypeScript, Helm, CiliumNetworkPolicy, Prometheus/OTEL, GitHub Actions.

**Source spec:** `docs/superpowers/specs/2026-07-06-agentic-query-runner-service-design.md`

---

## File structure

### Conversation idempotency and repair

- Create `migrations/063a_conversation_idempotency.up.sql` and `.down.sql` — message remote identity, snapshot version, idempotency request ledger.
- Create `migrations/064a_conversation_repair.up.sql` and `.down.sql` — repair runs, archive, soft-delete metadata.
- Modify `pkg/metadata/types.go` — wire fields for conversation/message identity.
- Modify `internal/metadata/model.go` — internal aliases remain aligned.
- Modify `internal/metadata/ai_conversations.go` — transactional snapshot write and active-row reads.
- Modify `internal/metadata/ai_conversations_test.go` — repository idempotency and conflict coverage.
- Modify `internal/http/handlers/ai_conversations.go` and `_test.go` — `Idempotency-Key`, version conflict, response contract.
- Modify `frontend/src/types/ai.ts` — `remote_id`, `ordinal`, `snapshot_version`.
- Modify `frontend/src/hooks/useConversation.ts` and `.test.ts` — stable IDs and versioned writes.
- Create `internal/metadata/conversation_repair.go` and `_test.go` — ordered-prefix detector and transactional repair.
- Create `cmd/conversation-repair/main.go` — detect/report/archive/apply/restore/purge CLI.

### Agent runtime

- Create `migrations/065a_agentic_query_runner.up.sql` and `.down.sql` — run modes/statuses, idempotent step/event fields, shadow comparison storage.
- Create `internal/agent/contracts.go` and `_test.go` — versioned job/step/result/error envelopes.
- Create `internal/agent/policy.go` and `_test.go` — deterministic tool proposal gate.
- Create `internal/agent/tools.go`, `catalog_tool.go`, `semantic_tool.go`, `query_tools.go`, `memory_tool.go` and focused tests.
- Create `internal/agent/planner.go` and `_test.go` — typed planner decisions.
- Create `internal/agent/runtime.go` and `_test.go` — bounded resumable state machine.
- Create `internal/agent/shadow.go` and `_test.go` — legacy/Agent comparison outside the tool registry.
- Create `internal/agent/service.go` and `_test.go` — queue handler, persistence, events.
- Modify `internal/metadata/agent_runs.go` and tests — append/idempotent steps and resumable runtime state.
- Modify `internal/config/config.go` and config tests — Agent environment defaults.
- Modify `internal/http/handlers/ai_admin_config.go`, `runtime_overrides.go`, and tests — dynamic Agent config domain.
- Modify `internal/queue/queue.go`, `nats.go`, and tests — subject-aware publisher/consumer.
- Modify `internal/http/handlers/ai_job_service.go` and tests — feature routing and normalized job payload.
- Create `cmd/agent/main.go` and `main_test.go` — internal service lifecycle.

### Production integration

- Create `.github/workflows/build-agent.yml`.
- Create `deploy/helm/biqly/charts/agent/**`.
- Modify umbrella Helm chart/values/lock and shared policies.
- Create `deploy/helm/biqly/templates/cnp-agent.yaml`.
- Modify `cnp-ai-external.yaml`, `cnp-shared-postgresql.yaml`, `cnp-dns.yaml`, OTEL and peer ingress policies as required.
- Modify `scripts/helm-bump-tags.sh`.
- Modify `Makefile`, `.env.dev.example`, Compose configuration, `docs/agents/local-dev.md`, `docs/configuration.md`, and `CONTEXT.md`.
- Create `scripts/assert-agent-helm.sh`.
- Extend eval regression and observability metrics/alerts.

---

### Task 1: Add conversation identity and write-ledger schema

**Files:**
- Create: `migrations/063a_conversation_idempotency.up.sql`
- Create: `migrations/063a_conversation_idempotency.down.sql`
- Modify: `cmd/migrate/ab_experiments_migration_test.go` only if its migration inventory assertion is generic and needs the new pair

- [ ] **Step 1: Write the migration contract test**

Add a migration test that applies `063a` to a clean metadata schema and asserts:

```sql
SELECT snapshot_version FROM ai_conversations LIMIT 0;
SELECT remote_id, ordinal, updated_at, deleted_at, deleted_by_repair_run_id
FROM ai_conversation_messages LIMIT 0;
SELECT idempotency_key, payload_hash, response_status
FROM conversation_write_requests LIMIT 0;
```

Assert duplicate `(conversation_id, remote_id)` insertion fails.

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run ConversationIdempotency -count=1
```

Expected: FAIL because migration `063a` and its columns do not exist.

- [ ] **Step 3: Implement the migration**

`063a_conversation_idempotency.up.sql` must contain:

```sql
ALTER TABLE ai_conversations
    ADD COLUMN snapshot_version BIGINT NOT NULL DEFAULT 0;

ALTER TABLE ai_conversation_messages
    ADD COLUMN remote_id TEXT,
    ADD COLUMN ordinal INTEGER,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN deleted_by_repair_run_id UUID;

CREATE UNIQUE INDEX ux_ai_conversation_messages_remote
    ON ai_conversation_messages(conversation_id, remote_id)
    WHERE remote_id IS NOT NULL;

CREATE INDEX idx_ai_conversation_messages_active_order
    ON ai_conversation_messages(conversation_id, ordinal, created_at)
    WHERE deleted_at IS NULL;

CREATE TABLE conversation_write_requests (
    idempotency_key TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    payload_hash TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')),
    response_status INTEGER,
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);
```

The down migration drops the ledger and indexes, then removes the added columns in reverse dependency order.

- [ ] **Step 4: Verify migration round-trip**

Run:

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./cmd/migrate -run ConversationIdempotency -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add migrations/063a_conversation_idempotency.* cmd/migrate
git commit -m "feat(conversations): add idempotent snapshot schema"
```

---

### Task 2: Implement transactional conversation snapshot persistence

**Files:**
- Modify: `pkg/metadata/types.go`
- Modify: `internal/metadata/ai_conversations.go`
- Modify: `internal/metadata/ai_conversations_test.go`
- Modify: `internal/http/handlers/ai_conversations.go`
- Modify: `internal/http/handlers/ai_conversations_test.go`

- [ ] **Step 1: Run gograph pre-edit plans**

Run `gograph_plan` for:

```text
(*AIHandler).CreateConversation
(*Repository).CreateAIConversation
(*Repository).CreateAIConversationMessage
(*Repository).ListAIConversations
```

Record any callers or tests not named above before editing.

- [ ] **Step 2: Write repository RED tests**

Cover:

```go
func TestSaveAIConversationSnapshotIsIdempotent(t *testing.T)
func TestSaveAIConversationSnapshotRejectsStaleVersion(t *testing.T)
func TestSaveAIConversationSnapshotRejectsImmutableMessageRewrite(t *testing.T)
func TestSaveAIConversationSnapshotRollsBackWholeSnapshot(t *testing.T)
func TestSaveAIConversationSnapshotReplaysStoredIdempotentResponse(t *testing.T)
func TestListAIConversationsExcludesSoftDeletedMessages(t *testing.T)
```

Use a message wire shape with:

```go
type AIConversationMessage struct {
    ID             string          `json:"id,omitempty"`
    RemoteID       string          `json:"remote_id"`
    ConversationID string          `json:"conversation_id,omitempty"`
    Ordinal        int             `json:"ordinal"`
    Role           string          `json:"role"`
    Content        string          `json:"content"`
    AIResponse     json.RawMessage `json:"ai_response,omitempty"`
    ResultSummary  *string         `json:"result_summary,omitempty"`
    CreatedAt      time.Time       `json:"created_at"`
    UpdatedAt      time.Time       `json:"updated_at"`
}
```

- [ ] **Step 3: Run focused tests and verify RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/metadata ./internal/http/handlers \
  -run 'ConversationSnapshot|AIConversations' -count=1
```

Expected: FAIL because `SaveAIConversationSnapshot` and conflict errors do not exist.

- [ ] **Step 4: Implement repository transaction**

Add:

```go
var (
    ErrConversationVersionConflict = errors.New("conversation version conflict")
    ErrConversationMessageConflict = errors.New("conversation message conflict")
    ErrIdempotencyKeyConflict       = errors.New("idempotency key conflict")
)

type ConversationSnapshotWrite struct {
    Conversation   AIConversation
    ExpectedVersion int64
    IdempotencyKey string
    PayloadHash    string
}

type ConversationSnapshotResult struct {
    Conversation AIConversation
    StatusCode   int
}

func (r *Repository) SaveAIConversationSnapshot(
    ctx context.Context,
    userID string,
    in ConversationSnapshotWrite,
) (ConversationSnapshotResult, error)
```

Inside one `sql.Tx`:

1. reserve/read `conversation_write_requests`;
2. upsert and `SELECT ... FOR UPDATE` the owned conversation;
3. compare `snapshot_version`;
4. upsert each message on `(conversation_id, remote_id)`;
5. reject a finalized message whose canonical payload hash differs;
6. increment version;
7. persist response and commit.

Do not call the old per-message insert loop from the HTTP handler.

- [ ] **Step 5: Implement HTTP contract**

`aiConversationRequest` gains:

```go
SnapshotVersion int64 `json:"snapshot_version"`
```

Require:

```text
Idempotency-Key: <non-empty key>
```

Map conflicts:

```text
version conflict       → 409
remote_id content clash → 409
idempotency mismatch   → 409
missing remote_id      → 400
missing key            → 400
```

Return the updated `snapshot_version` and server message IDs.

- [ ] **Step 6: Verify focused tests**

```bash
gofmt -w pkg/metadata/types.go internal/metadata/ai_conversations.go \
  internal/metadata/ai_conversations_test.go \
  internal/http/handlers/ai_conversations.go \
  internal/http/handlers/ai_conversations_test.go
GOCACHE=/private/tmp/biqly-gocache go test ./internal/metadata ./internal/http/handlers \
  -run 'ConversationSnapshot|AIConversations' -count=1
```

Expected: PASS.

- [ ] **Step 7: Run gograph review and commit**

Run `gograph_review --uncommitted`, then:

```bash
git add pkg/metadata/types.go internal/metadata/ai_conversations.go \
  internal/metadata/ai_conversations_test.go \
  internal/http/handlers/ai_conversations.go \
  internal/http/handlers/ai_conversations_test.go
git commit -m "fix(conversations): make snapshot writes idempotent"
```

---

### Task 3: Add stable frontend message identity and versioned writes

**Files:**
- Modify: `frontend/src/types/ai.ts`
- Modify: `frontend/src/hooks/useConversation.ts`
- Modify: `frontend/src/hooks/useConversation.test.ts`

- [ ] **Step 1: Write frontend RED tests**

Add tests proving:

- a new user or assistant message receives one stable `remote_id`;
- repeated saves retain the same IDs;
- remote IDs survive reload/normalization;
- `ordinal` is stable;
- `Idempotency-Key` changes per logical snapshot but remains stable across network retries;
- a `409` reloads the latest server snapshot instead of blindly retrying stale state.

- [ ] **Step 2: Run RED**

```bash
npm --prefix frontend run test -- useConversation
```

Expected: FAIL on missing identity/version fields.

- [ ] **Step 3: Implement minimal identity helpers**

Add:

```ts
export interface ConversationMessage {
  id?: string
  remote_id: string
  ordinal: number
  role: 'user' | 'assistant'
  content: string
  timestamp: string
  ai_response?: AIQueryResponse
  job_id?: string
  result_summary?: string
}

export interface Conversation {
  id: string
  snapshot_version: number
  messages: ConversationMessage[]
  // existing fields remain
}
```

Use `crypto.randomUUID()` at message creation. Normalize legacy local messages once by assigning
and persisting a remote ID before the first write.

Generate a snapshot request ID before the API call and reuse it only for retries of the exact
same serialized payload.

- [ ] **Step 4: Verify frontend**

```bash
npm --prefix frontend run test -- useConversation
make typecheck-frontend
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/ai.ts frontend/src/hooks/useConversation.ts \
  frontend/src/hooks/useConversation.test.ts
git commit -m "fix(frontend): persist stable conversation message ids"
```

---

### Task 4: Add repair schema and evidence detector

**Files:**
- Create: `migrations/064a_conversation_repair.up.sql`
- Create: `migrations/064a_conversation_repair.down.sql`
- Create: `internal/metadata/conversation_repair.go`
- Create: `internal/metadata/conversation_repair_test.go`

- [ ] **Step 1: Write RED property/table tests**

Test:

- `[U1]`, `[U1,A1]`, `[U1,A1,U2]` is a valid replay chain;
- different role/content/provenance at the same ordinal is ambiguous;
- a partial final batch is rejected;
- feedback/pin/export references force report-only;
- canonical hash changes abort apply;
- legitimate repeated identical text at different ordinals in the final batch survives.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/metadata \
  -run ConversationRepair -count=1
```

- [ ] **Step 3: Add repair tables**

Create `conversation_repair_runs` and `conversation_message_repair_archive` exactly as specified
in the design. Add the deferred FK from
`ai_conversation_messages.deleted_by_repair_run_id` to `conversation_repair_runs(id)`.

- [ ] **Step 4: Implement pure detector**

Expose:

```go
type RepairMessage struct {
    ID         string
    Role       string
    Content    string
    Response   json.RawMessage
    Summary    string
    CreatedAt  time.Time
    Provenance string
}

type RepairCandidate struct {
    ConversationID string
    CanonicalHash  string
    KeepIDs        []string
    ReplayIDs      []string
    Reason         string
}

func DetectReplayChain(messages []RepairMessage, batchGap time.Duration) (RepairCandidate, bool)
```

Use a fixed production batch gap of `250*time.Millisecond`. Canonicalize JSON before hashing.
Require an exact ordered-prefix chain and a unique longest final batch.

- [ ] **Step 5: Verify**

```bash
gofmt -w internal/metadata/conversation_repair.go internal/metadata/conversation_repair_test.go
GOCACHE=/private/tmp/biqly-gocache go test ./internal/metadata \
  -run ConversationRepair -count=1
```

- [ ] **Step 6: Commit**

```bash
git add migrations/064a_conversation_repair.* internal/metadata/conversation_repair*
git commit -m "feat(conversations): detect replay chains safely"
```

---

### Task 5: Implement repair CLI, archive, soft-delete, restore, and purge

**Files:**
- Modify: `internal/metadata/conversation_repair.go`
- Modify: `internal/metadata/conversation_repair_test.go`
- Create: `cmd/conversation-repair/main.go`
- Create: `cmd/conversation-repair/main_test.go`

- [ ] **Step 1: Write command RED tests**

Cover commands:

```text
detect --dry-run
report --conversation-id <id>
archive --run-id <id>
apply --run-id <id>
restore --run-id <id>
purge --run-id <id>
```

Prove `apply`:

- requires an existing dry-run;
- locks the conversation;
- recomputes the canonical hash;
- archives full rows;
- sets `deleted_at` and `deleted_by_repair_run_id`;
- never hard-deletes.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./cmd/conversation-repair ./internal/metadata \
  -run ConversationRepair -count=1
```

- [ ] **Step 3: Implement CLI**

Default behavior must be read-only:

```text
conversation-repair detect --dry-run
```

Require `--run-id` for mutations. Require `--confirm-purge <run-id>` for physical purge.
Emit JSON reports containing candidate counts, skipped reasons, canonical hashes, and row IDs.

- [ ] **Step 4: Verify and review**

```bash
gofmt -w cmd/conversation-repair internal/metadata/conversation_repair*
GOCACHE=/private/tmp/biqly-gocache go test ./cmd/conversation-repair ./internal/metadata \
  -run ConversationRepair -count=1
gograph_review --uncommitted
```

- [ ] **Step 5: Commit**

```bash
git add cmd/conversation-repair internal/metadata/conversation_repair*
git commit -m "feat(conversations): add reversible replay repair"
```

---

### Task 6: Define Agent contracts and configuration

**Files:**
- Create: `internal/agent/contracts.go`
- Create: `internal/agent/contracts_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/http/handlers/ai_admin_config.go`
- Modify: `internal/http/handlers/ai_admin_config_test.go`
- Modify: `internal/http/handlers/runtime_overrides.go`
- Modify: `docs/configuration.md`

- [ ] **Step 1: Write RED contract/config tests**

Require exact schema versions:

```go
const (
    JobSchemaV1    = "agent-job.v1"
    StepSchemaV1   = "agent-step.v1"
    ResultSchemaV1 = "agent-result.v1"
    ErrorSchemaV1  = "agent-error.v1"
)
```

Validate mode, IDs, `max_steps` 1–6, clarification rounds 0–2, timeout 1–45 seconds, max rows
1–1000, and unsupported major versions.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/config \
  ./internal/http/handlers -run 'AgentConfig|AgentContract' -count=1
```

- [ ] **Step 3: Implement config**

Add:

```go
type AgentConfig struct {
    Enabled                bool
    Mode                   string
    MaxSteps               int
    MaxClarificationRounds int
    Timeout                time.Duration
    MaxRows                int
    JobSubject             string
    StepSubject            string
    ResultSubject          string
    ErrorSubject           string
    WorkspaceAllowlist     []string
    LegacyFallbackEnabled  bool
}
```

Add `Agent AgentConfig` to `config.Config`. Add an `agent` dynamic runtime domain with pointer
fields and strict validation. Environment defaults: disabled, shadow, 6 steps, 2 clarification
rounds, 45 seconds, 1000 rows.

- [ ] **Step 4: Verify**

```bash
gofmt -w internal/agent/contracts* internal/config internal/http/handlers/ai_admin_config*
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/config \
  ./internal/http/handlers -run 'AgentConfig|AgentContract' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add internal/agent/contracts* internal/config internal/http/handlers/ai_admin_config* \
  internal/http/handlers/runtime_overrides.go docs/configuration.md
git commit -m "feat(agent): define runtime contracts and config"
```

---

### Task 7: Implement deterministic Policy Engine

**Files:**
- Create: `internal/agent/policy.go`
- Create: `internal/agent/policy_test.go`

- [ ] **Step 1: Write RED policy table tests**

Cover tenant/user/datasource mismatch, hidden columns, PII masking, row filters, invalid joins,
multi-statement SQL, writes/DDL, row limit, timeout, prompt injection, tool allowlist, retry
budget, and airgapped egress.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent -run Policy -count=1
```

- [ ] **Step 3: Implement**

```go
type ToolName string

const (
    ToolCatalog      ToolName = "catalog.resolve"
    ToolSemantic     ToolName = "semantic.resolve"
    ToolQueryCompile ToolName = "query.compile"
    ToolQueryExecute ToolName = "query.execute"
    ToolMemoryRecall ToolName = "memory.recall"
)

type Proposal struct {
    Tool      ToolName
    Arguments json.RawMessage
}

type Decision struct {
    Allowed    bool
    ReasonCode string
    Arguments  json.RawMessage
}

type PolicyEngine struct { /* deterministic dependencies only */ }

func (p *PolicyEngine) Evaluate(ctx context.Context, run RunContext, proposal Proposal) Decision
```

Policy may narrow arguments but never expand access.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w internal/agent/policy*
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent -run Policy -count=1
git add internal/agent/policy*
git commit -m "feat(agent): add deterministic tool policy engine"
```

---

### Task 8: Implement typed tool adapters

**Files:**
- Create: `internal/agent/tools.go`
- Create: `internal/agent/catalog_tool.go`
- Create: `internal/agent/catalog_tool_test.go`
- Create: `internal/agent/semantic_tool.go`
- Create: `internal/agent/semantic_tool_test.go`
- Create: `internal/agent/query_tools.go`
- Create: `internal/agent/query_tools_test.go`
- Create: `internal/agent/memory_tool.go`
- Create: `internal/agent/memory_tool_test.go`
- Modify: `pkg/catalogclient/*` only for missing internal endpoints
- Modify: `pkg/aiclient/*` only for missing semantic generation endpoints
- Modify: `pkg/queryclient/*` only for missing compile/execute endpoints

- [ ] **Step 1: Write fake-client RED tests**

Each tool must prove:

- typed decode rejects unknown fields;
- context deadline is propagated;
- internal caller is `agent`;
- request/trace IDs propagate;
- transient retry occurs at most once;
- policy-approved limits reach upstream;
- raw credentials and unrestricted destinations cannot be supplied.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./pkg/catalogclient \
  ./pkg/aiclient ./pkg/queryclient -run 'Tool|AgentCaller' -count=1
```

- [ ] **Step 3: Implement registry**

```go
type Tool interface {
    Name() ToolName
    Execute(context.Context, RunContext, json.RawMessage) (Observation, error)
}

type Registry struct {
    tools map[ToolName]Tool
}

func (r *Registry) Execute(
    ctx context.Context,
    run RunContext,
    proposal Proposal,
) (Observation, error)
```

Keep Query Compile and Query Execute separate. Query Execute accepts only the signed/opaque
compile result returned by Query service, not planner-authored SQL.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w internal/agent pkg/catalogclient pkg/aiclient pkg/queryclient
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./pkg/catalogclient \
  ./pkg/aiclient ./pkg/queryclient -run 'Tool|AgentCaller' -count=1
git add internal/agent pkg/catalogclient pkg/aiclient pkg/queryclient
git commit -m "feat(agent): add governed BI tool adapters"
```

---

### Task 9: Implement planner decisions and bounded runtime

**Files:**
- Create: `internal/agent/planner.go`
- Create: `internal/agent/planner_test.go`
- Create: `internal/agent/runtime.go`
- Create: `internal/agent/runtime_test.go`
- Modify: `internal/metadata/agent_runs.go`
- Modify: `internal/metadata/agent_runs_test.go`
- Create: `migrations/065a_agentic_query_runner.up.sql`
- Create: `migrations/065a_agentic_query_runner.down.sql`

- [ ] **Step 1: Write runtime RED tests**

Cover successful plan, policy denial/correction, clarification, max two clarification rounds,
max six steps, timeout, cancellation, retry, NATS redelivery resume, immutable terminal result,
and no fallback after Query Execute begins.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/metadata \
  -run 'Planner|Runtime|Resume' -count=1
```

- [ ] **Step 3: Extend persistence**

`065a` adds:

```sql
ALTER TABLE agent_runs
    ADD COLUMN job_id UUID,
    ADD COLUMN runtime_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN terminal_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN query_execute_started BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX ux_agent_runs_job ON agent_runs(job_id) WHERE job_id IS NOT NULL;

ALTER TABLE agent_steps
    ADD CONSTRAINT ux_agent_steps_run_seq UNIQUE(run_id, seq);

CREATE TABLE agent_shadow_comparisons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,
    legacy_run_id UUID,
    agent_run_id UUID,
    category TEXT NOT NULL,
    detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 4: Implement planner decision union**

```go
type DecisionKind string

const (
    DecisionTool          DecisionKind = "tool"
    DecisionClarification DecisionKind = "clarification"
    DecisionFinal         DecisionKind = "final"
    DecisionFail          DecisionKind = "fail"
)

type PlannerDecision struct {
    Kind          DecisionKind
    Proposal      *Proposal
    Clarification *Clarification
    Final         *FinalResponse
    Failure       *Failure
}
```

Strictly decode provider JSON; reject mixed/unknown variants.

- [ ] **Step 5: Implement resumable loop**

Persist before and after every external call. Allocate step sequence transactionally. Check
cancellation and deadlines between steps. Mark `query_execute_started` before calling Query
Execute.

- [ ] **Step 6: Verify and commit**

```bash
gofmt -w internal/agent internal/metadata/agent_runs*
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./internal/metadata \
  -run 'Planner|Runtime|Resume|AgentRun' -count=1
gograph_review --uncommitted
git add migrations/065a_agentic_query_runner.* internal/agent internal/metadata/agent_runs*
git commit -m "feat(agent): add bounded resumable query runtime"
```

---

### Task 10: Add subject-aware queue routing and shadow evaluator

**Files:**
- Modify: `internal/queue/queue.go`
- Modify: `internal/queue/nats.go`
- Modify: `internal/queue/nats_test.go`
- Modify: `internal/http/handlers/ai_job_service.go`
- Modify: `internal/http/handlers/ai_job_service_test.go`
- Create: `internal/agent/shadow.go`
- Create: `internal/agent/shadow_test.go`

- [ ] **Step 1: Write RED routing tests**

Prove:

```text
disabled → legacy subject
shadow → legacy authoritative + agent shadow subject
beta allowlisted → agent authoritative
beta non-allowlisted → legacy
default → agent
pre-execute Agent failure → legacy only when enabled
post-execute Agent failure → no fallback
```

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/queue ./internal/http/handlers \
  ./internal/agent -run 'Subject|FeatureRoute|Shadow' -count=1
```

- [ ] **Step 3: Make queue subject-aware**

Replace fixed-subject-only use with:

```go
type Publisher interface {
    Publish(ctx context.Context, subject, key string, payload []byte) error
}

type Consumer interface {
    Subscribe(
        ctx context.Context,
        subject string,
        group string,
        handler func(context.Context, []byte) error,
    ) error
}
```

Keep legacy adapter methods temporarily if existing callers need a compatibility seam.

- [ ] **Step 4: Implement external shadow comparison**

Compare normalized LogicalQuery/SQL/result/latency/clarification/policy outcomes. Persist
bounded categories; never expose SQL or question as metric labels.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w internal/queue internal/http/handlers/ai_job_service* internal/agent/shadow*
GOCACHE=/private/tmp/biqly-gocache go test ./internal/queue ./internal/http/handlers \
  ./internal/agent -run 'Subject|FeatureRoute|Shadow' -count=1
git add internal/queue internal/http/handlers/ai_job_service* internal/agent/shadow*
git commit -m "feat(agent): route and evaluate shadow jobs"
```

---

### Task 11: Build `cmd/agent` service

**Files:**
- Create: `internal/agent/service.go`
- Create: `internal/agent/service_test.go`
- Create: `cmd/agent/main.go`
- Create: `cmd/agent/main_test.go`
- Modify: `internal/app/dependencies.go`

- [ ] **Step 1: Run gograph plans**

Plan `app.NewAIDependencies`, `(*AIJobService).StartConsumer`, and `cmd/worker.main` before
extracting reusable lifecycle patterns.

- [ ] **Step 2: Write RED service tests**

Test dependency failure, unsupported queue backend, readiness before/after subscription,
graceful shutdown, cancellation, redelivery, internal token on diagnostics/cancel, and metrics
endpoint behavior.

- [ ] **Step 3: Implement `AgentDependencies`**

Expose only required dependencies:

```go
type AgentDependencies struct {
    Config        *config.Config
    MetaRepo      *metadata.Repository
    Queue         agent.Queue
    Planner       agent.Planner
    Policy        *agent.PolicyEngine
    Tools         *agent.Registry
    Runtime       *agent.Runtime
    Shadow        *agent.ShadowEvaluator
    Metrics       *observability.Metrics
    Close         func() error
}
```

- [ ] **Step 4: Implement internal server**

Serve:

```text
GET  /healthz
GET  /readyz
GET  /metrics
GET  /internal/agent/runs/{id}
POST /internal/agent/runs/{id}/cancel
```

Use `BI_HTTP_PORT=8084`, internal-token middleware, bounded HTTP timeouts, signal handling, and
10-second graceful shutdown.

- [ ] **Step 5: Verify and commit**

```bash
gofmt -w internal/agent/service* cmd/agent internal/app/dependencies.go
GOCACHE=/private/tmp/biqly-gocache go test ./internal/agent ./cmd/agent ./internal/app -count=1
go build ./cmd/agent
gograph_review --uncommitted
git add internal/agent/service* cmd/agent internal/app/dependencies.go
git commit -m "feat(agent): add internal query runner service"
```

---

### Task 12: Wire persisted trace and live Agent steps

**Files:**
- Modify: `frontend/src/api/agentRuns.ts`
- Modify: `frontend/src/components/aiQuery/RunTrace.tsx`
- Modify: `frontend/src/components/aiQuery/RunTrace.test.tsx` or create it
- Modify: `frontend/src/types/ai.ts`
- Modify: `frontend/src/i18n/locales/en/core.ts`
- Modify: `frontend/src/i18n/locales/tr/core.ts`

- [ ] **Step 1: Write RED UI tests**

Cover new Agent step kinds, persisted reload, policy-denied display, clarification step,
cancelled/failed terminal states, and no raw sensitive detail rendering.

- [ ] **Step 2: Run RED**

```bash
npm --prefix frontend run test -- RunTrace
```

- [ ] **Step 3: Implement minimal UI changes**

Reuse `RunTracePanel`; add labels for planner, policy, tool, observation, clarification,
final-response and shadow-comparison steps. Keep details collapsed and sanitized.

- [ ] **Step 4: Verify and commit**

```bash
npm --prefix frontend run test -- RunTrace
make check-frontend
git add frontend/src/api/agentRuns.ts frontend/src/components/aiQuery/RunTrace* \
  frontend/src/types/ai.ts frontend/src/i18n/locales/en/core.ts \
  frontend/src/i18n/locales/tr/core.ts
git commit -m "feat(frontend): display persisted agent query trace"
```

---

### Task 13: Add Agent image workflow and local developer loop

**Files:**
- Create: `.github/workflows/build-agent.yml`
- Modify: `scripts/helm-bump-tags.sh`
- Modify: `Makefile`
- Modify: `.env.dev.example`
- Modify: `docker-compose.yml` or the repository's active Compose file
- Modify: `docs/agents/local-dev.md`

- [ ] **Step 1: Write/check static expectations**

Assert:

- workflow builds `SERVICE=agent`;
- image name is `ghcr.io/biqly/agent`;
- path filters include `cmd/agent/**`, `internal/agent/**`, shared packages, module files,
  Dockerfile, and workflow;
- `helm-bump-tags.sh` maps `agent.image.tag`;
- `WATCH_SVCS` includes `agent`.

- [ ] **Step 2: Implement workflow**

Copy action versions exactly from `build-worker.yml`; change only names, paths, image, and
`SERVICE=agent`.

- [ ] **Step 3: Implement local loop**

Set:

```make
WATCH_SVCS ?= api auth mail agent
```

Document:

```bash
make watch SVC="api agent auth"
make debug-watch SVC=agent
```

Agent local service URLs point to embedded API endpoints on `localhost:8888`; NATS/Redis/DB
point to `localhost`.

- [ ] **Step 4: Verify and commit**

```bash
go build ./cmd/agent
docker build --build-arg SERVICE=agent -t biqly-agent:test .
git diff --check
git add .github/workflows/build-agent.yml scripts/helm-bump-tags.sh Makefile \
  .env.dev.example docker-compose.yml docs/agents/local-dev.md
git commit -m "build(agent): add image and local dev workflow"
```

---

### Task 14: Scaffold the Agent Helm subchart

**Files:**
- Create: `deploy/helm/biqly/charts/agent/Chart.yaml`
- Create: `deploy/helm/biqly/charts/agent/values.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/_helpers.tpl`
- Create: `deploy/helm/biqly/charts/agent/templates/configmap.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/deployment.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/service.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/hpa.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/pdb.yaml`
- Create: `deploy/helm/biqly/charts/agent/templates/tests/test-connection.yaml`
- Modify: `deploy/helm/biqly/Chart.yaml`
- Modify: `deploy/helm/biqly/Chart.lock`
- Modify: `deploy/helm/biqly/values.yaml`
- Modify: `deploy/helm/biqly/values-dev.yaml`
- Modify: `deploy/helm/biqly/values-prod.yaml`

- [ ] **Step 1: Add failing Helm render assertion**

Render with `agent.enabled=true` and assert Deployment, ClusterIP Service, HPA/PDB, probes,
security context, port 8084, resources, OTEL service name, and no HTTPRoute.

- [ ] **Step 2: Verify RED**

```bash
make helm-template
```

Expected: Agent resources absent.

- [ ] **Step 3: Implement subchart**

Use:

```yaml
enabled: false
replicaCount: 2
image:
  repository: agent
  tag: latest
service:
  port: 8084
config:
  BI_HTTP_PORT: "8084"
  BI_AGENT_ENABLED: "true"
  BI_AGENT_MODE: shadow
  BI_AGENT_MAX_STEPS: "6"
  BI_AGENT_MAX_CLARIFICATION_ROUNDS: "2"
  BI_AGENT_TIMEOUT_SECONDS: "45"
  BI_AGENT_MAX_ROWS: "1000"
  BI_AGENT_NATS_SUBJECT: biqly.ai.agent.jobs
resources:
  requests: {cpu: 250m, memory: 256Mi}
  limits: {cpu: "2", memory: 1Gi}
```

Use the existing shared service account with token automount disabled. Do not create an
HTTPRoute.

- [ ] **Step 4: Preserve user Query values changes**

Before editing umbrella values, verify the existing unstaged `/api/audit/query` path remains in
both `deploy/helm/biqly/charts/query/values.yaml` and `deploy/helm/biqly/values.yaml`. Do not
overwrite or stage it as part of Agent work unless the user explicitly includes it.

- [ ] **Step 5: Verify and commit**

```bash
helm dependency update deploy/helm/biqly
make helm-lint
make helm-template
git diff --check
git add deploy/helm/biqly/charts/agent deploy/helm/biqly/Chart.yaml \
  deploy/helm/biqly/Chart.lock deploy/helm/biqly/values-dev.yaml \
  deploy/helm/biqly/values-prod.yaml
git add -p deploy/helm/biqly/values.yaml
git commit -m "feat(deploy): add internal agent chart"
```

---

### Task 15: Add least-privilege Agent NetworkPolicies

**Files:**
- Create: `deploy/helm/biqly/templates/cnp-agent.yaml`
- Modify: `deploy/helm/biqly/templates/cnp-ai-external.yaml`
- Modify: `deploy/helm/biqly/templates/cnp-shared-postgresql.yaml`
- Modify: `deploy/helm/biqly/templates/cnp-dns.yaml`
- Modify peer ingress policy templates selected by Catalog, Query, AI, Auth, NATS, Dragonfly, OTEL
- Create: `scripts/assert-agent-helm.sh`
- Modify: `Makefile`

- [ ] **Step 1: Write failing rendered-policy assertions**

The script must fail unless:

- Agent ingress is only API/BFF and monitoring on 8084;
- Agent egress includes DNS, NATS 4222, Postgres 5432, Redis 6379, Catalog 8080, Query 8081,
  AI 8082, Auth 8889, OTEL 4317/4318;
- cloud/private permits policy-approved HTTPS 443;
- airgapped rendering omits world HTTPS;
- Agent has no query-user-database CIDR rule;
- no public route selects Agent.

- [ ] **Step 2: Run RED**

```bash
make helm-template
./scripts/assert-agent-helm.sh /tmp/biqly-helm-template.yaml
```

- [ ] **Step 3: Implement policies and peer symmetry**

Use selector `app.kubernetes.io/component: agent`. Add `agent` to shared PostgreSQL components
and only the peer ingress selectors required by the traffic matrix.

- [ ] **Step 4: Verify and commit**

```bash
make helm-lint
make helm-template
./scripts/assert-agent-helm.sh /tmp/biqly-helm-template.yaml
git add deploy/helm/biqly/templates scripts/assert-agent-helm.sh Makefile
git commit -m "feat(deploy): isolate agent network access"
```

---

### Task 16: Add Agent eval, metrics, tracing, and alerts

**Files:**
- Modify: `internal/platform/observability/tier2_metrics.go`
- Modify: `internal/platform/observability/tier2_metrics_test.go`
- Modify: `internal/agent/service.go`
- Modify: `internal/agent/shadow.go`
- Add focused Agent cases under the existing `internal/ai/eval/` fixtures/golden structure
- Modify: `deploy/helm/biqly/templates/alertmanager-config.yaml`

- [ ] **Step 1: Write RED metrics/eval tests**

Assert bounded labels only. Cover run states, step/tool latency, policy denial, clarification,
loop exhaustion, fallback, shadow divergence, queue redelivery, token/cost, timeout, and cancel.

- [ ] **Step 2: Run RED**

```bash
GOCACHE=/private/tmp/biqly-gocache go test ./internal/platform/observability ./internal/agent \
  -run 'AgentMetrics|Shadow' -count=1
make eval-regression
```

- [ ] **Step 3: Implement telemetry**

Trace:

```text
API → NATS publish → Agent run → policy → tool → Query execute → result
```

Never use question, SQL, credentials, or result rows as metric labels.

- [ ] **Step 4: Verify and commit**

```bash
gofmt -w internal/platform/observability internal/agent
GOCACHE=/private/tmp/biqly-gocache go test ./internal/platform/observability ./internal/agent
make eval-regression
make helm-template
git add internal/platform/observability internal/agent internal/ai/eval \
  deploy/helm/biqly/templates/alertmanager-config.yaml
git commit -m "feat(agent): add eval and observability gates"
```

---

### Task 17: Complete documentation and task tracker

**Files:**
- Modify: `CONTEXT.md`
- Modify: `docs/configuration.md`
- Modify: `docs/agents/local-dev.md`
- Modify: `tasks/todo.md`
- Modify: `tasks/lessons.md` only for implementation corrections actually encountered

- [ ] **Step 1: Document service ownership**

State that Agent owns bounded BI query orchestration; Query owns validation/execution; API owns
public auth/ACL; legacy worker remains fallback/report runner during rollout.

- [ ] **Step 2: Document operational controls**

Include mode/allowlist changes, fallback boundary, repair commands, restore procedure, metrics,
alerts, and airgapped behavior.

- [ ] **Step 3: Add tracker review section**

Record completed commits, focused verification, `make verify-main`, Helm assertions, repair
dry-run counts, live rollout stage, and remaining beta/default promotion decisions.

- [ ] **Step 4: Commit**

```bash
git add CONTEXT.md docs/configuration.md docs/agents/local-dev.md tasks/todo.md tasks/lessons.md
git commit -m "docs: document agentic query runner operations"
```

---

### Task 18: Full verification and production rollout

**Files:**
- No new implementation files; update `tasks/todo.md` review evidence

- [ ] **Step 1: Run all required local gates**

```bash
gofmt -w <all touched Go files>
make lint-go
make test-go
deadcode -test $(go list ./... | grep -v '/frontend')
make check-frontend
make eval-regression
make helm-lint
make helm-template
./scripts/assert-agent-helm.sh /tmp/biqly-helm-template.yaml
make verify-main
git diff --check
gograph_review --uncommitted
```

Expected: all green, zero lint issues, no unexplained dead code, no policy assertion failures.

- [ ] **Step 2: Build images**

```bash
docker build --build-arg SERVICE=agent -t biqly-agent:test .
GOCACHE=/private/tmp/biqly-gocache go build ./cmd/conversation-repair
```

The repair command is an operator CLI, not a continuously deployed service or published image.

- [ ] **Step 3: Run production repair dry-run**

Deploy schema and code with repair disabled, then run:

```text
conversation-repair detect --dry-run
conversation-repair report --conversation-id conv_1783340148968_jzzxkt
```

Verify the affected conversation resolves to the six-message final canonical batch and that all
ambiguous conversations are report-only.

- [ ] **Step 4: Apply reversible repair**

Create an archive run, apply soft-delete, verify API/UI no longer show duplicates, and retain the
archive. Do not purge.

- [ ] **Step 5: Dark-deploy Agent**

Deploy Agent with `BI_AGENT_ENABLED=false`. Verify:

```bash
kubectl -n biqly rollout status deployment/biqly-agent
kubectl -n biqly get pods -l app.kubernetes.io/component=agent
kubectl -n biqly logs deployment/biqly-agent --tail=100
```

Check health, readiness, NATS subscription readiness, DB connectivity, and NetworkPolicy flows.

- [ ] **Step 6: Enable shadow**

Enable shadow mode. Confirm:

- users still receive legacy answers;
- Agent runs/steps persist;
- no second customer query execution occurs;
- shadow comparisons populate;
- latency/token/cost and policy alerts remain healthy.

- [ ] **Step 7: Produce promotion report**

Update `tasks/todo.md` with comparison counts, divergence categories, policy denials, Agent
failures, and recommendation. Do not enable beta/default automatically.

- [ ] **Step 8: Commit final evidence**

```bash
git add tasks/todo.md
git commit -m "docs: record agent shadow rollout evidence"
```
