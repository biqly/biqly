# MCP Client-Generated LogicalQuery Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make client-generated LogicalQuery a first-class MCP mode: two new governed tools (`get_model_context`, `validate_logical_query`), structured validation surfaced over HTTP, schema visibility, and audit origin tagging.

**Architecture:** The validator already produces structured `ValidationError{Field, Code, Message, Value, AllowedAlternatives}` values with similarity suggestions (`suggestAlternatives`) — but `mapQueryServiceError` flattens them to a string at the HTTP boundary. We add a `POST /api/query/validate` endpoint that surfaces them as JSON, a `GET /api/semantic/models/{id}/context` endpoint returning an LLM-optimized model package (PII-filtered), register both as tools in the shared `toolcontract` package (one contract → MCP + web agent), and tag audit events with a query origin.

**Tech Stack:** Go 1.26, chi router, `modelcontextprotocol/go-sdk` (MCP), sonic JSON, existing validator/compiler/PII chain.

**Spec:** `docs/superpowers/specs/2026-07-08-mcp-client-logical-query-design.md`

## Global Constraints

- Go 1.26 idioms: `errors.Is`/`errors.AsType[T]`, `new(expr)`, `for i := range n`, `min`/`max`, `slices.Contains`.
- Never use grep/rg for Go symbols — use gograph MCP tools (`gograph_query`, `gograph_context`, `gograph_plan`); run `gograph_review` with uncommitted=true after edits. A PreToolUse hook BLOCKS grep on Go files.
- Before each commit: `gofmt -w <touched .go files>` + `make lint-go` + `make test-go`. The `.githooks/pre-commit` hook runs the full gate and takes 2+ minutes — this is normal, do NOT assume it hung.
- **Do NOT push.** Commit locally only. Another AI agent is working in this repo (T7 finalizer, currently modifying `internal/http/handlers/ai_agent_chat.go` and its test). NEVER `git add` those files or commit their hunks; stage only files this plan touches, by explicit path.
- No new gating/flags (decision: existing RBAC suffices). No raw SQL anywhere. `validate_logical_query` never returns compiled SQL.
- Existing callers must observe zero behavior change (`ValidationErrors.Error()` strings unchanged, `/api/query/run` response unchanged).
- The response field for suggestions is the EXISTING `allowed_alternatives` JSON key (spec's "suggestions[]" maps to it — do not rename).
- New LogicalQuery-facing JSON uses the existing field names from `pkg/query/types.go:149-155`.

---

### Task 1: Exported helpers in `internal/query` (PII access lookup + operator lists)

The model-context endpoint (Task 3) needs to ask "is this column hidden/masked for this user?" from the `handlers` package, and needs the legal operator lists. Both exist but are unexported (`cfg.lookup` in `pii_masking.go:72`, `validFilterOps`/`havingOps` in `validator.go:16-23`).

**Files:**
- Modify: `internal/query/pii_masking.go` (add exported method after `lookupStrategy`, ~line 102)
- Modify: `internal/query/validator.go` (add two exported funcs after `validFilterOps`, ~line 24)
- Test: `internal/query/pii_masking_access_test.go` (create)

**Interfaces:**
- Produces: `func (cfg *PIIMaskingConfig) AccessForColumnRef(refs ...string) (access string, found bool)` — nil-safe; returns effective access (`"raw"|"masked"|"hidden"`) for the first matching ref.
- Produces: `func ValidFilterOperators() []string`, `func ValidHavingOperators() []string` — defensive copies.

- [ ] **Step 1: Write the failing test**

```go
// internal/query/pii_masking_access_test.go
package query

import (
	"slices"
	"testing"
)

func TestAccessForColumnRef(t *testing.T) {
	cfg := &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"customers.email": "masked",
			"public.customers.ssn": "hidden",
		},
	}
	if access, ok := cfg.AccessForColumnRef("customers.email"); !ok || access != "masked" {
		t.Fatalf("email: got (%q, %v), want (masked, true)", access, ok)
	}
	if access, ok := cfg.AccessForColumnRef("customers.ssn", "public.customers.ssn"); !ok || access != "hidden" {
		t.Fatalf("ssn: got (%q, %v), want (hidden, true)", access, ok)
	}
	if _, ok := cfg.AccessForColumnRef("customers.name"); ok {
		t.Fatal("name: want not found")
	}
	var nilCfg *PIIMaskingConfig
	if _, ok := nilCfg.AccessForColumnRef("x"); ok {
		t.Fatal("nil config: want not found")
	}
}

func TestValidOperatorLists(t *testing.T) {
	fo := ValidFilterOperators()
	if !slices.Contains(fo, OpContains) || !slices.Contains(fo, OpBetween) {
		t.Fatalf("filter operators missing entries: %v", fo)
	}
	ho := ValidHavingOperators()
	if slices.Contains(ho, OpContains) {
		t.Fatalf("having operators must not contain %q", OpContains)
	}
	fo[0] = "mutated" // must not affect the source slice
	if ValidFilterOperators()[0] == "mutated" {
		t.Fatal("ValidFilterOperators must return a copy")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/query/ -run 'TestAccessForColumnRef|TestValidOperatorLists' -v`
Expected: FAIL — `cfg.AccessForColumnRef undefined`, `ValidFilterOperators undefined`.

- [ ] **Step 3: Implement**

In `internal/query/pii_masking.go` (after `lookupStrategy`):

```go
// AccessForColumnRef resolves the effective PII access level ("raw" | "masked"
// | "hidden") for the first matching column reference. Exported for read-side
// consumers (model context) that must not reveal hidden columns. Nil-safe: a
// nil config reports not found (no policy → raw access semantics decided by
// the caller).
func (cfg *PIIMaskingConfig) AccessForColumnRef(refs ...string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	access, _, ok := cfg.lookup(refs...)
	return access, ok
}
```

In `internal/query/validator.go` (after the `validFilterOps` var):

```go
// ValidFilterOperators returns the operators legal in a WHERE-clause filter.
// Returns a copy so callers cannot mutate validation behavior.
func ValidFilterOperators() []string {
	return slices.Clone(validFilterOps)
}

// ValidHavingOperators returns the operators legal in a HAVING clause.
func ValidHavingOperators() []string {
	return slices.Clone(havingOps)
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/query/ -run 'TestAccessForColumnRef|TestValidOperatorLists' -v`
Expected: PASS

- [ ] **Step 5: gofmt + lint + full package test + commit**

```bash
gofmt -w internal/query/pii_masking.go internal/query/validator.go internal/query/pii_masking_access_test.go
make lint-go && go test ./internal/query/...
git add internal/query/pii_masking.go internal/query/validator.go internal/query/pii_masking_access_test.go
git commit -m "feat(query): export PII column access lookup and operator lists"
```

---

### Task 2: `POST /api/query/validate` endpoint (structured errors, no execution)

**Files:**
- Modify: `internal/http/handlers/query.go` (add `Validate` method after `Compile`, ~line 58)
- Modify: `internal/http/query_router.go:66` (add route)
- Test: `internal/http/handlers/query_validate_test.go` (create)

**Interfaces:**
- Consumes: `internalQueryRunner.CompileWithModel` (interface at `internal/http/handlers/internal_ports.go:35-44`); `query.ValidationErrors` (`pkg/query/types.go:163`, aliased via `internal/query`).
- Produces: `POST /api/query/validate` accepting the same payload as `/query/run` (`{"logical_query": {...}}`), returning `200 {"valid": true}` or `200 {"valid": false, "errors": [{"field","code","message","value","allowed_alternatives"}]}`. Non-validation failures (model not found, policy load error) keep their existing status codes via `writeServiceError`.

- [ ] **Step 1: Write the failing test**

```go
// internal/http/handlers/query_validate_test.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// fakeQueryRunner returns canned results; only CompileWithModel is exercised.
type fakeQueryRunner struct {
	compileErr error // wrapped into a ServiceError when non-nil
}

func (f *fakeQueryRunner) Compile(ctx context.Context, lq *query.LogicalQuery) (*core.CompileResult, *core.ServiceError) {
	return f.CompileWithModel(ctx, lq, nil)
}
func (f *fakeQueryRunner) Run(ctx context.Context, lq *query.LogicalQuery) (*core.RunResult, *core.ServiceError) {
	return nil, nil
}
func (f *fakeQueryRunner) RunWithModel(ctx context.Context, lq *query.LogicalQuery, m *semantic.SemanticModel) (*core.RunResult, *core.ServiceError) {
	return nil, nil
}
func (f *fakeQueryRunner) CompileWithModel(_ context.Context, _ *query.LogicalQuery, _ *semantic.SemanticModel) (*core.CompileResult, *core.ServiceError) {
	if f.compileErr != nil {
		return nil, core.ToServiceError(f.compileErr)
	}
	return &core.CompileResult{}, nil
}

func newValidateHandler(runner internalQueryRunner) *QueryHandler {
	h := NewQueryHandler(&app.QueryDeps{})
	h.query = runner
	return h
}

const validateBody = `{"logical_query":{"datasource_id":"ds1","model_id":"m1","select":[{"type":"metric","name":"revenu"}]}}`

func TestValidateReturnsStructuredErrors(t *testing.T) {
	valErrs := query.ValidationErrors{{
		Field:               "select",
		Code:                "UNKNOWN_METRIC",
		Message:             "unknown metric: revenu",
		Value:               "revenu",
		AllowedAlternatives: []string{"revenue"},
	}}
	h := newValidateHandler(&fakeQueryRunner{compileErr: valErrs})

	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", strings.NewReader(validateBody))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Valid  bool                     `json:"valid"`
		Errors []*query.ValidationError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Valid {
		t.Fatal("valid = true, want false")
	}
	if len(resp.Errors) != 1 || resp.Errors[0].Code != "UNKNOWN_METRIC" {
		t.Fatalf("errors = %+v", resp.Errors)
	}
	if len(resp.Errors[0].AllowedAlternatives) == 0 || resp.Errors[0].AllowedAlternatives[0] != "revenue" {
		t.Fatalf("allowed_alternatives = %v, want [revenue]", resp.Errors[0].AllowedAlternatives)
	}
	if strings.Contains(rec.Body.String(), `"sql"`) {
		t.Fatal("validate response must never contain compiled SQL")
	}
}

func TestValidateReturnsValidTrue(t *testing.T) {
	h := newValidateHandler(&fakeQueryRunner{})
	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", strings.NewReader(validateBody))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] != true {
		t.Fatalf("valid = %v, want true", resp["valid"])
	}
}

func TestValidateNonValidationErrorKeepsStatus(t *testing.T) {
	h := newValidateHandler(&fakeQueryRunner{compileErr: core.ErrModelIDRequired})
	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", strings.NewReader(validateBody))
	rec := httptest.NewRecorder()
	h.Validate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

Notes for the implementer:
- Check `internal/core/errors.go` for the exact sentinel name (`ErrModelIDRequired` is referenced from `internal/core/service_error.go:64`).
- If `NewQueryHandler(&app.QueryDeps{})` panics on nil fields, construct `&QueryHandler{query: runner}` directly instead (same package).
- `query.ValidationErrors` / `query.ValidationError` are re-exports of `pkg/query/types.go` — confirm the alias exists in `internal/query/logical.go` (gograph shows `internal/query/result.go:23-24`).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/http/handlers/ -run TestValidate -v`
Expected: FAIL — `h.Validate undefined`.

- [ ] **Step 3: Implement the handler**

In `internal/http/handlers/query.go`, after `Compile` (add `"errors"` to imports):

```go
// Validate checks a LogicalQuery against the semantic model and security
// policy without executing it. Validation failures return HTTP 200 with
// structured, machine-readable errors (code + allowed_alternatives) so MCP
// clients and agents can self-correct; the compiled SQL is never returned.
func (h *QueryHandler) Validate(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeQueryPayload(w, r)
	if !ok {
		return
	}

	start := time.Now()
	_, se := h.query.CompileWithModel(r.Context(), &payload.LogicalQuery, payload.Model)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		if valErrs, isValidation := errors.AsType[query.ValidationErrors](error(se)); isValidation {
			writeJSON(w, http.StatusOK, map[string]any{"valid": false, "errors": valErrs})
			return
		}
		writeServiceError(r.Context(), w, se)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}
```

(`errors.AsType` walks `ServiceError.Unwrap()` → the original `ValidationErrors` set as `cause` by `MapQueryServiceError`, `internal/core/service_error.go:48-60`.)

In `internal/http/query_router.go`, after the `/query/run` line (66):

```go
	r.With(dsAccess).Post("/query/validate", queryHandler.Validate)
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/http/handlers/ -run TestValidate -v`
Expected: PASS

- [ ] **Step 5: End-to-end repair-loop test with the REAL validator**

Append to `query_validate_test.go` — this proves the full loop: invalid metric → suggestion → corrected query → valid. Uses `core.QueryService` with only Validator wired via a compile that fails before driver use. Simpler and robust: call the real `query.NewValidator` directly through a `fakeQueryRunner` variant that runs it:

```go
// validatingRunner runs the real validator against a fixed model, mirroring
// what core.QueryService.CompileWithContext does before compilation.
type validatingRunner struct {
	fakeQueryRunner
	model *semantic.SemanticModel
}

func (v *validatingRunner) CompileWithModel(_ context.Context, lq *query.LogicalQuery, _ *semantic.SemanticModel) (*core.CompileResult, *core.ServiceError) {
	if err := query.NewValidator(10000).Validate(lq, v.model); err != nil {
		return nil, core.ToServiceError(err)
	}
	return &core.CompileResult{}, nil
}

func TestValidateRepairLoop(t *testing.T) {
	model := &semantic.SemanticModel{
		Name: "sales",
		Metrics: []semantic.Metric{{Name: "revenue", Aggregation: "sum", IsActive: true}},
		Dimensions: []semantic.Dimension{{Name: "region", Type: "text", IsActive: true}},
	}
	h := newValidateHandler(&validatingRunner{model: model})

	// Round 1: misspelled metric → UNKNOWN_METRIC with "revenue" suggested.
	bad := `{"logical_query":{"datasource_id":"ds1","model_id":"m1","select":[{"type":"metric","name":"revenu"}]}}`
	rec := httptest.NewRecorder()
	h.Validate(rec, httptest.NewRequest(http.MethodPost, "/api/query/validate", strings.NewReader(bad)))
	var resp struct {
		Valid  bool                     `json:"valid"`
		Errors []*query.ValidationError `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || resp.Valid {
		t.Fatalf("round 1: err=%v resp=%+v body=%s", err, resp, rec.Body.String())
	}
	found := false
	for _, e := range resp.Errors {
		if e.Code == "UNKNOWN_METRIC" && slices.Contains(e.AllowedAlternatives, "revenue") {
			found = true
		}
	}
	if !found {
		t.Fatalf("round 1: want UNKNOWN_METRIC suggesting revenue, got %+v", resp.Errors)
	}

	// Round 2: corrected query → valid.
	good := `{"logical_query":{"datasource_id":"ds1","model_id":"m1","select":[{"type":"metric","name":"revenue"}]}}`
	rec = httptest.NewRecorder()
	h.Validate(rec, httptest.NewRequest(http.MethodPost, "/api/query/validate", strings.NewReader(good)))
	var ok map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil || ok["valid"] != true {
		t.Fatalf("round 2: err=%v body=%s", err, rec.Body.String())
	}
}
```

(Add `"slices"` to test imports. Check `semantic.Metric`/`semantic.Dimension` field names against `internal/semantic` — `IsActive` exists per `export.go:105,130`; the internal structs may differ from `pkg/semantic` — use gograph_source on `semantic.SemanticModel` first.)

Run: `go test ./internal/http/handlers/ -run TestValidateRepairLoop -v`
Expected: PASS

- [ ] **Step 6: gofmt + lint + commit**

```bash
gofmt -w internal/http/handlers/query.go internal/http/query_router.go internal/http/handlers/query_validate_test.go
make lint-go && go test ./internal/http/...
git add internal/http/handlers/query.go internal/http/query_router.go internal/http/handlers/query_validate_test.go
git commit -m "feat(query): add POST /api/query/validate with structured repair errors"
```

---

### Task 3: `GET /api/semantic/models/{id}/context` (LLM-optimized model context)

**Files:**
- Create: `pkg/logicalquery/schema.go` (LogicalQuery v1 core JSON schema constant)
- Create: `internal/http/handlers/semantic_context.go` (response types + pure builder + handler)
- Modify: `internal/http/catalog_router.go:114` (add route with `modelRead` middleware)
- Test: `internal/http/handlers/semantic_context_test.go` (create)

**Interfaces:**
- Consumes: `PIIMaskingConfig.AccessForColumnRef` (Task 1), `query.ValidFilterOperators()`, `query.ValidHavingOperators()` (Task 1), `h.deps.PIIPolicies.QueryPolicy(ctx, datasourceID)` (`internal/core/pii_policy.go:60`), `h.deps.SemanticRepo.GetPublishedFullModel`, `h.deps.Config.Query.MaxRows`, `logicalquery.CurrentLogicalQueryVersion` + `logicalquery.CoreSchemaJSON`.
- Produces: `GET /api/semantic/models/{id}/context` → `modelContextResponse` JSON (shape below). Later tasks reference the endpoint path only.

Response shape (write exactly this):

```json
{
  "model_id": "…", "name": "…", "label": "…", "description": "…",
  "datasource_id": "…",
  "dimensions": [
    {"name":"…","label":"…","type":"date","time_grain":"month","synonyms":["…"],
     "description":"…","enum_values":[{"raw_value":"…","label":"…"}],"masked":true}
  ],
  "metrics": [
    {"name":"…","label":"…","aggregation":"sum","synonyms":["…"],"description":"…","format":"…"}
  ],
  "joins": [{"name":"…","from_table":"…","to_table":"…","join_type":"left","relationship":"many_to_one"}],
  "constraints": {
    "logical_query_version": "v1",
    "max_rows": 10000,
    "time_grains": ["hour","day","week","month","quarter","year"],
    "filter_operators": ["…"],
    "having_operators": ["…"]
  },
  "logical_query_schema": { }
}
```

Deliberate exclusions (security-lean, note in code comments): metric `expression` (raw SQL fragment), dimension `column_ref` (physical detail — clients author by name), schemas/tables beyond join names. Dimensions fully hidden by the caller's PII policy are OMITTED; masked ones get `"masked": true`.

- [ ] **Step 1: Write `pkg/logicalquery/schema.go`**

```go
package logicalquery

// CoreSchemaJSON is the JSON Schema for the LogicalQuery v1 CORE surface:
// the fields an external author (MCP client model) needs to build governed
// queries. Advanced fields (ctes, from_subquery, window/case/formula select
// items) are intentionally not schematized here — they remain valid input and
// are documented in tool descriptions. Keep in sync with types.go.
const CoreSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Biqly LogicalQuery v1 (core)",
  "type": "object",
  "required": ["datasource_id", "select"],
  "properties": {
    "version": {"type": "string", "const": "v1"},
    "datasource_id": {"type": "string"},
    "model_id": {"type": "string"},
    "select": {
      "type": "array", "minItems": 1,
      "items": {
        "type": "object", "required": ["type", "name"],
        "properties": {
          "type": {"enum": ["dimension", "metric"]},
          "name": {"type": "string", "description": "dimension or metric name from get_model_context"},
          "alias": {"type": "string"}
        }
      }
    },
    "filters": {
      "type": "array",
      "items": {
        "type": "object", "required": ["field", "operator"],
        "properties": {
          "field": {"type": "string"},
          "operator": {"enum": ["eq","neq","gt","gte","lt","lte","in","not_in","contains","starts_with","ends_with","between","is_null","is_not_null","is_empty","is_not_empty"]},
          "value": {}
        }
      }
    },
    "group_by": {
      "type": "array",
      "items": {
        "type": "object", "required": ["field"],
        "properties": {
          "field": {"type": "string"},
          "time_grain": {"enum": ["hour","day","week","month","quarter","year"]}
        }
      }
    },
    "having": {"type": "array", "items": {"type": "object", "required": ["field", "operator"], "properties": {"field": {"type": "string"}, "operator": {"enum": ["eq","neq","gt","gte","lt","lte","between","is_null","is_not_null"]}, "value": {}}}},
    "order_by": {"type": "array", "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}, "direction": {"enum": ["asc", "desc"]}}}},
    "limit": {"type": "integer", "minimum": 0},
    "offset": {"type": "integer", "minimum": 0}
  }
}`
```

**Verify enum values against `pkg/logicalquery/types.go:239-295`** (operator constants `OpEq`… and `OrderAsc`/`OrderDesc`, time grains at 213-220) — fix any mismatch in the schema, not the types.

- [ ] **Step 2: Write the failing test for the pure builder**

```go
// internal/http/handlers/semantic_context_test.go
package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func strPtr(s string) *string { return &s }

func testContextModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		ID: "m1", DatasourceID: "ds1", Name: "sales", BaseSchema: "public", BaseTable: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "region", Type: "text", ColumnRef: "orders.region", IsActive: true},
			{Name: "email", Type: "text", ColumnRef: "orders.email", IsActive: true},
			{Name: "ssn", Type: "text", ColumnRef: "orders.ssn", IsActive: true},
			{Name: "inactive_dim", Type: "text", ColumnRef: "orders.x", IsActive: false},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Aggregation: "sum", Expression: "SUM(amount)", IsActive: true},
		},
	}
}

func TestBuildModelContextAppliesPIIPolicy(t *testing.T) {
	piiCfg := &query.PIIMaskingConfig{ColumnAccess: map[string]string{
		"orders.email": "masked",
		"orders.ssn":   "hidden",
	}}
	ctxResp := buildModelContext(testContextModel(), piiCfg, 10000)

	names := map[string]bool{}
	for _, d := range ctxResp.Dimensions {
		names[d.Name] = true
		if d.Name == "email" && !d.Masked {
			t.Fatal("email must be flagged masked")
		}
		if d.Name == "region" && d.Masked {
			t.Fatal("region must not be masked")
		}
	}
	if names["ssn"] {
		t.Fatal("hidden dimension ssn must be omitted from context")
	}
	if names["inactive_dim"] {
		t.Fatal("inactive dimensions must be omitted")
	}
	if len(ctxResp.Metrics) != 1 || ctxResp.Metrics[0].Name != "revenue" {
		t.Fatalf("metrics = %+v", ctxResp.Metrics)
	}
	if ctxResp.Constraints.MaxRows != 10000 || ctxResp.Constraints.LogicalQueryVersion != "v1" {
		t.Fatalf("constraints = %+v", ctxResp.Constraints)
	}
	if len(ctxResp.Constraints.FilterOperators) == 0 {
		t.Fatal("filter_operators must be populated")
	}
	if len(ctxResp.LogicalQuerySchema) == 0 {
		t.Fatal("logical_query_schema must be embedded")
	}
}

func TestBuildModelContextNeverEmitsExpressions(t *testing.T) {
	ctxResp := buildModelContext(testContextModel(), nil, 100)
	for _, m := range ctxResp.Metrics {
		_ = m // modelContextMetric has no Expression field — compile-time guarantee.
	}
	if ctxResp.Constraints.MaxRows != 100 {
		t.Fatalf("max_rows = %d", ctxResp.Constraints.MaxRows)
	}
}
```

(Adjust struct literals to `internal/semantic` field types — `Label`/`Description` are `*string` per `export.go:96-97`; use `strPtr` where needed. Verify with `gograph_source semantic.SemanticModel` / `semantic.Dimension` / `semantic.Metric` before writing.)

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/http/handlers/ -run TestBuildModelContext -v`
Expected: FAIL — `buildModelContext undefined`.

- [ ] **Step 4: Implement builder + handler**

`internal/http/handlers/semantic_context.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/logicalquery"
)

// modelContextResponse is the LLM-optimized, single-call package an MCP
// client model needs to author a LogicalQuery: the queryable surface
// (dimensions, metrics, joins), the server's query constraints, and the
// LogicalQuery core JSON schema. Metric expressions and physical column refs
// are deliberately excluded; dimensions hidden by the caller's PII policy are
// omitted entirely so clients cannot learn about fields they may not see.
type modelContextResponse struct {
	ModelID            string                  `json:"model_id"`
	Name               string                  `json:"name"`
	Label              string                  `json:"label,omitempty"`
	Description        string                  `json:"description,omitempty"`
	DatasourceID       string                  `json:"datasource_id"`
	Dimensions         []modelContextDimension `json:"dimensions"`
	Metrics            []modelContextMetric    `json:"metrics"`
	Joins              []modelContextJoin      `json:"joins,omitempty"`
	Constraints        modelContextConstraints `json:"constraints"`
	LogicalQuerySchema json.RawMessage         `json:"logical_query_schema"`
}

type modelContextDimension struct {
	Name        string                  `json:"name"`
	Label       string                  `json:"label,omitempty"`
	Type        string                  `json:"type"`
	TimeGrain   string                  `json:"time_grain,omitempty"`
	Synonyms    []string                `json:"synonyms,omitempty"`
	Description string                  `json:"description,omitempty"`
	EnumValues  []modelContextEnumValue `json:"enum_values,omitempty"`
	Masked      bool                    `json:"masked,omitempty"`
}

type modelContextEnumValue struct {
	RawValue string `json:"raw_value"`
	Label    string `json:"label"`
}

type modelContextMetric struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Aggregation string   `json:"aggregation"`
	Synonyms    []string `json:"synonyms,omitempty"`
	Description string   `json:"description,omitempty"`
	Format      string   `json:"format,omitempty"`
}

type modelContextJoin struct {
	Name         string `json:"name"`
	FromTable    string `json:"from_table"`
	ToTable      string `json:"to_table"`
	JoinType     string `json:"join_type"`
	Relationship string `json:"relationship"`
}

type modelContextConstraints struct {
	LogicalQueryVersion string   `json:"logical_query_version"`
	MaxRows             int      `json:"max_rows"`
	TimeGrains          []string `json:"time_grains"`
	FilterOperators     []string `json:"filter_operators"`
	HavingOperators     []string `json:"having_operators"`
}

// buildModelContext assembles the context response, applying the caller's PII
// policy: hidden dimensions are dropped, masked ones flagged. piiCfg may be nil
// (no policy → everything raw).
func buildModelContext(m *semantic.SemanticModel, piiCfg *query.PIIMaskingConfig, maxRows int) modelContextResponse {
	resp := modelContextResponse{
		ModelID:      m.ID,
		Name:         m.Name,
		Label:        derefStr(m.Label),
		Description:  derefStr(m.Description),
		DatasourceID: m.DatasourceID,
		Dimensions:   []modelContextDimension{},
		Metrics:      []modelContextMetric{},
		Constraints: modelContextConstraints{
			LogicalQueryVersion: logicalquery.CurrentLogicalQueryVersion,
			MaxRows:             maxRows,
			TimeGrains:          []string{"hour", "day", "week", "month", "quarter", "year"},
			FilterOperators:     query.ValidFilterOperators(),
			HavingOperators:     query.ValidHavingOperators(),
		},
		LogicalQuerySchema: json.RawMessage(logicalquery.CoreSchemaJSON),
	}
	for i := range m.Dimensions {
		d := &m.Dimensions[i]
		if !d.IsActive {
			continue
		}
		access, found := piiCfg.AccessForColumnRef(d.ColumnRef, m.BaseSchema+"."+d.ColumnRef)
		if found && access != pii.AccessRaw && access != pii.AccessMasked {
			continue // hidden for this caller: do not reveal existence
		}
		cd := modelContextDimension{
			Name:        d.Name,
			Label:       derefStr(d.Label),
			Type:        d.Type,
			TimeGrain:   d.TimeGrain,
			Synonyms:    d.Synonyms,
			Description: derefStr(d.Description),
			Masked:      found && access == pii.AccessMasked,
		}
		for _, ev := range d.EnumValues {
			cd.EnumValues = append(cd.EnumValues, modelContextEnumValue{RawValue: ev.RawValue, Label: ev.Label})
		}
		resp.Dimensions = append(resp.Dimensions, cd)
	}
	for i := range m.Metrics {
		mt := &m.Metrics[i]
		if !mt.IsActive {
			continue
		}
		resp.Metrics = append(resp.Metrics, modelContextMetric{
			Name:        mt.Name,
			Label:       derefStr(mt.Label),
			Aggregation: mt.Aggregation,
			Synonyms:    mt.Synonyms,
			Description: derefStr(mt.Description),
			Format:      derefStr(mt.Format),
		})
	}
	for i := range m.Joins {
		j := &m.Joins[i]
		if !j.IsActive {
			continue
		}
		resp.Joins = append(resp.Joins, modelContextJoin{
			Name: j.Name, FromTable: j.FromTable, ToTable: j.ToTable,
			JoinType: j.JoinType, Relationship: j.Relationship,
		})
	}
	return resp
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GetModelContext returns the published model's LLM-optimized query-authoring
// context for the calling user. 404s when the model has no published version
// (unpublished models cannot be queried, so there is nothing to author against).
func (h *SemanticHandler) GetModelContext(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetPublishedFullModel(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	piiCfg, _, err := h.deps.PIIPolicies.QueryPolicy(ctx, model.DatasourceID)
	if err != nil {
		// Fail closed: without a resolvable policy we must not emit column info.
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve security policy", err)
		return
	}
	writeJSON(w, http.StatusOK, buildModelContext(model, piiCfg, h.deps.Config.Query.MaxRows))
}
```

Notes: check whether a `deref` helper already exists in `handlers` (avoid duplicate name — `derefStr` chosen to dodge `internal/semantic.deref`). Verify `pii.AccessRaw`/`pii.AccessMasked` constants exist (`internal/security/pii` — referenced at `pii_policy.go:139`). Verify `SemanticRepo.GetPublishedFullModel` exists (it satisfies `core.ModelLoader`, `internal/core/query_service.go:27-29`); if the repo method has a different name, use gograph to find the `ModelLoader` implementer and call that.

In `internal/http/catalog_router.go` (after line 114, `/fields` route):

```go
	r.With(modelRead).Get("/semantic/models/{id}/context", semHandler.GetModelContext)
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/http/handlers/ -run TestBuildModelContext -v`
Expected: PASS

- [ ] **Step 6: gofmt + lint + commit**

```bash
gofmt -w pkg/logicalquery/schema.go internal/http/handlers/semantic_context.go internal/http/catalog_router.go internal/http/handlers/semantic_context_test.go
make lint-go && go test ./internal/http/... ./pkg/...
git add pkg/logicalquery/schema.go internal/http/handlers/semantic_context.go internal/http/catalog_router.go internal/http/handlers/semantic_context_test.go
git commit -m "feat(semantic): add GET /api/semantic/models/{id}/context for query authoring"
```

---

### Task 4: toolcontract — two new tools, dispatch helpers, contract-grade descriptions

**Files:**
- Modify: `internal/toolcontract/contract.go`
- Test: `internal/toolcontract/contract_test.go` (extend if exists — check with `ls internal/toolcontract/`; create otherwise)

**Interfaces:**
- Produces: `ToolGetModelContext ToolName = "get_model_context"`, `ToolValidateLogicalQuery ToolName = "validate_logical_query"`; `GetModelContextInput{ModelID string}`, `ValidateLogicalQueryInput{LogicalQuery map[string]any}`; `DispatchGetModelContext(ctx, disp, in, cred, channel)`, `DispatchValidateLogicalQuery(ctx, disp, in, cred, channel)`. `AllTools` grows to 8 entries (stable order: insert `get_model_context` after `list_models`, `validate_logical_query` before `run_logical_query`).

- [ ] **Step 1: Write the failing test**

```go
// internal/toolcontract/contract_test.go (append or create; package toolcontract)
package toolcontract

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

type recordingDispatcher struct {
	method, path string
	body         any
}

func (r *recordingDispatcher) Dispatch(_ context.Context, method, path string, body any, _ Credential, _ string) (DispatchResult, error) {
	r.method, r.path, r.body = method, path, body
	return DispatchResult{StatusCode: 200, Body: json.RawMessage(`{}`)}, nil
}

func TestAllToolsHasEightGovernedTools(t *testing.T) {
	if len(AllTools) != 8 {
		t.Fatalf("AllTools = %d entries, want 8", len(AllTools))
	}
	names := map[ToolName]bool{}
	for _, spec := range AllTools {
		names[spec.Name] = true
	}
	for _, want := range []ToolName{ToolGetModelContext, ToolValidateLogicalQuery} {
		if !names[want] {
			t.Fatalf("AllTools missing %s", want)
		}
	}
}

func TestDispatchGetModelContext(t *testing.T) {
	rec := &recordingDispatcher{}
	_, err := DispatchGetModelContext(context.Background(), rec, GetModelContextInput{ModelID: "m 1"}, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodGet || rec.path != "/api/semantic/models/m%201/context" {
		t.Fatalf("dispatched %s %s", rec.method, rec.path)
	}
}

func TestDispatchValidateLogicalQuery(t *testing.T) {
	rec := &recordingDispatcher{}
	lq := map[string]any{"datasource_id": "ds1"}
	_, err := DispatchValidateLogicalQuery(context.Background(), rec, ValidateLogicalQueryInput{LogicalQuery: lq}, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatal(err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/validate" {
		t.Fatalf("dispatched %s %s", rec.method, rec.path)
	}
	body, _ := rec.body.(map[string]any)
	if _, ok := body["logical_query"]; !ok {
		t.Fatalf("body = %+v, want logical_query wrapper", rec.body)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/toolcontract/ -v`
Expected: FAIL — undefined `ToolGetModelContext` etc.

- [ ] **Step 3: Implement in `contract.go`**

Add tool name constants (after line 51):

```go
	ToolGetModelContext      ToolName = "get_model_context"
	ToolValidateLogicalQuery ToolName = "validate_logical_query"
```

Insert into `AllTools` — `get_model_context` right after the `list_models` entry, `validate_logical_query` right before `run_logical_query`:

```go
	{
		Name: ToolGetModelContext,
		Description: "Get the full query-authoring context for one semantic model: " +
			"dimensions (name, type, synonyms, time grains, enum values), metrics " +
			"(name, aggregation, synonyms), joins, server constraints (max rows, " +
			"allowed operators, time grains) and the LogicalQuery JSON schema. " +
			"Call this BEFORE authoring a LogicalQuery; only fields returned here " +
			"may be referenced. Columns you are not allowed to see are omitted.",
		Method: http.MethodGet,
		Path:   "/api/semantic/models/{id}/context",
	},
	{
		Name: ToolValidateLogicalQuery,
		Description: "Validate a LogicalQuery JSON document against the semantic model " +
			"and security policy WITHOUT executing it. Returns valid=true, or " +
			"valid=false with machine-readable errors (code, field, " +
			"allowed_alternatives suggestions) you can use to correct the query and " +
			"retry. Always validate before run_logical_query when authoring queries " +
			"yourself. Never returns SQL.",
		Method: http.MethodPost,
		Path:   "/api/query/validate",
	},
```

Update the `run_logical_query` description (replace existing, line 90-93) to state the authoring contract:

```go
		Description: "Compile and execute a Biqly LogicalQuery JSON document you authored " +
			"(client-generated mode). Use only fields, dimensions and metrics exposed by " +
			"get_model_context, and validate with validate_logical_query first. The " +
			"backend enforces the semantic model, permissions, row-level security, PII " +
			"masking and read-only execution; raw SQL is never accepted and unsafe " +
			"requests are rejected.",
```

Add input structs (after `RunLogicalQueryInput`):

```go
// GetModelContextInput identifies the model to fetch authoring context for.
type GetModelContextInput struct {
	ModelID string `json:"model_id" jsonschema:"id of the semantic model to fetch context for"`
}

// ValidateLogicalQueryInput validates a LogicalQuery document without executing it.
type ValidateLogicalQueryInput struct {
	LogicalQuery map[string]any `json:"logical_query" jsonschema:"the LogicalQuery document to validate"`
}
```

Add dispatch helpers (after `DispatchRunLogicalQuery`):

```go
// DispatchGetModelContext calls GET /api/semantic/models/{id}/context.
func DispatchGetModelContext(ctx context.Context, disp Dispatcher, in GetModelContextInput, cred Credential, channel string) (DispatchResult, error) {
	path := "/api/semantic/models/" + url.PathEscape(in.ModelID) + "/context"
	return disp.Dispatch(ctx, http.MethodGet, path, nil, cred, channel)
}

// DispatchValidateLogicalQuery calls POST /api/query/validate.
func DispatchValidateLogicalQuery(ctx context.Context, disp Dispatcher, in ValidateLogicalQueryInput, cred Credential, channel string) (DispatchResult, error) {
	return disp.Dispatch(ctx, http.MethodPost, "/api/query/validate", map[string]any{"logical_query": in.LogicalQuery}, cred, channel)
}
```

Also update the package doc comment and `ToolName` doc ("six governed tools" → "eight"). Search the file for the literal "six" and fix each.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/toolcontract/ -v`
Expected: PASS. Note: if an existing test asserts 6 tools, update it to 8.

- [ ] **Step 5: gofmt + lint + commit**

```bash
gofmt -w internal/toolcontract/contract.go internal/toolcontract/contract_test.go
make lint-go && go test ./internal/toolcontract/...
git add internal/toolcontract/contract.go internal/toolcontract/contract_test.go
git commit -m "feat(toolcontract): add get_model_context and validate_logical_query tools"
```

---

### Task 5: MCP server — register the 8 tools + embed LogicalQuery input schema

**Files:**
- Modify: `internal/http/mcp_server.go`
- Test: `internal/http/mcp_server_test.go` (extend if exists; check with `ls internal/http/mcp_server*`; create otherwise)

**Interfaces:**
- Consumes: `toolcontract.ToolGetModelContext`, `ToolValidateLogicalQuery`, `DispatchGetModelContext`, `DispatchValidateLogicalQuery` (Task 4); `logicalquery.CoreSchemaJSON` (Task 3).
- Produces: MCP server exposing 8 tools; `run_logical_query` and `validate_logical_query` advertise an explicit `inputSchema` whose `logical_query` property is the LogicalQuery v1 core schema.

- [ ] **Step 1: Inspect the MCP SDK's Tool.InputSchema contract**

Run: `gograph_query` for `AddTool` usage is not enough — read the SDK source:
```bash
ls $(go env GOMODCACHE)/github.com/modelcontextprotocol/ 2>/dev/null
```
Then Read the SDK's `mcp/tool.go` (or wherever `type Tool struct` lives) to confirm: (a) the `InputSchema` field type (`*jsonschema.Schema` from the SDK's own jsonschema package), and (b) that `mcp.AddTool` only infers a schema when `InputSchema` is nil. **If (b) does not hold** (AddTool always overwrites), skip the InputSchema embedding, keep schema exposure via `get_model_context.logical_query_schema` + descriptions only, and note the deviation in the commit message. The rest of this task proceeds either way.

- [ ] **Step 2: Write the failing test**

```go
// internal/http/mcp_server_test.go (package http; extend existing file if present)
package http

import (
	"net/http"
	"testing"

	"github.com/biqly/biqly/internal/toolcontract"
)

// TestMCPServerCoversAllContractTools fails when a toolcontract tool has no
// registration case in newMCPServer — the switch silently skips unknown names.
func TestMCPServerCoversAllContractTools(t *testing.T) {
	s := newMCPServer(http.NewServeMux(), "", "")
	if s == nil {
		t.Fatal("newMCPServer returned nil")
	}
	if got, want := len(toolcontract.AllTools), 8; got != want {
		t.Fatalf("AllTools = %d, want %d", got, want)
	}
	// Assert per-tool registration via the SDK's listing if available.
	// See implementer note below.
}
```

Implementer note: the exact assertion mechanism depends on what the SDK exposes — check first (Step 1 already has you reading the SDK source). Two workable options: (a) if `mcp.Server` exposes registered tools (e.g. a `ListTools` result or exported field), assert each `toolcontract.AllTools` name is present; (b) otherwise refactor `newMCPServer` to map each spec to its handler via a helper that returns `(handlerRegistered bool)` and fail the test when any spec is unhandled — e.g. make the `switch` a function `registerMCPTool(s, d, spec) bool` with `default: return false`, and have `newMCPServer` collect the names it registered into a package-level-testable slice return. Pick whichever is smaller; the acceptance criterion is: **the test fails if a toolcontract tool is not registered in the MCP server.** If an existing `mcp_server_test.go` already asserts registration, just extend its expected set.

- [ ] **Step 3: Implement**

In `newMCPServer` add the two cases to the switch (after `ToolRunLogicalQuery` case):

```go
		case toolcontract.ToolGetModelContext:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description}, d.getModelContext)
		case toolcontract.ToolValidateLogicalQuery:
			mcp.AddTool(s, &mcp.Tool{Name: string(spec.Name), Description: spec.Description, InputSchema: logicalQueryToolSchema()}, d.validateLogicalQuery)
```

And set the same `InputSchema: logicalQueryToolSchema()` on the existing `ToolRunLogicalQuery` case.

Add the handlers + schema helper:

```go
func (d *mcpToolDispatcher) getModelContext(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.GetModelContextInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchGetModelContext(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

func (d *mcpToolDispatcher) validateLogicalQuery(ctx context.Context, _ *mcp.CallToolRequest, in toolcontract.ValidateLogicalQueryInput) (*mcp.CallToolResult, any, error) {
	res, err := toolcontract.DispatchValidateLogicalQuery(ctx, d.disp, in, d.cred, toolcontract.ChannelMCP)
	return toMCPResult(res), nil, err
}

// logicalQueryToolSchema builds the MCP input schema advertising the
// LogicalQuery v1 core shape under the logical_query property, so client
// models see the document structure instead of an opaque object.
func logicalQueryToolSchema() *jsonschema.Schema {
	var lqSchema jsonschema.Schema
	// CoreSchemaJSON is a compile-time constant; unmarshal cannot fail at
	// runtime, but fall back to a permissive object schema defensively.
	if err := json.Unmarshal([]byte(logicalquery.CoreSchemaJSON), &lqSchema); err != nil {
		lqSchema = jsonschema.Schema{Type: "object"}
	}
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"logical_query"},
		Properties: map[string]*jsonschema.Schema{
			"logical_query": &lqSchema,
		},
	}
}
```

(Import paths: the go-sdk's jsonschema package — confirm from Step 1; plus `encoding/json` and `github.com/biqly/biqly/pkg/logicalquery`. Adjust field names to the actual `jsonschema.Schema` struct.)

Update the file's doc comments ("six governed tools" → "eight").

- [ ] **Step 4: Run to verify**

Run: `go test ./internal/http/ -run TestMCPServer -v && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: gofmt + lint + commit**

```bash
gofmt -w internal/http/mcp_server.go internal/http/mcp_server_test.go
make lint-go && go test ./internal/http/...
git add internal/http/mcp_server.go internal/http/mcp_server_test.go
git commit -m "feat(mcp): expose get_model_context and validate_logical_query tools with input schema"
```

---### Task 6: Web agent parity (policy allowlist + web tools + role narrowing)

⚠️ **Conflict guard:** `internal/http/handlers/ai_agent_chat.go` and `ai_agent_chat_test.go` have uncommitted changes from another agent (T7 finalizer). Before touching them, run `git status --porcelain internal/http/handlers/ai_agent_chat*`. If still dirty, implement ONLY `internal/agent/` changes in this task, commit them, and leave the `webAgentAllowedTools`/`webAgentRetryBudget` wiring as a deferred sub-task (record it at the bottom of this plan file under "Deferred") to apply after the other work lands. If clean, do everything.

**Files:**
- Modify: `internal/agent/policy.go` (consts ~line 35, Evaluate switch ~line 226, `webAgentTools` var ~line 236)
- Modify: `internal/agent/web_tools.go` (`All()` ~line 29, new dispatch fns ~line 160)
- Modify (conditional): `internal/http/handlers/ai_agent_chat.go:389-412` (`webAgentAllowedTools`, `webAgentRetryBudget`)
- Test: `internal/agent/web_tools_test.go` (extend), `internal/http/handlers/ai_agent_chat_test.go` (conditional, extend)

**Interfaces:**
- Consumes: Task 4's toolcontract symbols.
- Produces: `agent.ToolWebGetModelContext`, `agent.ToolWebValidateLogicalQuery`; `WebTools.All()` returns 8 tools.

- [ ] **Step 1: Write the failing test**

Extend `internal/agent/web_tools_test.go` (follow the file's existing fake-dispatcher pattern — read it first):

```go
func TestWebToolsAllReturnsEightTools(t *testing.T) {
	w := NewWebTools(nil, toolcontract.Credential{})
	tools := w.All()
	if len(tools) != 8 {
		t.Fatalf("All() = %d tools, want 8", len(tools))
	}
	names := map[ToolName]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	if !names[ToolWebGetModelContext] || !names[ToolWebValidateLogicalQuery] {
		t.Fatalf("missing new web tools in %v", names)
	}
}

func TestPolicyAllowsNewWebTools(t *testing.T) {
	// Mirror TestPolicyEngine_AllowsWebToolsWithoutIdentity's construction.
	for _, tool := range []ToolName{ToolWebGetModelContext, ToolWebValidateLogicalQuery} {
		if !isWebAgentTool(tool) {
			t.Fatalf("%s must be a web agent tool", tool)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/agent/ -run 'TestWebToolsAllReturnsEightTools|TestPolicyAllowsNewWebTools' -v`
Expected: FAIL — undefined constants.

- [ ] **Step 3: Implement `internal/agent` changes**

`policy.go` — extend the const block (~line 35):

```go
	ToolWebGetModelContext      ToolName = ToolName(toolcontract.ToolGetModelContext)
	ToolWebValidateLogicalQuery ToolName = ToolName(toolcontract.ToolValidateLogicalQuery)
```

Extend the Evaluate switch case (line 226-228) and `webAgentTools` (line 236-239) to include both new names. Update "six" wording in comments to "eight" / "MCP-parity".

`web_tools.go` — extend `All()`:

```go
		webTool{w, ToolWebGetModelContext, webGetModelContext},
		webTool{w, ToolWebValidateLogicalQuery, webValidateLogicalQuery},
```

Add dispatch fns (bottom, matching existing style):

```go
func webGetModelContext(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.GetModelContextInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchGetModelContext(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

func webValidateLogicalQuery(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.ValidateLogicalQueryInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchValidateLogicalQuery(ctx, disp, in, cred, toolcontract.ChannelAgent)
}
```

- [ ] **Step 4: (conditional — only if ai_agent_chat.go is clean) role narrowing**

In `webAgentAllowedTools` (`ai_agent_chat.go:389`): add `agent.ToolWebGetModelContext` to the base list (read-only, all roles); add `agent.ToolWebValidateLogicalQuery` next to `ToolWebRunLogicalQuery` inside the analyst/admin branch. In `webAgentRetryBudget` (line 403): `agent.ToolWebGetModelContext: 2, agent.ToolWebValidateLogicalQuery: 3` (validate is the repair loop — give it 3). Extend `TestWebAgentAllowedToolsNarrowsByRole` (`ai_agent_chat_test.go:433`) accordingly.

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/agent/... ./internal/http/handlers/ -run 'TestWebTools|TestPolicy|TestWebAgent' -v`
Expected: PASS

- [ ] **Step 6: gofmt + lint + commit (stage ONLY the files you touched)**

```bash
gofmt -w internal/agent/policy.go internal/agent/web_tools.go internal/agent/web_tools_test.go
make lint-go && go test ./internal/agent/... ./internal/http/...
git add internal/agent/policy.go internal/agent/web_tools.go internal/agent/web_tools_test.go
# plus ai_agent_chat.go + test ONLY if Step 4 ran on a clean file
git commit -m "feat(agent): web-agent parity for get_model_context and validate_logical_query"
```

---

### Task 7: Audit origin tagging

**Files:**
- Create: `internal/audit/origin.go`
- Modify: `internal/http/handlers/query.go` (`Run`, line 60)
- Modify: `internal/http/handlers/ai.go` (the `Run` method registered at `internal/http/ai_router.go:129` — find it with gograph: `gograph_query AIHandler` then locate `Run`)
- Modify: `internal/core/query_service.go` (`auditQueryExecution`, line 368)
- Test: `internal/audit/origin_test.go` (create)

**Interfaces:**
- Produces: `audit.OriginClientLogicalQuery = "client_logical_query"`, `audit.OriginAIGenerated = "ai_generated"`, `audit.WithQueryOrigin(ctx, origin) context.Context`, `audit.QueryOriginFromContext(ctx) string` (empty when unset).
- Known limitation (accepted, note in code comment): when the AI service runs split-out and executes via `/internal/query/run`, the origin context does not cross the process boundary — those events keep `channel=internal` as their identifying mark.

- [ ] **Step 1: Write the failing test**

```go
// internal/audit/origin_test.go
package audit

import (
	"context"
	"testing"
)

func TestQueryOriginContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	if got := QueryOriginFromContext(ctx); got != "" {
		t.Fatalf("unset origin = %q, want empty", got)
	}
	ctx = WithQueryOrigin(ctx, OriginClientLogicalQuery)
	if got := QueryOriginFromContext(ctx); got != OriginClientLogicalQuery {
		t.Fatalf("origin = %q, want %q", got, OriginClientLogicalQuery)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/audit/ -run TestQueryOrigin -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement**

`internal/audit/origin.go` (mirror `channel.go`'s pattern exactly):

```go
package audit

import "context"

// Query origin values distinguish who authored an executed LogicalQuery.
// Combined with the channel (ui/api/mcp/agent/internal) this makes each
// executed query attributable: e.g. channel=mcp + origin=client_logical_query
// is an MCP client model running a query it authored itself.
const (
	// OriginClientLogicalQuery marks a LogicalQuery authored by the caller
	// (MCP client model, agent, or API consumer) and submitted pre-built.
	OriginClientLogicalQuery = "client_logical_query"
	// OriginAIGenerated marks a LogicalQuery generated by Biqly's own AI
	// pipeline from a natural-language question.
	OriginAIGenerated = "ai_generated"
)

type queryOriginKeyType struct{}

var queryOriginKey queryOriginKeyType

// WithQueryOrigin tags the context with the LogicalQuery origin.
func WithQueryOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, queryOriginKey, origin)
}

// QueryOriginFromContext returns the tagged origin, or "" when unset.
func QueryOriginFromContext(ctx context.Context) string {
	if o, ok := ctx.Value(queryOriginKey).(string); ok {
		return o
	}
	return ""
}
```

`internal/http/handlers/query.go` — first lines of `Run` (after `decodeQueryPayload`):

```go
	// Queries arriving here are pre-built by the caller (MCP client model,
	// web agent, or API consumer) — tag the origin for audit attribution.
	r = r.WithContext(audit.WithQueryOrigin(r.Context(), audit.OriginClientLogicalQuery))
```

(Import `"github.com/biqly/biqly/internal/audit"`.)

`internal/http/handlers/ai.go` — at the top of the AI `Run` handler (the one serving `/ai/query/run`), same pattern with `audit.OriginAIGenerated`.

`internal/core/query_service.go` — in `auditQueryExecution`, right after the `details` map literal (line 380):

```go
	if origin := audit.QueryOriginFromContext(ctx); origin != "" {
		details["origin"] = origin
	}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/audit/ ./internal/core/ ./internal/http/... `
Expected: PASS (existing query_service tests unaffected — origin key absent means no `origin` detail).

- [ ] **Step 5: gofmt + lint + commit**

```bash
gofmt -w internal/audit/origin.go internal/audit/origin_test.go internal/http/handlers/query.go internal/http/handlers/ai.go internal/core/query_service.go
make lint-go && make test-go
git add internal/audit/origin.go internal/audit/origin_test.go internal/http/handlers/query.go internal/http/handlers/ai.go internal/core/query_service.go
git commit -m "feat(audit): tag executed queries with logical-query origin"
```

---

### Task 8: Full verification + docs sync

**Files:**
- Modify: `CONTEXT.md` and/or `docs/**` wherever the MCP tool list is documented (find with: `rg -l "run_logical_query" CONTEXT.md docs/ README.md` — plain-text files, the gograph guard only blocks Go-file greps)
- Modify: this plan file (mark Deferred items if Task 6 was partial)

- [ ] **Step 1: Repo-wide gates**

```bash
make precommit
deadcode -test $(go list ./... | grep -v '/frontend')
```
Expected: clean. `make precommit` takes minutes — wait it out. Triage any deadcode findings per CLAUDE.md rules (new exported helpers used by handlers/tests should not appear; if one does, wire or remove it).

- [ ] **Step 2: gograph review**

Run `gograph_review` with uncommitted=true (should be empty after commits) and `gograph_stale`; rebuild the index if stale so subsequent sessions see the new symbols.

- [ ] **Step 3: Docs sync**

Update every doc that enumerates the MCP tool set (six → eight, add one-line descriptions for the two new tools, document `POST /api/query/validate` + `GET /api/semantic/models/{id}/context` alongside the existing query/semantic endpoint lists). Document the two agent modes with the framing from the spec:

> Biqly supports two governed agent paths: **Question-first** (`run_question` — Biqly's AI generates the LogicalQuery) and **LogicalQuery-first** (`get_model_context` → author → `validate_logical_query` → `run_logical_query` — the client model authors the plan; Biqly validates, compiles and executes it safely).

- [ ] **Step 4: Success-criteria walkthrough (manual, local)**

With `make dev-up` + `make watch` running and a PAT:
1. `list_models` → pick a model id.
2. `GET /api/semantic/models/{id}/context` → confirm dimensions/metrics/constraints/schema present; PII-hidden columns absent.
3. `POST /api/query/validate` with a misspelled metric → `valid:false`, code `UNKNOWN_METRIC`, `allowed_alternatives` contains the right name.
4. Correct and re-validate → `valid:true`.
5. `POST /api/query/run` with the same document → rows returned.
6. Check `audit_events`: the run carries `details.origin = "client_logical_query"` (and `channel: "mcp"` when driven through the MCP endpoint).

- [ ] **Step 5: Final commit**

```bash
git add CONTEXT.md docs/ tasks/todo.md docs/superpowers/plans/2026-07-08-mcp-client-logical-query.md
git commit -m "docs: document MCP client-generated LogicalQuery mode (8-tool contract)"
```

**Do not push. Do not merge dev → main.** Deployment follows the standard flow later (`make verify-main`, helm bump) — out of scope here.

---

## Deviations from spec (intentional, decided during planning)

1. **Spec §2 is already implemented** — the validator has produced structured `ValidationError`s with `AllowedAlternatives` since before this plan (`pkg/query/types.go:149`, `internal/query/validation_helpers.go:59` `suggestAlternatives`). No validator changes needed; the gap was only HTTP exposure (Task 2).
2. **`validate_logical_query` input** is `{logical_query}` with `datasource_id`/`model_id` embedded in the document — matching `run_logical_query`'s existing contract — instead of the spec's separate top-level ids.
3. **Suggestions field name** is the pre-existing `allowed_alternatives`, not `suggestions`.
4. **InputSchema embedding** (spec §4) is conditional on the MCP go-sdk honoring a caller-provided `Tool.InputSchema` (Task 5 Step 1); the guaranteed path is the schema inside `get_model_context.logical_query_schema`.

## Deferred

(record here anything skipped due to the ai_agent_chat.go conflict, with date)
