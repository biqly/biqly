# MCP Client-Generated LogicalQuery Mode — Design

**Date:** 2026-07-08
**Status:** Approved
**Owner:** baris.dogu

## Problem

Biqly's MCP gateway already exposes two execution paths: `run_question` (Biqly's
AI pipeline generates the LogicalQuery) and `run_logical_query` (the caller
supplies a pre-built LogicalQuery that runs through the full validate → RLS/PII
→ compile → read-only execute → audit chain, with no LLM step). The skeleton of
"two modes" therefore exists, but an MCP client model authoring its own
LogicalQuery today is flying blind:

1. **Schema blindness** — `RunLogicalQueryInput.LogicalQuery` is
   `map[string]any` (`internal/toolcontract/contract.go:132`); the MCP tool
   input schema tells the client nothing about the LogicalQuery shape.
2. **No model context** — `list_models` returns bare model rows; dimensions,
   metrics, synonyms, time grains, and enum values are not exposed via MCP.
3. **No self-correction loop** — there is no validate tool; validation errors
   are plain strings with no machine-readable codes or suggestions.
4. **Weak audit distinction** — `channel=mcp` exists, but nothing records
   whether a query's LogicalQuery was client-authored or AI-generated.

## Goals

- Make client-generated LogicalQuery a **first-class, documented mode**: the
  MCP client model can discover the model's queryable surface, author a
  LogicalQuery, validate it with structured repair feedback, and execute it.
- Keep the execution authority entirely server-side: the existing
  validate/RLS/PII/compile/read-only/audit chain is never bypassed or widened.
- Single tool contract shared by MCP and the web agent (via `toolcontract`).

## Non-Goals

- No new gating. Existing RBAC is sufficient: `run_logical_query` already
  requires PAT auth → `query:execute` permission → `RequireDatasourceAccess`
  → RLS/PII policy. No feature flag, no workspace allowlist, no role-based
  tool narrowing (decision: YAGNI).
- No raw SQL acceptance anywhere.
- No SQL preview in the validate response (deliberately excluded — do not leak
  compiled SQL to clients).
- No new DB tables or migrations.

## Chosen Approach

Extend the existing validator (`internal/query/validator.go`) to return
structured errors, expose them via a new `/api/query/validate` endpoint, and
add two new MCP tools. Alternatives rejected:

- *Handler-level string parsing* of existing error messages — brittle, silently
  breaks when validator text changes.
- *MCP-side validation* — violates the thin-gateway architecture (MCP is
  deliberately stateless and DB-less; every tool loops back through `/api/*`).

## Design

### 1. Tool set (6 → 8 tools, `internal/toolcontract/contract.go`)

Existing six tools unchanged. Two new tools:

- **`get_model_context(model_id)`** → new `GET /api/semantic/models/{id}/context`.
  Returns an LLM-optimized, single-call package:
  - dimensions: `name, label, type, synonyms, time_grains, enum_values`
  - metrics: `name, label, aggregation, synonyms, description`
  - join summary
  - **query constraints**: allowed filter operators per field type, valid time
    grains, `max_rows`, `CurrentLogicalQueryVersion`
  - Basis: `ModelFile` from `internal/semantic/export.go`, plus a constraints
    block.
- **`validate_logical_query(datasource_id, model_id, logical_query)`** → new
  `POST /api/query/validate`. Runs validate + compile (never executes).
  Response: `{valid, errors: [{code, field, message, suggestions[]}]}`.

### 2. Structured validator errors (`internal/query/validator.go`)

New type:

```go
type ValidationError struct {
    Code        string   // e.g. UNKNOWN_METRIC
    Field       string   // offending field/metric/dimension name
    Message     string   // human-readable, matches today's text
    Suggestions []string // up to 3 similar allowed names
}
```

Error codes: `UNKNOWN_METRIC`, `UNKNOWN_DIMENSION`, `UNKNOWN_FIELD`,
`INVALID_OPERATOR`, `INVALID_TIME_GRAIN`, `LIMIT_EXCEEDED`,
`DISALLOWED_SCHEMA`, and codes for the remaining existing validation failures.

Suggestions: simple similarity over allowed metric/dimension names plus
synonyms (contains/prefix match + Levenshtein distance, max 3 suggestions).

**Backward compatibility:** `ValidationError.Error()` produces today's string;
existing callers see no behavior change.

### 3. Security — the unchanged chain

Execution stays exactly today's `run_logical_query` → `/api/query/run` path:
PAT auth → `query:execute` permission → `RequireDatasourceAccess` → validate →
RLS injection → PII masking → read-only tx → row limit/timeout → audit.

Two security details specific to the new tools:

- `get_model_context` resolves the **caller's** PII policy
  (`PIIPolicyService.QueryPolicy`): fully hidden columns are **omitted** from
  the context; masked columns are flagged `masked: true`. The client model
  must not learn about fields it cannot see.
- `validate_logical_query` returns only `valid`/`errors` — never compiled SQL.

### 4. Schema visibility

`RunLogicalQueryInput.LogicalQuery` stays `map[string]any` at the transport
level, but the MCP tool `inputSchema` embeds the LogicalQuery v1 core schema
(`select`, `filters`, `group_by`, `having`, `order_by`, `limit`, `offset`).
Advanced fields (CTEs, window, case, formula, subqueries) are noted in the
description rather than fully schematized.

Tool descriptions state the contract explicitly:

> Use this tool only with valid LogicalQuery JSON. Never provide raw SQL. Only
> use fields, dimensions, and metrics exposed by `get_model_context`. The
> server validates permissions and may reject or rewrite unsafe requests.

### 5. Audit

`/api/query/run` audit details gain `origin: "client_logical_query"`; the AI
pipeline path records `origin: "ai_generated"`. Combined with the existing
`channel` (ui/api/mcp/agent/internal), this distinguishes who authored each
executed query. No schema change — the `details` JSON column suffices.

### 6. Web agent parity

Both new tools are added to `toolcontract.AllTools`, so they surface in the
web agent automatically via `internal/agent/web_tools.go`. The
`internal/agent/policy.go` allowlist gains the two new tool names. One
contract, two surfaces.

### 7. Testing

- Validator unit tests: structured errors, error codes, suggestion quality,
  backward-compatible `Error()` strings.
- Handler tests: `POST /api/query/validate` (valid, invalid-with-suggestions,
  permission-denied) and `GET /api/semantic/models/{id}/context` (including
  PII hiding/masking behavior).
- Toolcontract dispatch tests for both new tools.
- MCP end-to-end test: happy path + repair loop (invalid metric → suggestions
  → corrected query → valid → execute).

## Success Criteria

- An MCP client model can, with only the 8 tools, go from zero knowledge to a
  successfully executed LogicalQuery: `list_models` → `get_model_context` →
  author → `validate_logical_query` (repair if needed) → `run_logical_query`.
- Validation errors for unknown metrics/dimensions include at least one
  correct suggestion when a close match exists.
- `get_model_context` never reveals columns fully hidden by the caller's PII
  policy.
- Audit rows for MCP-executed client-authored queries carry
  `channel=mcp, origin=client_logical_query`.
- All existing callers of the validator and `/api/query/run` observe no
  behavior change (`make lint-go`, `make test-go` green).
