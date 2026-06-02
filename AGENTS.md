# Biqly — Agent Guide (Text-to-SQL / NL-to-LogicalQuery)

This file is a **working guide for coding agents** operating on Biqly. Keep it aligned with `README.md`.

## Non-negotiables

- **AI generates `LogicalQuery` JSON — never raw SQL.**
- **All SQL is produced by the backend compiler** and must be **parameterized**.
- **Never weaken security**: read-only checker, permissions, row-level filters, timeouts, and row limits must remain enforced.
- **Treat permissions as fail-closed**: if a policy is missing/unknown, deny.
- **Do not log secrets** (DSNs, API keys, tokens).

## Current architecture (high level)

Pipeline (monolith or microservices):

```
User Question (NL)
  → Table Router (keyword + synonym + embedding + FK graph)
  → Prompt Builder (semantic context + few-shot + glossary + sample data)
  → LLM Provider (OpenAI-compatible / Anthropic)
  → LogicalQuery JSON
  → Validator (semantic + permissions)
  → Compiler (LogicalQuery → dialect SQL + args)
  → Read-only Checker
  → Executor (timeout + row limit)
  → Enrichment + Audit/History
```

Biqly can run:
- **Monolith**: `cmd/api` handles everything.
- **Microservices behind BFF**: Catalog `:8880`, Query `:8881`, AI `:8882`, Auth `:8889`, Mail `:8890`, Worker consumes async jobs via **NATS JetStream**. BFF stays on `:8888`.

## Where to make changes (by intent)

- **NL → LogicalQuery behavior**
  - Table routing: `internal/ai/routing/`
  - Prompt building/templates/budget/glossary: `internal/ai/prompt/`
  - Provider integrations: `internal/ai/provider/`
  - JSON extraction/validation: `internal/ai/jsonextract/`, `internal/ai/*validator*`
  - Evaluation: `internal/ai/eval/`

- **LogicalQuery → SQL correctness**
  - Core compile: `internal/query/compiler*.go`
  - Validation rules: `internal/query/validator.go`
  - Joins/fanout: `internal/query/planner.go`
  - Execution safety: `internal/query/executor.go`
  - Fingerprinting/cache keys: `internal/query/fingerprint.go`

- **Security & permissions**
  - Read-only SQL checker: `internal/security/readonly.go`
  - DSN encryption: `internal/security/encryption.go`
  - RLS injection & policy enforcement: `internal/security/*`

- **Semantic layer**
  - Types + workflow: `internal/semantic/` (draft/publish/rollback, versioning)
  - Auto-generation from metadata: `internal/semanticgen/`

- **Runtime AI settings**
  - Providers/models are **DB-backed and configurable at runtime** via admin APIs/UI (not env vars).
  - Glossary and prompt templates are first-class and versioned.

## Repo layout (agent-relevant)

- `cmd/` — entrypoints (`api`, `catalog`, `query`, `ai`, `auth`, `mail`, `worker`, `migrate*`, `export-sft`)
- `services/` — service-specific wiring for standalone modes
- `internal/` — core domain implementation (AI, query engine, semantic layer, metadata, security, auth, platform)
- `pkg/` — shared clients and canonical types for inter-service communication
- `frontend/` — React 19 + Vite 6 UI (admin, AI query, modeling canvas)
- `deploy/` — Helm + Argo CD configs
- `migrations/` — metadata/auth/mail DB migrations (see `README.md` for current counts)

## Development commands

See `README.md` for the full list. Common:

- `make docker-up`
- `make dev` (monolith API)
- `make run-catalog`, `make run-query`, `make run-ai`, `make run` (BFF)
- `make test`, `make lint`

## Common failure modes to guard against

- Invented fields not present in semantic model
- Wrong join path / fanout (one-to-many blowups)
- Missing time constraints leading to huge scans
- Locale/timezone mistakes in date filters
- Leaking denied fields into prompts or results
- Generating non-read-only SQL (must be blocked)
