# Semantic Canvas UX — Design (Spec 1)

Date: 2026-07-04
Status: Approved (design), pending implementation plan
Scope: `frontend/src/components/modeling/*` — no backend changes.

## Goal

Bring the semantic-modeling canvas closer to the reference product's look and
feel, in three parts:

1. **Scroll-zoom that works** — smooth, cursor-anchored mouse-wheel / trackpad
   zoom.
2. **Redesigned table cards** — a clean 3-section card (Columns / Calculated
   Fields / Relationships) with per-column type icons.
3. **Click-to-detail modal** — clicking a card opens a detail modal (Name /
   Alias / Description, columns table, relationships table, 100-row data
   preview).

The AI "recommend semantics" wizard is deliberately **out of scope** here; it is
a separate spec (Spec 2).

## Decisions (locked)

- **Keep the bespoke canvas** (no React Flow migration). Reuse the tested
  `canvasMath` / pan / drag / SVG-edge code; change only what each feature needs.
- **Full 3-section card** faithful to the reference (not a columns-only refresh).
- **Header single click = open detail modal**; dragging is disambiguated by a
  movement threshold (a mousedown→mouseup with no meaningful move counts as a
  click; a drag past the threshold moves the card and suppresses the click).
- **`＋ add` buttons** in the Calculated Fields / Relationships card sections
  open the *existing* add-dimension/add-metric/add-join modals. No new add
  behavior is introduced in this spec.
- **No new backend endpoint.** The detail modal's data preview reuses
  `POST /api/datasources/{ds}/tables/{schema}/{table}/rows` (already supports
  `pageSize: 100`). Column/join/description data comes from the already-loaded
  semantic model detail + metadata.

## Current state (baseline)

- Canvas is fully custom (no react-flow). Scene is one `<div>` transformed with
  `translate3d(tx,ty,0) scale(scale)`; edges are hand-built SVG paths.
  - `ModelingCanvas.tsx` (render), `useModelingCanvas.ts` (viewport/pan/drag/
    wheel), `canvasMath.ts` (zoom ladder, layout, join-path geometry; unit
    tested), `constants.ts` (`CARD_WIDTH=280`, `ROW_HEIGHT=25.8`, `COL_LIMIT=10`,
    `MIN_SCALE=0.3`, `MAX_SCALE=2.5`, discrete `ZOOM_STEPS`).
- Wheel zoom **already exists** (`useModelingCanvas.ts:176-190`) but steps
  through a **discrete ladder** (`zoomStep`), one rung per wheel event — feels
  unresponsive / "broken" on a trackpad, and the gesture can be hijacked by the
  browser (page scroll / overscroll).
- Table cards are rendered inline in `ModelingCanvas.tsx`: header (schema +
  table name, doubles as the drag handle) + a `<ul>` of columns with **text**
  data-type and text PK/FK badges. **No type icons. No Calculated Fields or
  Relationships sections. No click-to-detail.**
- Styling is Tailwind utility strings centralized in
  `frontend/src/lib/modelingClasses.ts` (composed via `cn()`); no CSS/BEM
  modules.
- Data preview already exists as a *separate* page (`TableBrowser`) hitting the
  rows endpoint above; it is not reachable from the canvas.
- Semantic model shape: `frontend/src/types/semantic.ts` — `ColumnRow`
  (`data_type`, `is_primary_key`, `is_foreign_key`, `description`, `referenced_*`),
  `SemanticModelDetail` (`dimensions[]`, `metrics[]`, `joins[]`),
  `SemanticDimension`/`SemanticMetric` (`calculated_expr`/`expr`, `column_ref`),
  `SemanticJoin` (`from_*`/`to_*`, `join_type`, `relationship`).

## Design

### 0. Scroll-zoom fix

- Replace the discrete `zoomStep` used by the wheel handler with **continuous
  multiplicative zoom**:
  `newScale = clamp(scale * exp(-deltaY * K), MIN_SCALE, MAX_SCALE)` where `K` is
  a small sensitivity constant tuned so a normal wheel notch and a trackpad
  two-finger scroll both feel smooth. Keep anchoring at the cursor via the
  existing `zoomViewportAtPoint`.
- Keep the existing behavior of ignoring horizontal-dominant scroll unless
  `ctrl`/`meta` is held (so pinch-zoom still routes here).
- Prevent the browser from stealing the gesture: add `touch-action: none` and
  `overscroll-behavior: contain` to the canvas wrap class in
  `modelingClasses.ts`. Verify `preventDefault()` on the non-passive listener
  reliably fires.
- The `+ / − / fit / 1:1` buttons and the `%` readout stay. The `+ / −` buttons
  may keep the discrete ladder or adopt a fixed step; either is fine.
- Update `canvasMath` zoom unit tests: the continuous zoom helper is pure and
  testable (given `scale`, `deltaY` → `newScale`), replacing the ladder-step
  assertions for the wheel path.

### 1. Table card redesign (3 sections)

Extract the inline card render into a new `ModelingTableCard.tsx` component
(props: the table, its columns, the calculated fields touching it, the joins
touching it, plus drag/click handlers). Layout mirrors the reference:

- **Header**: table type icon + `table_name` (bold) + `schema_name` (muted) +
  kebab (⋮) menu. The kebab menu items call the existing palette actions
  (rename / display-name, make-base, remove/hide). The header is both the drag
  handle and the click target for the detail modal (see §2).
- **Columns**: each row = a **type icon** + column name + PK/FK badge. Icon
  mapping lives in a new pure `columnTypeIcon.ts`:
  - numeric (`int`, `bigint`, `numeric`, `decimal`, `float`, `double`, `real`,
    `serial`) → `123`
  - text (`text`, `varchar`, `char`, `uuid`, `citext`, `name`) → `A-Z`
  - boolean → `✓`
  - date → calendar
  - timestamp / time / timestamptz → clock
  - json / jsonb → `{}`
  - array → `[]`
  - fallback → generic/text icon
  Keep `COL_LIMIT` + "+N more".
- **Calculated Fields**: section header with a count of calculated dimensions/
  metrics that reference this table (dimensions with `calculated_expr`/
  `calculated_expr` node, metrics) + a `＋` button that opens the existing
  add-metric / edit-dimension modal.
- **Relationships**: list of the opposite-table names for joins touching this
  table + a `＋` button that opens the existing join editor.
- **Variable card height**: `canvasMath.buildCardLayouts` / `cardHeight` must
  account for the three sections' heights. **Columns stays the first section**
  so the existing join-path geometry (anchored to a column row's vertical
  center) keeps working; only the total card height and any below-columns anchor
  math changes.

New/updated class helpers go in `modelingClasses.ts`.

### 2. Click → detail modal

New `TableDetailModal.tsx`, opened from a card header click. Content mirrors the
reference detail panel:

- **Header**: Name / Alias (= `label` / display name) / Description, plus an
  **Edit** button that opens the existing rename/edit flow.
- **Columns** table: Name / Alias / Type (icon) / Description — paginated using
  the existing `DataTable` + pagination pattern.
- **Relationships** table: Name / From / To / Type (`many_to_one`, etc.) /
  Description — filtered from `SemanticModelDetail.joins` to those touching this
  table.
- **Data preview (100 rows)**: a "Preview data" button lazily fetches via
  `POST /api/datasources/{ds}/tables/{schema}/{table}/rows` with `pageSize: 100`
  and renders the rows (reuse the TableBrowser cell/rendering helpers where
  practical).

Click vs drag disambiguation: track pointer movement between mousedown and
mouseup on the header; below a small pixel threshold → open modal; at/above →
treat as a card drag (existing behavior) and suppress the click.

## Components / files touched

Frontend only:

- `frontend/src/components/modeling/useModelingCanvas.ts` — continuous wheel zoom;
  click-vs-drag threshold plumbing.
- `frontend/src/components/modeling/canvasMath.ts` (+ `canvasMath.test.ts`) —
  continuous-zoom helper; 3-section card height.
- `frontend/src/components/modeling/ModelingCanvas.tsx` — render `ModelingTableCard`
  instead of inline `<article>`; wire click→modal + modal state.
- `frontend/src/components/modeling/ModelingTableCard.tsx` — **new**.
- `frontend/src/components/modeling/TableDetailModal.tsx` — **new**.
- `frontend/src/components/modeling/columnTypeIcon.ts` — **new** (pure map; unit
  tested).
- `frontend/src/lib/modelingClasses.ts` — new/updated card + section + zoom-wrap
  classes.
- `frontend/src/components/modeling/useModelingPageState.ts` /
  `modelingModelActions.ts` — only if the modal/kebab need to reach existing
  actions (rename, add-dimension/metric, add-join, data preview).

No backend changes.

## Testing / success criteria

- **Zoom**: mouse-wheel and trackpad two-finger scroll over the canvas zoom
  smoothly toward the cursor; the page never scrolls; `+/−/fit/1:1` still work.
  `canvasMath` continuous-zoom helper unit tested (clamping at MIN/MAX, monotonic
  with `deltaY`, cursor-anchor invariant).
- **Cards**: each card shows the three sections; column type icons match the map;
  PK/FK badges preserved; "+N more" preserved; join edges still land on the
  correct column rows after the height change (visual + `computeJoinPath` tests
  unaffected or updated).
- **Modal**: header single click opens the modal; a drag does not; modal shows
  columns (paginated), relationships (filtered to this table), and a 100-row
  preview on demand; Edit opens the existing edit flow; keyboard/aria accessible;
  all text via `useT()`.
- Gates: `make check-frontend` (eslint + tailwind + format + knip + vitest +
  tsc build) passes; `make lint-frontend` clean.

## Out of scope (Spec 2)

- AI "recommend semantics" wizard (pick models → describe purpose → generate →
  review → save), including any new backend endpoint that threads a user-purpose
  prompt into `semanticgen` / `internal/ai/describe`.
