# Add Dimension (derived / calculated) — Design

**Date:** 2026-07-13
**Status:** Approved (design), pending implementation plan
**Scope:** Frontend-only. No backend changes.

## Context

The semantic modeling UI (`frontend/src/components/modeling/`) lets a user add a
**metric** from scratch via `AddMetricModal` (Simple Aggregation | Custom
Expression, with a reusable `ExpressionBuilder`). Dimensions, however, can only
be created implicitly — by "Sync dimensions" (auto from columns) or per-column
toggle — with no way to **create a derived/calculated dimension** (e.g. `CONCAT`,
`CASE`, `COALESCE`) from the UI. Editing an existing dimension already supports a
calculated expression (`EditDimensionModal` has a column/calculated toggle and
reuses `ExpressionBuilder`), but there is no create path.

The backend already fully supports calculated dimensions end-to-end:
- `pkg/semantic/types.go` `Dimension` has `CalculatedExpression string` +
  `CalculatedExpr ExprNode`.
- `POST /api/semantic/models/{id}/dimensions` (`CreateDimension`,
  `internal/http/handlers/semantic.go`) accepts `calculated_expression` +
  `calculated_expr` and persists both (`internal/semantic/repository.go`).
- `POST /api/semantic/models/{id}/compile-expression` compiles an expression to
  dialect SQL, metric/dimension-agnostic; used by `ExpressionBuilder`.
- The expression compiler (`internal/query/expr_compiler.go`) + whitelist
  (`pkg/semantic/expr.go` `AllowedFunctions`) support `CONCAT, COALESCE, CASE,
  UPPER, LOWER, ROUND, LENGTH, TRIM, ABS, CEIL, FLOOR, CAST, EXTRACT,
  DATE_TRUNC, NULLIF, IFNULL, ISNULL, SUBSTRING, REPLACE, LEFT, RIGHT`, with a
  read-only/DML guard and strict publish-time validation
  (`validateCalculatedDimension`). Aggregate functions (`sum/count/...`) are
  intentionally NOT allowed in expressions — correct for non-aggregated
  dimensions.

**The only gap is a "create derived dimension" UI.** This design adds it by
mirroring the existing metric-add and dimension-edit patterns.

## Goal

Let a user create a new dimension from the modeling palette — either backed by a
single **column** or by a **derived expression** (concat/case/coalesce/…) — with
a live compiled-SQL preview, and have it appear on the canvas.

## Non-goals

- No backend changes (endpoints, schema, compiler all already support this).
- No aggregate functions in dimension expressions (backend rejects them by
  design; the modal surfaces the compile error like the metric path does).
- Not changing the implicit column-toggle / Sync-dimensions flows.
- The unrelated `sum([...])` → 400 behavior in the expression editor is out of
  scope (correct for dimensions; aggregation is a metric concept).

## Components

### 1. `AddDimensionModal.tsx` (new)
`frontend/src/components/modeling/AddDimensionModal.tsx`. Mirrors
`AddMetricModal` layout; effectively the create-mode counterpart of
`EditDimensionModal` with an editable name.

- **Header:** `Name` (editable, required) + `Display name` (optional label).
- **Mode toggle (tablist):** `Column` | `Derived expression` — same UI as the
  metric modal's Simple/Custom tabs.
- **Column mode:** Select Schema → Table → Column + a `Type` select. Payload:
  `{ name, label?, column_ref, type }`.
- **Derived mode:** `<ExpressionBuilder>` (Text/Visual, live compiled-SQL
  preview, `[table.column]` autocomplete) + a `Type` select. Payload:
  `{ name, label?, type, calculated_expression, calculated_expr }`.
- **Submit:** `postData('/api/semantic/models/${model.id}/dimensions', body)`.
- **Create enabled when:** name non-empty AND (column mode: a column selected;
  derived mode: expression present and last compile succeeded). Reuses
  `ExpressionBuilder`'s existing compile state to gate.

### 2. `useAddDimensionModalState.ts` (new)
`frontend/src/components/modeling/useAddDimensionModalState.ts`, paralleling
`useAddMetricModalState`: holds `mode` (`column | derived`), the field values,
the built AST/text, `buildSubmitBody()`, and compile-success gating. Keeps the
modal component thin.

### 3. Palette button
`ModelingPalette.tsx` (~lines 683-701): add a `+ Add dimension` button to the
existing grid alongside `Sync dimensions` and `+ Add metric`, wired to a new
`onOpenAddDimension` prop. Grid becomes 3 items (or a 2-col grid that wraps).

### 4. Page-state plumbing
`useModelingPageState.ts` + `ModelingModals.tsx`: add
`addDimensionOpen`, `onOpenAddDimension`, `onDimensionCreated` (triggers the same
model refresh used after add-metric), mirroring the existing add-metric plumbing
(`addMetricOpen` / `onOpenAddMetric` / `onMetricCreated`). Register
`AddDimensionModal` in `ModelingModals.tsx`.

### 5. Function parity
`ExpressionBuilder.tsx` `ALLOWED_FUNCTIONS` (lines ~21-40): add `CAST` and
`EXTRACT` so the Visual builder's function list matches the backend
`AllowedFunctions` whitelist. (Useful for date/number derivations.)

### 6. i18n
Add EN + TR keys mirroring the metric keys: modal title ("Add dimension"), tab
labels ("Column" / "Derived expression"), type label, and any hint text. Follow
the existing `modeling.*` key structure.

## Data flow

Palette `+ Add dimension` → `onOpenAddDimension` sets `addDimensionOpen` →
`AddDimensionModal` renders → user picks Column or Derived → (Derived) types an
expression → debounced `POST /compile-expression` shows generated SQL / error →
`Create` → `POST /dimensions` with the built body → `onDimensionCreated` refreshes
the model → dimension appears on the canvas / dimension list.

## Error handling

- Expression compile errors: shown inline by `ExpressionBuilder` (existing
  "Failed to compile expression" banner); `Create` stays disabled until compile
  succeeds.
- Create request failure: surfaced via the modal's existing error display (same
  pattern as `AddMetricModal`/`EditDimensionModal`).
- Name/column required: `Create` disabled until satisfied.

## Reuse (no changes needed)

- `ExpressionBuilder` — reused as-is (already dimension-capable).
- `compile-expression` endpoint — reused as-is.
- `CreateDimension` endpoint + repo — reused as-is (already accepts calculated
  fields; the frontend just starts sending them).
- `EditDimensionModal` — the structural template for the new modal.

## Testing

- Vitest component test for `AddDimensionModal`: renders both tabs; switching
  modes; builds the correct payload for Column vs Derived; `Create` disabled
  until valid. Follow the existing modeling modal test patterns.
- Manual: create a `CONCAT([t.a], ' ', [t.b])` dimension, confirm live SQL
  preview, create, and that it appears and is usable in a query.

## Files touched

- New: `frontend/src/components/modeling/AddDimensionModal.tsx`,
  `frontend/src/components/modeling/useAddDimensionModalState.ts`,
  `AddDimensionModal` test.
- Edit: `ModelingPalette.tsx`, `ModelingModals.tsx`, `useModelingPageState.ts`,
  `ExpressionBuilder.tsx` (function parity), EN + TR locale files.
- Backend: none.
