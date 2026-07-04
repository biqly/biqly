# Semantic Canvas UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the semantic-modeling canvas closer to the reference product: smooth cursor-anchored scroll-zoom, a 3-section table card (Columns / Calculated Fields / Relationships) with per-column type icons, and a click-to-detail modal.

**Architecture:** Keep the existing bespoke canvas (no React Flow). Zoom becomes continuous multiplicative (pure helper in `canvasMath`). Card rendering moves out of `ModelingCanvas` into a focused `ModelingTableCard` component; per-table section data + total card height are computed in pure `canvasMath` helpers so the existing layout/bounds/edge math keeps a single source of truth. A new `TableDetailModal` (built on the existing `Modal` + `DataTable`) opens on header click and lazily previews rows through the existing metadata rows endpoint. No backend changes.

**Tech Stack:** React 19 + TypeScript + Vite 6, Tailwind v4 utility strings via `cn()` (`frontend/src/lib/modelingClasses.ts`), Vitest. Data preview reuses `POST /api/datasources/{ds}/tables/{schema}/{table}/rows`.

## Global Constraints

- Frontend only — **no backend changes**, no new dependencies.
- Tailwind utility strings centralized in `frontend/src/lib/modelingClasses.ts`; compose with `cn()`. No inline styles except the existing dynamic canvas transform / card `left/top/width/height`.
- All user-facing text via the `useT()` / `t()` i18n path with `TranslationKey`s; add new keys to the i18n catalog.
- Accessibility: interactive elements semantic + keyboard reachable + `aria-*`; unique ids.
- Gate before every commit: `make check-frontend` (eslint + tailwind + format:check + knip + vitest + `tsc` build) must pass; `make lint-frontend` clean. Run `npm --prefix frontend run test` for focused vitest.
- Column geometry invariant: the **Columns section stays first** in the card (below the header) and its row math (`HEADER_HEIGHT + CARD_PAD_Y + idx*ROW_HEIGHT`) is unchanged, so `computeJoinPath` edge anchoring keeps working. New sections only add to total card height.

---

### Task 1: Continuous cursor-anchored scroll-zoom

Replace discrete ladder stepping on the wheel path with continuous multiplicative zoom, and stop the browser from hijacking trackpad gestures.

**Files:**
- Modify: `frontend/src/components/modeling/canvasMath.ts` (add `continuousZoomScale`)
- Modify: `frontend/src/components/modeling/useModelingCanvas.ts:176-189` (wheel handler)
- Modify: `frontend/src/index.css:418` (`.modeling-canvas-wrap` — add `touch-action: none`)
- Test: `frontend/src/components/modeling/canvasMath.test.ts`

**Interfaces:**
- Produces: `continuousZoomScale(scale: number, deltaY: number): number` — clamped to `[MIN_SCALE, MAX_SCALE]`, `deltaY < 0` (scroll up) zooms in, `deltaY > 0` zooms out.
- Consumes: existing `clampScale`, `zoomViewportAtPoint` from `canvasMath.ts`; `MIN_SCALE`/`MAX_SCALE` from `constants.ts`.

- [x] **Step 1: Write the failing test**

Add to `frontend/src/components/modeling/canvasMath.test.ts`:

```ts
import { continuousZoomScale } from './canvasMath'
import { MAX_SCALE, MIN_SCALE } from './constants'

describe('continuousZoomScale', () => {
  it('zooms in when deltaY is negative and out when positive', () => {
    expect(continuousZoomScale(1, -100)).toBeGreaterThan(1)
    expect(continuousZoomScale(1, 100)).toBeLessThan(1)
  })

  it('clamps to the configured min and max', () => {
    expect(continuousZoomScale(MIN_SCALE, 5000)).toBe(MIN_SCALE)
    expect(continuousZoomScale(MAX_SCALE, -5000)).toBe(MAX_SCALE)
  })

  it('is a no-op for zero delta', () => {
    expect(continuousZoomScale(1, 0)).toBeCloseTo(1, 10)
  })

  it('scales more for a larger delta magnitude', () => {
    const small = continuousZoomScale(1, -50)
    const large = continuousZoomScale(1, -200)
    expect(large).toBeGreaterThan(small)
  })
})
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm --prefix frontend run test -- canvasMath`
Expected: FAIL — `continuousZoomScale is not a function` / not exported.

- [x] **Step 3: Implement the pure helper**

In `frontend/src/components/modeling/canvasMath.ts`, add after `zoomStep` (around line 41):

```ts
// Continuous multiplicative zoom for the wheel/trackpad path. A discrete
// ladder (zoomStep) slams to min/max under trackpad momentum (many events per
// gesture); scaling by deltaY magnitude stays controllable on both mouse and
// trackpad. ZOOM_WHEEL_SENSITIVITY is tuned so one mouse notch (~100px) is a
// gentle step.
const ZOOM_WHEEL_SENSITIVITY = 0.0015

export function continuousZoomScale(scale: number, deltaY: number): number {
  return clampScale(scale * Math.exp(-deltaY * ZOOM_WHEEL_SENSITIVITY))
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `npm --prefix frontend run test -- canvasMath`
Expected: PASS (existing canvasMath tests + the 4 new ones).

- [x] **Step 5: Wire the wheel handler to the continuous helper**

In `frontend/src/components/modeling/useModelingCanvas.ts`:

Add `continuousZoomScale` to the import from `./canvasMath` (the block at lines 4-16), keeping `zoomStep` (still used by `zoomBy`).

Replace the `setViewport` body inside `onWheel` (lines 184-188) with:

```ts
      setViewport((vp) => {
        const newScale = continuousZoomScale(vp.scale, ev.deltaY)
        return zoomViewportAtPoint(vp, cx, cy, newScale)
      })
```

(Delete the now-unused `direction` line. Leave the horizontal-scroll guard at lines 177-179 unchanged so pinch-zoom still routes here.)

- [x] **Step 6: Stop the browser from stealing the gesture**

In `frontend/src/index.css`, inside the `.modeling-canvas-wrap` rule (line 418), add `touch-action: none;` next to the existing `overscroll-behavior: contain;` (line 425):

```css
  overscroll-behavior: contain;
  touch-action: none;
```

- [ ] **Step 7: Verify gate + manual check**

Run: `make check-frontend`
Expected: PASS.
Manual: over the canvas, mouse-wheel and trackpad two-finger scroll zoom smoothly toward the cursor; the page never scrolls; `+ / − / ⤢ / 1:1` still work; `%` readout updates.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/modeling/canvasMath.ts frontend/src/components/modeling/canvasMath.test.ts frontend/src/components/modeling/useModelingCanvas.ts frontend/src/index.css
git commit -m "feat(modeling): continuous cursor-anchored scroll-zoom"
```

---

### Task 2: Column type icon map

A pure map from a Postgres data type to a compact glyph + kind, reusing the existing type-bucketing.

**Files:**
- Create: `frontend/src/components/modeling/columnTypeIcon.ts`
- Test: `frontend/src/components/modeling/columnTypeIcon.test.ts`

**Interfaces:**
- Consumes: `normalizeJoinDataType(dataType: string): string` from `./utils` (buckets to `integer|text|boolean|timestamp|date|decimal|json|…`).
- Produces:
  - `type ColumnTypeKind = 'number' | 'text' | 'boolean' | 'date' | 'timestamp' | 'json' | 'array' | 'other'`
  - `interface ColumnTypeIcon { kind: ColumnTypeKind; glyph: string }`
  - `columnTypeIcon(dataType: string): ColumnTypeIcon`

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/modeling/columnTypeIcon.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import { columnTypeIcon } from './columnTypeIcon'

describe('columnTypeIcon', () => {
  it('maps numeric types to the 123 glyph', () => {
    expect(columnTypeIcon('bigint')).toEqual({ kind: 'number', glyph: '123' })
    expect(columnTypeIcon('numeric(10,2)').kind).toBe('number')
  })

  it('maps text types to the A-Z glyph', () => {
    expect(columnTypeIcon('text')).toEqual({ kind: 'text', glyph: 'A-Z' })
    expect(columnTypeIcon('character varying').kind).toBe('text')
  })

  it('maps boolean, date and timestamp', () => {
    expect(columnTypeIcon('boolean').kind).toBe('boolean')
    expect(columnTypeIcon('date').kind).toBe('date')
    expect(columnTypeIcon('timestamp with time zone').kind).toBe('timestamp')
  })

  it('detects json and arrays and falls back to other', () => {
    expect(columnTypeIcon('jsonb').kind).toBe('json')
    expect(columnTypeIcon('text[]').kind).toBe('array')
    expect(columnTypeIcon('geometry').kind).toBe('other')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm --prefix frontend run test -- columnTypeIcon`
Expected: FAIL — cannot resolve `./columnTypeIcon`.

- [ ] **Step 3: Implement the map**

Create `frontend/src/components/modeling/columnTypeIcon.ts`:

```ts
import { normalizeJoinDataType } from './utils'

export type ColumnTypeKind =
  | 'number'
  | 'text'
  | 'boolean'
  | 'date'
  | 'timestamp'
  | 'json'
  | 'array'
  | 'other'

export interface ColumnTypeIcon {
  kind: ColumnTypeKind
  glyph: string
}

const GLYPHS: Record<ColumnTypeKind, string> = {
  number: '123',
  text: 'A-Z',
  boolean: '✓',
  date: '▦',
  timestamp: '◷',
  json: '{}',
  array: '[]',
  other: '·',
}

// normalizeJoinDataType strips precision/params and buckets to integer|text|
// boolean|timestamp|date|decimal|json (else the raw type). We map those buckets
// to display kinds; arrays (trailing []) and unknowns fall through.
export function columnTypeIcon(dataType: string): ColumnTypeIcon {
  if (/\[\]\s*$/.test(dataType.trim())) {
    return { kind: 'array', glyph: GLYPHS.array }
  }
  const bucket = normalizeJoinDataType(dataType)
  const kind: ColumnTypeKind =
    bucket === 'integer' || bucket === 'decimal'
      ? 'number'
      : bucket === 'text'
        ? 'text'
        : bucket === 'boolean'
          ? 'boolean'
          : bucket === 'date'
            ? 'date'
            : bucket === 'timestamp'
              ? 'timestamp'
              : bucket === 'json'
                ? 'json'
                : 'other'
  return { kind, glyph: GLYPHS[kind] }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm --prefix frontend run test -- columnTypeIcon`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/modeling/columnTypeIcon.ts frontend/src/components/modeling/columnTypeIcon.test.ts
git commit -m "feat(modeling): column type icon map"
```

---

### Task 3: Per-table card sections + variable card height

Compute, for each table card, the count of calculated fields (dimensions/metrics referencing it) and the list of related tables (from joins touching it), and fold the two new sections' heights into `CardLayout.height` so layout/bounds stay single-sourced.

**Files:**
- Modify: `frontend/src/components/modeling/constants.ts` (section-height constants)
- Modify: `frontend/src/components/modeling/types.ts:55-60` (`CardLayout` + new `CardSection`)
- Modify: `frontend/src/components/modeling/canvasMath.ts` (`cardHeight`, `buildCardSections`, `buildCardLayouts`)
- Modify: `frontend/src/components/modeling/useModelingCanvas.ts:37-52` (pass model to layout build)
- Test: `frontend/src/components/modeling/canvasMath.test.ts`

**Interfaces:**
- Consumes: `columnRefMatchesTable(ref, schema, table, baseSchema)` and `tableKey`, `splitTableKey` from `./utils`; `SemanticModelDetail`, `TableRow` from `../../types/semantic`.
- Produces:
  - `interface CardSection { calcFieldCount: number; relatedTables: string[] }` (in `types.ts`)
  - `CardLayout` gains `calcFieldCount: number` and `relatedTables: string[]`.
  - `buildCardSections(tableCards: TableRow[], model: SemanticModelDetail | null): Map<string, CardSection>`
  - `cardHeight(colRowCount: number, section?: CardSection): number`
  - `buildCardLayouts(tableCards, columns, joinColumns, colLimit, sections: Map<string, CardSection>): Map<string, CardLayout>`

- [ ] **Step 1: Write the failing tests**

Add to `frontend/src/components/modeling/canvasMath.test.ts`:

```ts
import { buildCardSections, cardHeight } from './canvasMath'
import { CALC_SECTION_HEIGHT, REL_SECTION_LABEL_HEIGHT, ROW_HEIGHT } from './constants'
import type { SemanticModelDetail, TableRow } from '../../types/semantic'

const mkTable = (schema: string, table: string): TableRow =>
  ({ schema_name: schema, table_name: table, id: `${schema}.${table}` }) as TableRow

describe('buildCardSections', () => {
  const model = {
    base_schema: 'public',
    base_table: 'orders',
    joins: [
      {
        id: 'j1',
        name: 'orders_to_users',
        from_table: 'orders',
        from_column: 'user_id',
        to_table: 'users',
        to_column: 'id',
        join_type: 'LEFT',
        relationship: 'many_to_one',
        is_active: true,
      },
    ],
    dimensions: [
      { id: 'd1', name: 'full_name', column_ref: 'users.name', type: 'string' },
      { id: 'd2', name: 'rev', column_ref: '', type: 'number', calculated_expression: 'x', column_ref_calc: '' },
    ],
    metrics: [{ id: 'm1', name: 'total', expression: 'sum(orders.amount)', aggregation: 'sum' }],
  } as unknown as SemanticModelDetail

  it('lists related tables for a table touched by a join', () => {
    const sections = buildCardSections(
      [mkTable('public', 'orders'), mkTable('public', 'users')],
      model,
    )
    expect(sections.get('public.orders')?.relatedTables).toEqual(['users'])
    expect(sections.get('public.users')?.relatedTables).toEqual(['orders'])
  })

  it('counts calculated fields whose ref matches the table', () => {
    const sections = buildCardSections([mkTable('public', 'orders')], model)
    // metric total references public.orders.amount
    expect(sections.get('public.orders')?.calcFieldCount).toBeGreaterThanOrEqual(1)
  })

  it('returns zeros for a null model', () => {
    const sections = buildCardSections([mkTable('public', 'orders')], null)
    expect(sections.get('public.orders')).toEqual({ calcFieldCount: 0, relatedTables: [] })
  })
})

describe('cardHeight with sections', () => {
  it('adds the calc + relationship section heights', () => {
    const base = cardHeight(3)
    const withSections = cardHeight(3, { calcFieldCount: 2, relatedTables: ['a', 'b'] })
    expect(withSections).toBe(
      base + CALC_SECTION_HEIGHT + REL_SECTION_LABEL_HEIGHT + 2 * ROW_HEIGHT,
    )
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm --prefix frontend run test -- canvasMath`
Expected: FAIL — `buildCardSections` not exported / constants missing.

- [ ] **Step 3: Add section-height constants**

In `frontend/src/components/modeling/constants.ts`, append:

```ts
// 3-section card: fixed heights for the Calculated Fields row and the
// Relationships section label; each related-table row reuses ROW_HEIGHT.
export const CALC_SECTION_HEIGHT = 26
export const REL_SECTION_LABEL_HEIGHT = 22
```

- [ ] **Step 4: Extend `CardLayout` + add `CardSection`**

In `frontend/src/components/modeling/types.ts`, replace the `CardLayout` interface (lines 55-60) with:

```ts
export interface CardSection {
  calcFieldCount: number
  relatedTables: string[]
}

export interface CardLayout {
  columnsShown: ColumnRow[]
  columnIndex: Map<string, number>
  height: number
  hiddenCount: number
  calcFieldCount: number
  relatedTables: string[]
}
```

- [ ] **Step 5: Implement `buildCardSections`, extend `cardHeight` + `buildCardLayouts`**

In `frontend/src/components/modeling/canvasMath.ts`:

Update imports at the top:

```ts
import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  CALC_SECTION_HEIGHT,
  CARD_PAD_Y,
  CARD_WIDTH,
  GRID_X,
  GRID_Y,
  HEADER_HEIGHT,
  KEYBOARD_MOVE_STEP,
  KEYBOARD_MOVE_STEP_SHIFT,
  LAYOUT_COLS,
  MAX_SCALE,
  MIN_SCALE,
  ORIGIN_X,
  ORIGIN_Y,
  REL_SECTION_LABEL_HEIGHT,
  ROW_HEIGHT,
  ZOOM_STEPS,
} from './constants'
import type { CardLayout, CardSection, JoinPath, Pt, Viewport } from './types'
import { columnOptions, columnRefMatchesTable, compareColumns, splitTableKey, tableKey } from './utils'
```

Replace `cardHeight` (line 54) with:

```ts
export const cardHeight = (count: number, section?: CardSection) => {
  const base = HEADER_HEIGHT + count * ROW_HEIGHT + CARD_PAD_Y * 2
  if (!section) {
    return base
  }
  const relRows = section.relatedTables.length
  return base + CALC_SECTION_HEIGHT + REL_SECTION_LABEL_HEIGHT + relRows * ROW_HEIGHT
}
```

Add `buildCardSections` (place near `buildCardLayouts`):

```ts
// For each table card, count calculated fields (dimensions with a calculated
// expression + metrics) whose expression/ref points at the table, and collect
// the opposite-table names of active joins touching it. Pure so it stays unit
// tested and layout height has a single source of truth.
export function buildCardSections(
  tableCards: TableRow[],
  model: SemanticModelDetail | null,
): Map<string, CardSection> {
  const out = new Map<string, CardSection>()
  const baseSchema = model?.base_schema ?? ''
  const activeJoins = (model?.joins ?? []).filter((j) => j.is_active !== false)
  const calcDims = (model?.dimensions ?? []).filter(
    (d) => d.is_active !== false && (d.calculated_expression || d.calculated_expr),
  )
  const metrics = (model?.metrics ?? []).filter((m) => m.is_active !== false)

  for (const tbl of tableCards) {
    const key = tableKey(tbl.schema_name, tbl.table_name)
    const related: string[] = []
    for (const join of activeJoins) {
      const fromKey = tableKey(join.from_schema ?? baseSchema, join.from_table)
      const toKey = tableKey(join.to_schema ?? baseSchema, join.to_table)
      if (fromKey === key) {
        related.push(join.to_table)
      } else if (toKey === key) {
        related.push(join.from_table)
      }
    }
    let calcFieldCount = 0
    for (const d of calcDims) {
      if (columnRefMatchesTable(d.column_ref, tbl.schema_name, tbl.table_name, baseSchema)) {
        calcFieldCount++
      }
    }
    for (const m of metrics) {
      if (columnRefMatchesTable(m.expression, tbl.schema_name, tbl.table_name, baseSchema)) {
        calcFieldCount++
      }
    }
    out.set(key, { calcFieldCount, relatedTables: related })
  }
  return out
}
```

Replace `buildCardLayouts` (lines 143-167) with the section-aware version:

```ts
export function buildCardLayouts(
  tableCards: TableRow[],
  columns: ColumnRow[],
  joinColumns: Map<string, Set<string>>,
  colLimit: number,
  sections: Map<string, CardSection>,
): Map<string, CardLayout> {
  const out = new Map<string, CardLayout>()
  for (const tbl of tableCards) {
    const key = tableKey(tbl.schema_name, tbl.table_name)
    const linked = joinColumns.get(key) ?? new Set<string>()
    const allCols = columnOptions(columns, key)
    const cols = [...allCols].sort((a, b) => compareColumns(a, b, linked)).slice(0, colLimit)
    const idx = new Map<string, number>()
    cols.forEach((c, i) => idx.set(c.column_name, i))
    const hidden = Math.max(0, allCols.length - cols.length)
    const rowCount = cols.length + (hidden > 0 ? 1 : 0)
    const section = sections.get(key) ?? { calcFieldCount: 0, relatedTables: [] }
    out.set(key, {
      columnsShown: cols,
      columnIndex: idx,
      height: cardHeight(rowCount, section),
      hiddenCount: hidden,
      calcFieldCount: section.calcFieldCount,
      relatedTables: section.relatedTables,
    })
  }
  return out
}
```

> Note: `columnRefMatchesTable` matching a metric's full `expression` (e.g. `sum(orders.amount)`) is a substring-free prefix check today; it matches when the expression *starts with* `table.` or `schema.table.`. For metrics whose expression wraps the ref in a function, count may under-report — acceptable for a section badge (it is a hint, not a guarantee). Do not add new matching logic in this task.

- [ ] **Step 6: Update the `buildCardLayouts` caller**

In `frontend/src/components/modeling/useModelingCanvas.ts`, update the import from `./canvasMath` to add `buildCardSections`, then change the `cardLayouts` memo (lines 37-52) to build sections and pass them:

```ts
  const cardLayouts = useMemo(() => {
    const joinColumns = new Map<string, Set<string>>()
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      const fromKey = tableKey(join.from_schema ?? model?.base_schema ?? '', join.from_table)
      const toKey = tableKey(join.to_schema ?? model?.base_schema ?? '', join.to_table)
      if (!joinColumns.has(fromKey)) {
        joinColumns.set(fromKey, new Set())
      }
      if (!joinColumns.has(toKey)) {
        joinColumns.set(toKey, new Set())
      }
      joinColumns.get(fromKey)!.add(join.from_column)
      joinColumns.get(toKey)!.add(join.to_column)
    }
    const sections = buildCardSections(tableCards, model)
    return buildCardLayouts(tableCards, columns, joinColumns, COL_LIMIT, sections)
  }, [tableCards, columns, model])
```

- [ ] **Step 7: Run tests + gate**

Run: `npm --prefix frontend run test -- canvasMath`
Expected: PASS (new section/height tests + existing layout/bounds/join-path tests still green — heights grew but Columns geometry unchanged).
Run: `make check-frontend`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/modeling/constants.ts frontend/src/components/modeling/types.ts frontend/src/components/modeling/canvasMath.ts frontend/src/components/modeling/canvasMath.test.ts frontend/src/components/modeling/useModelingCanvas.ts
git commit -m "feat(modeling): per-table card sections + variable card height"
```

---

### Task 4: `ModelingTableCard` component (3-section card + type icons)

Extract the inline `<article>` card render from `ModelingCanvas` into a focused component with header + kebab menu, Columns (with type icons), Calculated Fields, and Relationships sections. Header stays the drag/keyboard handle **and** becomes the modal click target (wired in Task 5 — this task exposes the callbacks and renders the sections).

**Files:**
- Create: `frontend/src/components/modeling/ModelingTableCard.tsx`
- Modify: `frontend/src/lib/modelingClasses.ts` (section + type-icon + kebab classes)
- Modify: `frontend/src/components/modeling/ModelingCanvas.tsx:118-182` (render `ModelingTableCard`)
- Add i18n keys (see Step 3).

**Interfaces:**
- Consumes: `CardLayout` (Task 3, has `columnsShown`, `hiddenCount`, `calcFieldCount`, `relatedTables`), `columnTypeIcon` (Task 2), `modelingTableCardClass`/`modelingTableRowClass`/`modelingColumnNameClass` + new class exports, `formatDataType`/`tableKey` from `./utils`.
- Produces: `ModelingTableCard` component with props:

```ts
interface ModelingTableCardProps {
  table: TableRow
  layout: CardLayout
  pos: { x: number; y: number }
  isBase: boolean
  isHi: boolean
  highlightedColumns: Set<string> | undefined
  highlightedJoinColumns: { from: string; to: string } | null
  onDragStart: (event: React.MouseEvent) => void
  onKeyDown: (event: React.KeyboardEvent) => void
  onOpenDetail: () => void
  onAddCalcField: () => void
  onAddRelationship: () => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}
```

- [ ] **Step 1: Add class helpers**

In `frontend/src/lib/modelingClasses.ts`, after `modelingColumnNameClass` (line 345), add:

```ts
export const modelingTypeIconClass = cn(
  'shrink-0 inline-flex h-[1.05rem] min-w-[1.5rem] items-center justify-center rounded',
  'bg-[color-mix(in_srgb,var(--foreground)_8%,transparent)] px-[0.25rem]',
  'text-[0.58rem] font-bold text-foreground-muted tabular-nums',
)

export const modelingCardSectionClass = cn(
  'flex items-center justify-between border-t border-border px-3 py-[0.28rem]',
  'text-[0.66rem] font-semibold tracking-wide text-foreground-muted uppercase',
)

export const modelingCardSectionAddClass = cn(
  'flex h-[1.15rem] w-[1.15rem] items-center justify-center rounded border-0',
  'cursor-pointer bg-transparent text-[0.85rem] leading-none text-foreground-muted',
  'hover:bg-[color-mix(in_srgb,var(--accent)_14%,transparent)] hover:text-accent',
)

export const modelingRelRowClass =
  'flex items-center gap-[0.4rem] px-3 py-[0.18rem] text-[0.72rem] text-foreground-muted'

export const modelingKebabClass = cn(
  'ml-auto flex h-[1.3rem] w-[1.3rem] items-center justify-center rounded border-0',
  'cursor-pointer bg-transparent text-white/80 text-[0.95rem] leading-none',
  'hover:bg-white/15 hover:text-white',
)

export const modelingCardHeaderRowClass = 'flex items-center gap-[0.4rem]'
```

- [ ] **Step 2: Create the component**

Create `frontend/src/components/modeling/ModelingTableCard.tsx`:

```tsx
import type { TranslationKey } from '../../i18n'
import {
  modelingCardHeaderRowClass,
  modelingCardSectionAddClass,
  modelingCardSectionClass,
  modelingColumnNameClass,
  modelingKebabClass,
  modelingRelRowClass,
  modelingTableCardClass,
  modelingTableRowClass,
  modelingTypeIconClass,
} from '../../lib/modelingClasses'
import type { TableRow } from '../../types/semantic'
import { CARD_WIDTH } from './constants'
import { columnTypeIcon } from './columnTypeIcon'
import type { CardLayout } from './types'
import { formatDataType } from './utils'

interface ModelingTableCardProps {
  table: TableRow
  layout: CardLayout
  pos: { x: number; y: number }
  isBase: boolean
  isHi: boolean
  highlightedColumns: Set<string> | undefined
  highlightedJoinColumns: { from: string; to: string } | null
  onDragStart: (event: React.MouseEvent) => void
  onKeyDown: (event: React.KeyboardEvent) => void
  onOpenDetail: () => void
  onAddCalcField: () => void
  onAddRelationship: () => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

export function ModelingTableCard({
  table,
  layout,
  pos,
  isBase,
  isHi,
  highlightedColumns,
  highlightedJoinColumns,
  onDragStart,
  onKeyDown,
  onOpenDetail,
  onAddCalcField,
  onAddRelationship,
  t,
}: ModelingTableCardProps) {
  const key = `${table.schema_name}.${table.table_name}`
  return (
    <article
      className={modelingTableCardClass({ base: isBase, hi: isHi })}
      style={{ left: pos.x, top: pos.y, width: CARD_WIDTH, height: layout.height }}
    >
      <header
        role="button"
        tabIndex={0}
        aria-label={t('modeling.table_card_aria', {
          name: `${table.schema_name}.${table.table_name}`,
        })}
        onMouseDown={onDragStart}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onOpenDetail()
            return
          }
          onKeyDown(e)
        }}
      >
        <div className={modelingCardHeaderRowClass}>
          <strong>{table.table_name}</strong>
          <button
            type="button"
            className={modelingKebabClass}
            aria-label={t('modeling.table_card_menu', {
              name: `${table.schema_name}.${table.table_name}`,
            })}
            onMouseDown={(e) => e.stopPropagation()}
            onClick={onOpenDetail}
          >
            ⋮
          </button>
        </div>
        <span>{table.schema_name}</span>
      </header>

      <ul>
        {layout.columnsShown.map((column) => {
          const isJoinCol = highlightedColumns?.has(column.column_name)
          const colKey = `${key}::${column.column_name}`
          const isActiveJoinCol =
            !!highlightedJoinColumns &&
            (highlightedJoinColumns.from === colKey || highlightedJoinColumns.to === colKey)
          const icon = columnTypeIcon(column.data_type)
          return (
            <li
              key={column.id}
              className={modelingTableRowClass({ joined: isJoinCol, active: isActiveJoinCol })}
            >
              <span className={modelingColumnNameClass}>
                <span className={modelingTypeIconClass} aria-hidden="true">
                  {icon.glyph}
                </span>
                {column.is_primary_key && <b>{t('modeling.pk_badge')}</b>}
                {column.is_foreign_key && <b>{t('modeling.fk_badge')}</b>}
                {column.column_name}
              </span>
              <small>{formatDataType(t, column.data_type)}</small>
            </li>
          )
        })}
        {layout.hiddenCount > 0 && (
          <li className={modelingTableRowClass({ more: true })}>
            +{layout.hiddenCount} {t('modeling.more_columns')}
          </li>
        )}
      </ul>

      <div className={modelingCardSectionClass}>
        <span>
          {t('modeling.calc_fields_section')} ({layout.calcFieldCount})
        </span>
        <button
          type="button"
          className={modelingCardSectionAddClass}
          aria-label={t('modeling.add_calc_field')}
          onMouseDown={(e) => e.stopPropagation()}
          onClick={onAddCalcField}
        >
          ＋
        </button>
      </div>

      <div className={modelingCardSectionClass}>
        <span>
          {t('modeling.relationships_section')} ({layout.relatedTables.length})
        </span>
        <button
          type="button"
          className={modelingCardSectionAddClass}
          aria-label={t('modeling.add_relationship')}
          onMouseDown={(e) => e.stopPropagation()}
          onClick={onAddRelationship}
        >
          ＋
        </button>
      </div>
      {layout.relatedTables.map((rel, i) => (
        <div key={`${rel}-${i}`} className={modelingRelRowClass}>
          <span aria-hidden="true">⇄</span>
          <span className="overflow-hidden text-ellipsis whitespace-nowrap">{rel}</span>
        </div>
      ))}
    </article>
  )
}
```

- [ ] **Step 3: Add i18n keys**

Add these keys to the i18n catalog (search for where `modeling.more_columns` / `modeling.table_card_aria` are defined and add alongside, for every locale):

```
modeling.table_card_menu: "Open {name} details"        // TR: "{name} ayrıntılarını aç"
modeling.calc_fields_section: "Calculated Fields"       // TR: "Hesaplanan Alanlar"
modeling.relationships_section: "Relationships"         // TR: "İlişkiler"
modeling.add_calc_field: "Add calculated field"         // TR: "Hesaplanan alan ekle"
modeling.add_relationship: "Add relationship"           // TR: "İlişki ekle"
```

(Match the existing catalog format; if keys are typed in `frontend/src/i18n`, add them to the `TranslationKey` union / message maps so `tsc` passes.)

- [ ] **Step 4: Render `ModelingTableCard` from `ModelingCanvas`**

In `frontend/src/components/modeling/ModelingCanvas.tsx`:

Add the import:

```ts
import { ModelingTableCard } from './ModelingTableCard'
```

Extend `ModelingCanvasProps` with the new callbacks (these are supplied by Task 5's wiring):

```ts
  onOpenTableDetail: (table: TableRow) => void
  onAddCalcField: (table: TableRow) => void
  onAddRelationship: (table: TableRow) => void
```

Destructure them in the component signature, and replace the `tableCards.map(...)` `<article>` block (lines 118-182) with:

```tsx
        {tableCards.map((table) => {
          const key = tableKey(table.schema_name, table.table_name)
          const pos = positions[key] ?? { x: 0, y: 0 }
          const layout = cardLayouts.get(key)
          if (!layout) {
            return null
          }
          const isHi = highlightedTables?.has(key) ?? false
          if (highlightedTables && !isHi) {
            return null
          }
          return (
            <ModelingTableCard
              key={key}
              table={table}
              layout={layout}
              pos={pos}
              isBase={baseKey === key}
              isHi={isHi}
              highlightedColumns={highlightedColumns.get(key)}
              highlightedJoinColumns={highlightedJoinColumns}
              onDragStart={onCardDragStart(key)}
              onKeyDown={onCardKeyDown(key)}
              onOpenDetail={() => onOpenTableDetail(table)}
              onAddCalcField={() => onAddCalcField(table)}
              onAddRelationship={() => onAddRelationship(table)}
              t={t}
            />
          )
        })}
```

Now-unused imports in `ModelingCanvas.tsx` (`modelingColumnNameClass`, `modelingTableCardClass`, `modelingTableRowClass`, `formatDataType`, `CARD_WIDTH` if no longer referenced) must be removed to keep eslint/knip green — verify against remaining usage (the SVG block still uses `CARD_WIDTH`? it does not; edges use `getJoinPath`. Remove whatever the linter flags).

- [ ] **Step 5: Wire temporary no-op props so the app compiles**

In `frontend/src/components/Modeling.tsx`, pass temporary handlers to `<ModelingCanvas>` (replaced with real ones in Task 5) so the build passes in isolation:

```tsx
              onOpenTableDetail={() => {}}
              onAddCalcField={() => {}}
              onAddRelationship={() => {}}
```

- [ ] **Step 6: Run gate + manual check**

Run: `make check-frontend`
Expected: PASS.
Manual: cards show a kebab, per-column type-icon badges, and Calculated Fields / Relationships sections with counts and ＋ buttons; related-table rows render; drag still works from the header; join edges still land on the right column rows.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/modeling/ModelingTableCard.tsx frontend/src/lib/modelingClasses.ts frontend/src/components/modeling/ModelingCanvas.tsx frontend/src/components/Modeling.tsx frontend/src/i18n
git commit -m "feat(modeling): 3-section table card with type icons"
```

---

### Task 5: `TableDetailModal` + click-to-open wiring

Add the detail modal (Name / Alias / Description + Edit, columns table, relationships table, on-demand 100-row preview) and wire the card header click, disambiguated from drag by a movement threshold. Wire the ＋ section buttons to existing add flows.

**Files:**
- Create: `frontend/src/components/modeling/TableDetailModal.tsx`
- Modify: `frontend/src/components/modeling/useModelingCanvas.ts` (drag-vs-click threshold)
- Modify: `frontend/src/components/Modeling.tsx` (detail state, real handlers, render modal)
- Test: `frontend/src/components/modeling/TableDetailModal.test.tsx`
- Add i18n keys (see Step 5).

**Interfaces:**
- Consumes: `Modal` (`../ui/Modal`, props `{ open, title, subtitle?, onClose, children }`), `DataTable` (`../ui/DataTable`), `columnTypeIcon` (Task 2), `buildTableRowsUrl` + `tableRowsBody` + `TableRowsResult` (`../tableBrowser/useTableBrowserQueryState`), `columnOptions` (`./utils`), types `ColumnRow`/`SemanticModelDetail`/`SemanticJoin`/`TableRow`.
- Consumes from `useModelingPageState` (already returned): `columns`, `model`, `datasourceId`, `postData`, `renameTable`, `setAddMetricOpen`, `setEditorOpen` (or `toggleEditor`), `t`.
- Produces: `TableDetailModal` with props:

```ts
interface TableDetailModalProps {
  open: boolean
  table: TableRow | null
  model: SemanticModelDetail | null
  columns: ColumnRow[]
  datasourceId: string
  postData: <T>(url: string, body: unknown) => Promise<T | null>
  onClose: () => void
  onEdit: (table: TableRow) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}
```

- [ ] **Step 1: Add the drag-vs-click threshold to `onCardDragStart`**

In `frontend/src/components/modeling/useModelingCanvas.ts`, the hook must accept an `onCardClick` callback and only fire it when the pointer barely moved. Change the hook signature to accept a fifth arg:

```ts
export function useModelingCanvas(
  modelId: string,
  tableCards: TableRow[],
  columns: ColumnRow[],
  model: SemanticModelDetail | null,
  onCardClick?: (key: string) => void,
) {
```

In `onCardDragStart`, track movement and invoke `onCardClick` on mouseup if under threshold. Replace the `onMove`/`onUp` region (lines 90-112) with:

```ts
      let moved = false
      const CLICK_THRESHOLD = 4
      const onMove = (ev: MouseEvent) => {
        const scale = viewportRef.current.scale
        const dx = ev.clientX - startX
        const dy = ev.clientY - startY
        if (Math.abs(dx) > CLICK_THRESHOLD || Math.abs(dy) > CLICK_THRESHOLD) {
          moved = true
        }
        setPositions((prev) => ({
          ...prev,
          [key]: applyDragDelta(startPos, dx, dy, scale),
        }))
      }
      const onUp = () => {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        window.removeEventListener('blur', onUp)
        document.body.classList.remove('modeling-grabbing')
        activeDragCleanupRef.current = null
        if (!moved) {
          onCardClick?.(key)
        }
      }
```

Add `onCardClick` to the `onCardDragStart` `useCallback` dependency array (`[positions, onCardClick]`).

- [ ] **Step 2: Write the failing modal test**

Create `frontend/src/components/modeling/TableDetailModal.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import type { ColumnRow, SemanticModelDetail, TableRow } from '../../types/semantic'
import { TableDetailModal } from './TableDetailModal'

const t = ((k: string) => k) as never
const table = { schema_name: 'public', table_name: 'orders', id: 'public.orders' } as TableRow
const columns: ColumnRow[] = [
  {
    id: 'c1',
    schema_name: 'public',
    table_name: 'orders',
    column_name: 'amount',
    data_type: 'numeric',
    nullable: false,
    description: 'Order amount',
    is_primary_key: false,
    is_foreign_key: false,
    referenced_table: null,
    referenced_column: null,
  },
]
const model = { base_schema: 'public', base_table: 'orders', joins: [] } as unknown as SemanticModelDetail

describe('TableDetailModal', () => {
  it('renders the table name and its columns when open', () => {
    render(
      <TableDetailModal
        open
        table={table}
        model={model}
        columns={columns}
        datasourceId="ds1"
        postData={vi.fn().mockResolvedValue(null)}
        onClose={vi.fn()}
        onEdit={vi.fn()}
        t={t}
      />,
    )
    expect(screen.getByText('orders')).toBeInTheDocument()
    expect(screen.getByText('amount')).toBeInTheDocument()
  })

  it('renders nothing when closed', () => {
    const { container } = render(
      <TableDetailModal
        open={false}
        table={table}
        model={model}
        columns={columns}
        datasourceId="ds1"
        postData={vi.fn()}
        onClose={vi.fn()}
        onEdit={vi.fn()}
        t={t}
      />,
    )
    expect(container).toBeEmptyDOMElement()
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `npm --prefix frontend run test -- TableDetailModal`
Expected: FAIL — cannot resolve `./TableDetailModal`.

- [ ] **Step 4: Implement `TableDetailModal`**

Create `frontend/src/components/modeling/TableDetailModal.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { modelingTypeIconClass } from '../../lib/modelingClasses'
import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  buildTableRowsUrl,
  tableRowsBody,
  type TableRowsResult,
} from '../tableBrowser/useTableBrowserQueryState'
import { Modal } from '../ui/Modal'
import { columnTypeIcon } from './columnTypeIcon'
import { columnOptions, tableKey } from './utils'

interface TableDetailModalProps {
  open: boolean
  table: TableRow | null
  model: SemanticModelDetail | null
  columns: ColumnRow[]
  datasourceId: string
  postData: <T>(url: string, body: unknown) => Promise<T | null>
  onClose: () => void
  onEdit: (table: TableRow) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

const PREVIEW_LIMIT = 100

export function TableDetailModal({
  open,
  table,
  model,
  columns,
  datasourceId,
  postData,
  onClose,
  onEdit,
  t,
}: TableDetailModalProps) {
  const [preview, setPreview] = useState<TableRowsResult | null>(null)
  const [previewing, setPreviewing] = useState(false)

  // Reset the preview whenever the inspected table changes.
  useEffect(() => {
    setPreview(null)
    setPreviewing(false)
  }, [table])

  const loadPreview = useCallback(async () => {
    if (!table) {
      return
    }
    setPreviewing(true)
    const res = await postData<TableRowsResult>(
      buildTableRowsUrl(datasourceId, table.schema_name, table.table_name),
      tableRowsBody([], null, PREVIEW_LIMIT, 0),
    )
    if (res) {
      setPreview(res)
    }
    setPreviewing(false)
  }, [table, datasourceId, postData])

  if (!open || !table) {
    return null
  }

  const key = tableKey(table.schema_name, table.table_name)
  const cols = columnOptions(columns, key)
  const rels: SemanticJoin[] = (model?.joins ?? []).filter((j) => {
    const from = tableKey(j.from_schema ?? model?.base_schema ?? '', j.from_table)
    const to = tableKey(j.to_schema ?? model?.base_schema ?? '', j.to_table)
    return from === key || to === key
  })
  const previewCols = preview?.columns ?? []
  const previewRows = preview?.rows ?? []

  return (
    <Modal
      open={open}
      title={table.label || table.table_name}
      subtitle={`${table.schema_name}.${table.table_name}`}
      onClose={onClose}
    >
      <div className="flex flex-col gap-4 text-sm">
        <div className="flex items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <span className="text-foreground-muted text-xs">{t('modeling.detail_alias')}</span>
            <span>{table.label || table.table_name}</span>
          </div>
          <button type="button" className={buttonClass('secondary', { size: 'sm' })} onClick={() => onEdit(table)}>
            {t('modeling.detail_edit')}
          </button>
        </div>
        {table.description && <p className="text-foreground-muted">{table.description}</p>}

        <section>
          <h3 className="mb-2 font-semibold">
            {t('modeling.detail_columns')} ({cols.length})
          </h3>
          <ul className="flex flex-col gap-1">
            {cols.map((c) => {
              const icon = columnTypeIcon(c.data_type)
              return (
                <li key={c.id} className="flex items-center gap-2">
                  <span className={modelingTypeIconClass} aria-hidden="true">
                    {icon.glyph}
                  </span>
                  <span className="font-medium">{c.column_name}</span>
                  <span className="text-foreground-muted text-xs">{c.data_type}</span>
                  {c.description && (
                    <span className="text-foreground-muted ml-auto truncate text-xs">{c.description}</span>
                  )}
                </li>
              )
            })}
          </ul>
        </section>

        {rels.length > 0 && (
          <section>
            <h3 className="mb-2 font-semibold">
              {t('modeling.detail_relationships')} ({rels.length})
            </h3>
            <ul className="flex flex-col gap-1">
              {rels.map((r) => (
                <li key={r.id} className="text-foreground-muted flex items-center gap-2 text-xs">
                  <span className="font-medium">{r.name}</span>
                  <span>
                    {r.from_table} → {r.to_table}
                  </span>
                  <span className="ml-auto">{r.relationship}</span>
                </li>
              ))}
            </ul>
          </section>
        )}

        <section>
          <div className="mb-2 flex items-center gap-2">
            <h3 className="font-semibold">{t('modeling.detail_preview')}</h3>
            <button
              type="button"
              className={buttonClass('secondary', { size: 'sm' })}
              onClick={() => void loadPreview()}
              disabled={previewing}
            >
              {previewing ? t('modeling.detail_preview_loading') : t('modeling.detail_preview_btn')}
            </button>
          </div>
          {previewRows.length > 0 && (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr>
                    {previewCols.map((c) => (
                      <th key={c.name} className="border-border border-b px-2 py-1 text-left font-semibold">
                        {c.name}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {previewRows.map((row, ri) => (
                    <tr key={ri}>
                      {row.map((cell, ci) => (
                        <td key={ci} className="border-border/50 border-b px-2 py-1">
                          {cell === null || cell === undefined ? '' : String(cell)}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </Modal>
  )
}
```

> The preview renders a plain scrollable table (not `DataTable`) because the rows endpoint returns positional `unknown[][]` + a `columns` header list, which matches this simple render; `DataTable` expects typed row objects. Keep it minimal.

- [ ] **Step 5: Add i18n keys**

Add to the catalog (all locales), alongside the Task 4 keys:

```
modeling.detail_alias: "Alias"                    // TR: "Takma ad"
modeling.detail_edit: "Edit"                       // TR: "Düzenle"
modeling.detail_columns: "Columns"                 // TR: "Sütunlar"
modeling.detail_relationships: "Relationships"     // TR: "İlişkiler"
modeling.detail_preview: "Data preview"            // TR: "Veri önizleme"
modeling.detail_preview_btn: "Preview data"        // TR: "Veriyi önizle"
modeling.detail_preview_loading: "Loading…"        // TR: "Yükleniyor…"
```

- [ ] **Step 6: Run modal test**

Run: `npm --prefix frontend run test -- TableDetailModal`
Expected: PASS.

- [ ] **Step 7: Wire real handlers in `Modeling.tsx`**

In `frontend/src/components/Modeling.tsx`:

Add imports + local state:

```tsx
import { TableDetailModal } from './modeling/TableDetailModal'
// ...
const [detailTable, setDetailTable] = useState<TableRow | null>(null)
```

(import `TableRow` type where the file imports semantic types; add `import type { TableRow } from './types/semantic'` if not present — path is `../types/semantic` relative to `components/` → `./types/semantic` is wrong; use `../types/semantic`.)

Replace the temporary no-op props on `<ModelingCanvas>` (from Task 4 Step 5) with:

```tsx
              onOpenTableDetail={(table) => setDetailTable(table)}
              onAddCalcField={() => s.setAddMetricOpen(true)}
              onAddRelationship={() => s.setEditorOpen(true)}
```

> `s.setEditorOpen` opens the JoinEditor panel; if `useModelingPageState` exposes only `toggleEditor`, use `() => { if (!s.editorOpen) s.toggleEditor() }`. Verify the exact exported name before wiring; do not invent a setter.

Render the modal near `ModelVersionsModal` (inside the `s.model &&` region is fine):

```tsx
      <TableDetailModal
        open={detailTable !== null}
        table={detailTable}
        model={s.model}
        columns={s.columns}
        datasourceId={s.datasourceId}
        postData={s.postData}
        onClose={() => setDetailTable(null)}
        onEdit={(table) => {
          setDetailTable(null)
          s.renameTable(table)
        }}
        t={s.t}
      />
```

> Verify `s.renameTable`'s exact signature (from `useModelingPageState` / passed to `ModelingPalette` as `onRenameTable`). In `Modeling.tsx` it is already used as `onRenameTable={s.renameTable}`. If it takes `(schema, table)` rather than a `TableRow`, adapt the call accordingly (`s.renameTable(table.schema_name, table.table_name)`).

- [ ] **Step 8: Pass `onCardClick` into the canvas hook**

The `useModelingCanvas` call lives inside `useModelingPageState`. Find it (search `useModelingCanvas(`) and pass a click handler that stores the clicked table so `Modeling.tsx` can open the modal. Simplest wiring that avoids a second state source: have `useModelingPageState` expose `detailTable`/`setDetailTable` itself and pass `(key) => setDetailTableByKey(key)` where it resolves the key to a `TableRow` via `splitTableKey` + `tableCards`. Concretely, in `useModelingPageState`:

```ts
const [detailTable, setDetailTable] = useState<TableRow | null>(null)
const openDetailByKey = useCallback(
  (key: string) => {
    const { schema, table } = splitTableKey(key)
    const found = tableCards.find((c) => c.schema_name === schema && c.table_name === table)
    if (found) {
      setDetailTable(found)
    }
  },
  [tableCards],
)
const canvas = useModelingCanvas(modelId, tableCards, columns, model, openDetailByKey)
```

Return `detailTable` and `setDetailTable` from the hook, and in `Modeling.tsx` use `s.detailTable` / `s.setDetailTable` instead of the local `useState` from Step 7. (This removes the local state and the header kebab/`onOpenTableDetail` both funnel through the same setter — pick this single-source approach; delete the local `useState` added in Step 7.)

> Import `splitTableKey` from `./utils` and `useState`/`useCallback` in `useModelingPageState` if not already imported.

- [ ] **Step 9: Run gate + manual check**

Run: `make check-frontend`
Expected: PASS.
Manual: single-clicking a card header (or its kebab) opens the modal with columns, relationships, and a working "Preview data" (100 rows); dragging a card does NOT open the modal; Edit opens the rename flow; ＋ on Calculated Fields opens the add-metric modal; ＋ on Relationships opens the JoinEditor; Esc / backdrop closes; keyboard focus is trapped in the modal.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/modeling/TableDetailModal.tsx frontend/src/components/modeling/TableDetailModal.test.tsx frontend/src/components/modeling/useModelingCanvas.ts frontend/src/components/modeling/useModelingPageState.ts frontend/src/components/Modeling.tsx frontend/src/i18n
git commit -m "feat(modeling): click-to-detail table modal with data preview"
```

---

## Self-Review

**Spec coverage:**
- §0 scroll-zoom fix → Task 1 (continuous zoom + `touch-action: none`). ✓
- §1 3-section card + type icons → Task 2 (icons) + Task 3 (section data/height) + Task 4 (component). ✓
- §2 click→detail modal (Name/Alias/Description+Edit, columns, relationships, 100-row preview) → Task 5. ✓
- "No backend changes" → all tasks are frontend; preview reuses the existing rows endpoint. ✓
- Column geometry invariant (edges keep landing) → Task 3 keeps Columns first; height grows downward only. ✓

**Placeholder scan:** No TBD/TODO; every code step has concrete code. Two explicit "verify the exact exported name" notes (Task 5 Steps 7-8) are guardrails against inventing symbols in `useModelingPageState`, not deferred work — the implementer confirms the real setter name from the file.

**Type consistency:** `CardLayout` gains `calcFieldCount`/`relatedTables` in Task 3 and is consumed with those exact names in Task 4. `columnTypeIcon` returns `{ kind, glyph }` (Task 2), consumed as `icon.glyph` in Tasks 4 & 5. `continuousZoomScale(scale, deltaY)` defined in Task 1, used in Task 1 Step 5. `buildCardLayouts` 5th arg `sections` added in Task 3 and passed by its only caller in the same task. `TableRowsResult` / `buildTableRowsUrl` / `tableRowsBody` imported from the existing `useTableBrowserQueryState` in Task 5.

**Risk note:** The riskiest coupling is card height (Task 3) vs. existing `canvasMath.test.ts` assertions — heights change, so any test asserting an exact card height must be updated to pass a `CardSection` (tests that assert edge geometry via `computeJoinPath` are unaffected because Columns geometry is unchanged). Run the full `canvasMath` suite in Task 3 Step 7 and update height-dependent assertions inline.
