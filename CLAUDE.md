# Biqly — Agent Memory (Text-to-SQL / NL-to-LogicalQuery)

Source of truth for product shape is `README.md`. This file exists to keep **agent-facing constraints** and **navigation hints** aligned with current code.

## Hard constraints (do not violate)

- **LogicalQuery-first**: AI outputs `LogicalQuery` JSON, never SQL.
- **Compiler owns SQL**: always parameterized, dialect-quoted identifiers; never concatenate values.
- **Security gates stay strict**: read-only checker, permissions, row-level filter injection, timeouts, row limits.
- **Fail closed**: missing/unknown permission policy must deny.
- **No secrets in logs**: DSNs, API keys, tokens, session IDs.

## Runtime modes

- **Monolith**: `cmd/api` serves everything on `:8888`.
- **Microservices + BFF** (BFF remains `:8888` and proxies when service URLs are set):
  - Catalog `:8880`
  - Query `:8881`
  - AI `:8882`
  - Auth `:8889`
  - Mail `:8890`
  - Worker consumes async AI jobs via **NATS JetStream**

## Where to change things (quick map)

- **NL → LogicalQuery**
  - Routing: `internal/ai/routing/`
  - Prompt/templates/budget/glossary: `internal/ai/prompt/`
  - Providers: `internal/ai/provider/`
  - JSON extraction/validation: `internal/ai/jsonextract/`
  - Eval: `internal/ai/eval/`

- **LogicalQuery → SQL**
  - Compiler: `internal/query/compiler*.go`
  - Validator: `internal/query/validator.go`
  - Joins/fanout: `internal/query/planner.go`
  - Execution: `internal/query/executor.go`

- **Security & permissions**
  - Read-only checker: `internal/security/readonly.go`
  - Encryption + DSN handling: `internal/security/encryption.go`
  - RLS/policies: `internal/security/`

- **Semantic layer**
  - Draft/publish/rollback: `internal/semantic/`
  - Auto-generation from metadata: `internal/semanticgen/`

- **Inter-service / shared types**
  - Clients + canonical types: `pkg/`

- **Frontend**
  - UI: `frontend/` (admin, modeling canvas, AI query panel)

## AI provider/model configuration

- Providers/models/templates/glossary are **DB-backed and configurable at runtime** via admin APIs/UI.
- Do **not** assume env vars select provider/model; treat env as infrastructure wiring only (ports, DSNs, redis/nats URLs, budgets).

## When adding/changing query semantics

If you add/modify any of these:

- filter operators
- select item types (e.g. window/case)
- time grains
- compiler behaviors

Then ensure you update (as applicable):

- request/response schema + prompt rendering
- validator + compiler
- golden tests / eval cases
- read-only checker compatibility (if SQL shape changes)
