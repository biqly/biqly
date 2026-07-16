import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  CALC_SECTION_HEIGHT,
  CARD_BOTTOM_PAD,
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
import { columnOptions, columnRefMatchesTable, compareColumns, tableKey } from './utils'

export function clampScale(scale: number) {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale))
}

export function zoomStepIndex(scale: number) {
  const clamped = clampScale(scale)
  let idx = 0
  for (let i = 0; i < ZOOM_STEPS.length; i++) {
    const step = ZOOM_STEPS[i]
    if (step !== undefined && step <= clamped + 1e-9) {
      idx = i
    }
  }
  return idx
}

export function zoomStep(scale: number, direction: 1 | -1) {
  const idx = zoomStepIndex(scale)
  const next = Math.min(ZOOM_STEPS.length - 1, Math.max(0, idx + direction))
  return ZOOM_STEPS[next] ?? clampScale(scale)
}

// Wheel and trackpad gestures emit different delta magnitudes and event rates.
// Scaling multiplicatively keeps both inputs smooth instead of stepping through
// the discrete button-zoom ladder.
const ZOOM_WHEEL_SENSITIVITY = 0.0015

export function continuousZoomScale(scale: number, deltaY: number): number {
  return clampScale(scale * Math.exp(-deltaY * ZOOM_WHEEL_SENSITIVITY))
}

export function snapScaleNearest(scale: number) {
  const clamped = clampScale(scale)
  let best = ZOOM_STEPS[0] ?? MIN_SCALE
  for (const step of ZOOM_STEPS) {
    if (Math.abs(step - clamped) < Math.abs(best - clamped)) {
      best = step
    }
  }
  return best
}

export const cardHeight = (count: number, section?: CardSection) => {
  const base = HEADER_HEIGHT + count * ROW_HEIGHT + CARD_PAD_Y * 2
  if (!section) {
    return base
  }
  return (
    base +
    CALC_SECTION_HEIGHT +
    REL_SECTION_LABEL_HEIGHT +
    section.relatedTables.length * ROW_HEIGHT +
    CARD_BOTTOM_PAD
  )
}
// Y offset (within a card) of a column row's center, accounting for the
// scrollable list window: rows scrolled out of view clamp to the window edge
// so join lines stay attached to the visible list area.
export function rowCenterY(idx: number, layout?: CardLayout, scrollTop = 0) {
  const contentY = CARD_PAD_Y + idx * ROW_HEIGHT + ROW_HEIGHT / 2
  if (!layout) {
    return HEADER_HEIGHT + contentY
  }
  const listHeight = CARD_PAD_Y * 2 + layout.visibleRowCount * ROW_HEIGHT
  const clamped = Math.min(Math.max(contentY - scrollTop, 0), listHeight)
  return HEADER_HEIGHT + clamped
}

export function applyDragDelta(startPos: Pt, dx: number, dy: number, scale: number): Pt {
  return {
    x: Math.max(0, startPos.x + dx / scale),
    y: Math.max(0, startPos.y + dy / scale),
  }
}

export function exceedsDragThreshold(dx: number, dy: number, threshold = 4): boolean {
  return Math.abs(dx) > threshold || Math.abs(dy) > threshold
}

export function applyKeyboardMove(cur: Pt, dx: number, dy: number, shiftKey: boolean): Pt {
  const step = shiftKey ? KEYBOARD_MOVE_STEP_SHIFT : KEYBOARD_MOVE_STEP
  return {
    x: Math.max(0, cur.x + dx * step),
    y: Math.max(0, cur.y + dy * step),
  }
}

export function keyboardDeltaFromKey(key: string): { dx: number; dy: number } | null {
  switch (key) {
    case 'ArrowLeft':
      return { dx: -1, dy: 0 }
    case 'ArrowRight':
      return { dx: 1, dy: 0 }
    case 'ArrowUp':
      return { dx: 0, dy: -1 }
    case 'ArrowDown':
      return { dx: 0, dy: 1 }
    default:
      return null
  }
}

export function panViewport(start: Viewport, dx: number, dy: number): Viewport {
  return { ...start, tx: start.tx + dx, ty: start.ty + dy }
}

export function zoomViewportAtPoint(
  vp: Viewport,
  cx: number,
  cy: number,
  newScale: number,
): Viewport {
  if (newScale === vp.scale) {
    return vp
  }
  const k = newScale / vp.scale
  return { scale: newScale, tx: cx - k * (cx - vp.tx), ty: cy - k * (cy - vp.ty) }
}

export function layoutInitialPositions(
  tableCards: TableRow[],
  cardLayouts: Map<string, CardLayout>,
): Record<string, Pt> {
  const next: Record<string, Pt> = {}
  const colCursors: number[] = Array.from({ length: LAYOUT_COLS }, () => ORIGIN_Y)
  tableCards.forEach((tbl, i) => {
    const key = tableKey(tbl.schema_name, tbl.table_name)
    const layout = cardLayouts.get(key)
    const h = layout?.height ?? HEADER_HEIGHT + 4 * ROW_HEIGHT
    const col = i % LAYOUT_COLS
    const cursorY = colCursors[col] ?? ORIGIN_Y
    next[key] = { x: ORIGIN_X + col * GRID_X, y: cursorY }
    colCursors[col] = Math.max(cursorY, next[key].y + h + GRID_Y)
  })
  return next
}

export type LayoutPreset = 'grid' | 'hierarchic' | 'compact'

// Masonry-style packing: each card goes to the currently shortest column, so
// tall and short cards mix without the fixed row rhythm of the grid preset.
export function layoutCompact(
  tableCards: TableRow[],
  cardLayouts: Map<string, CardLayout>,
  cols = LAYOUT_COLS,
): Record<string, Pt> {
  const next: Record<string, Pt> = {}
  const colHeights: number[] = Array.from({ length: cols }, () => ORIGIN_Y)
  for (const tbl of tableCards) {
    const key = tableKey(tbl.schema_name, tbl.table_name)
    const h = cardLayouts.get(key)?.height ?? HEADER_HEIGHT + 4 * ROW_HEIGHT
    let col = 0
    for (let i = 1; i < cols; i++) {
      if (colHeights[i]! < colHeights[col]!) {
        col = i
      }
    }
    next[key] = { x: ORIGIN_X + col * GRID_X, y: colHeights[col]! }
    colHeights[col] = colHeights[col]! + h + GRID_Y / 2
  }
  return next
}

// Relationship-driven layered layout: BFS over active joins puts the base
// table in the leftmost column and each join hop one column to the right;
// tables without any relationship land in a final column on the far right.
function buildJoinAdjacency(
  joins: SemanticJoin[],
  baseSchema: string,
  keySet: Set<string>,
): Map<string, string[]> {
  const adjacency = new Map<string, string[]>()
  const addEdge = (a: string, b: string) => {
    if (!adjacency.has(a)) {
      adjacency.set(a, [])
    }
    adjacency.get(a)!.push(b)
  }
  for (const join of joins) {
    if (join.is_active === false) {
      continue
    }
    const a = tableKey(join.from_schema ?? baseSchema, join.from_table)
    const b = tableKey(join.to_schema ?? baseSchema, join.to_table)
    if (!keySet.has(a) || !keySet.has(b) || a === b) {
      continue
    }
    addEdge(a, b)
    addEdge(b, a)
  }
  return adjacency
}

// BFS distance from `baseKey` over the join graph; tables unreachable from it
// (including when there is no base) get BFS'd from their own component so
// every connected table still lands next to its neighbors, and anything left
// fully isolated is parked one column past the deepest connected table.
function computeJoinDepths(
  keys: string[],
  adjacency: Map<string, string[]>,
  keySet: Set<string>,
  baseKey?: string,
): Map<string, number> {
  const depth = new Map<string, number>()
  const bfs = (start: string) => {
    depth.set(start, 0)
    const queue = [start]
    while (queue.length > 0) {
      const cur = queue.shift()!
      for (const nb of adjacency.get(cur) ?? []) {
        if (!depth.has(nb)) {
          depth.set(nb, depth.get(cur)! + 1)
          queue.push(nb)
        }
      }
    }
  }
  if (baseKey && keySet.has(baseKey)) {
    bfs(baseKey)
  }
  for (const key of keys) {
    if (!depth.has(key) && (adjacency.get(key)?.length ?? 0) > 0) {
      bfs(key)
    }
  }
  let maxDepth = 0
  for (const d of depth.values()) {
    maxDepth = Math.max(maxDepth, d)
  }
  const isolatedCol = depth.size > 0 ? maxDepth + 1 : 0
  for (const key of keys) {
    if (!depth.has(key)) {
      depth.set(key, isolatedCol)
    }
  }
  return depth
}

export function layoutHierarchic(
  tableCards: TableRow[],
  cardLayouts: Map<string, CardLayout>,
  joins: SemanticJoin[],
  baseSchema: string,
  baseKey?: string,
): Record<string, Pt> {
  const keys = tableCards.map((tbl) => tableKey(tbl.schema_name, tbl.table_name))
  const keySet = new Set(keys)
  const adjacency = buildJoinAdjacency(joins, baseSchema, keySet)
  const depth = computeJoinDepths(keys, adjacency, keySet, baseKey)

  const next: Record<string, Pt> = {}
  const colCursors = new Map<number, number>()
  for (const key of keys) {
    const col = depth.get(key)!
    const y = colCursors.get(col) ?? ORIGIN_Y
    const h = cardLayouts.get(key)?.height ?? HEADER_HEIGHT + 4 * ROW_HEIGHT
    next[key] = { x: ORIGIN_X + col * GRID_X, y }
    colCursors.set(col, y + h + GRID_Y / 2)
  }
  return next
}

// Pushes overlapping cards downward until no two cards intersect. Card heights
// change after the initial grid layout (model detail loads, column windows
// resize), and preserved positions can then collide; this reopens the gaps
// deterministically (only y grows, top-most card of a pair never moves).
export function resolveOverlaps(
  positions: Record<string, Pt>,
  cardLayouts: Map<string, CardLayout>,
  gap = GRID_Y / 2,
): Record<string, Pt> {
  const keys = Object.keys(positions).filter((key) => cardLayouts.has(key))
  const next: Record<string, Pt> = { ...positions }
  const heightOf = (key: string) => cardLayouts.get(key)?.height ?? 0
  let movedAny = false
  const maxPasses = 10
  for (let pass = 0; pass < maxPasses; pass++) {
    keys.sort((a, b) => next[a]!.y - next[b]!.y || next[a]!.x - next[b]!.x)
    let moved = false
    for (let i = 0; i < keys.length; i++) {
      const upper = keys[i]!
      const up = next[upper]!
      for (let j = i + 1; j < keys.length; j++) {
        const lower = keys[j]!
        const lp = next[lower]!
        const xOverlap = up.x < lp.x + CARD_WIDTH && lp.x < up.x + CARD_WIDTH
        const yOverlap = up.y < lp.y + heightOf(lower) && lp.y < up.y + heightOf(upper)
        if (xOverlap && yOverlap) {
          next[lower] = { x: lp.x, y: up.y + heightOf(upper) + gap }
          moved = true
          movedAny = true
        }
      }
    }
    if (!moved) {
      break
    }
  }
  return movedAny ? next : positions
}

export function computeCanvasBounds(
  tableCards: TableRow[],
  positions: Record<string, Pt>,
  cardLayouts: Map<string, CardLayout>,
): { width: number; height: number } {
  let maxX = 1
  let maxY = 1
  for (const tbl of tableCards) {
    const key = tableKey(tbl.schema_name, tbl.table_name)
    const pos = positions[key]
    const layout = cardLayouts.get(key)
    if (!pos || !layout) {
      continue
    }
    maxX = Math.max(maxX, pos.x + CARD_WIDTH)
    maxY = Math.max(maxY, pos.y + layout.height)
  }
  return { width: maxX + ORIGIN_X * 2, height: maxY + ORIGIN_Y * 2 }
}

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
    const section = sections.get(key) ?? { calcFieldCount: 0, relatedTables: [] }

    // All columns are listed (PK/FK/join columns first); lists longer than
    // colLimit scroll inside a fixed window instead of being truncated.
    const cols = [...columnOptions(columns, key)].sort((a, b) => compareColumns(a, b, linked))
    const visibleRowCount = Math.min(cols.length, colLimit)

    const idx = new Map<string, number>()
    cols.forEach((c, i) => idx.set(c.column_name, i))
    out.set(key, {
      columnsShown: cols,
      columnIndex: idx,
      height: cardHeight(visibleRowCount, section),
      visibleRowCount,
      calcFieldCount: section.calcFieldCount,
      relatedTables: section.relatedTables,
    })
  }
  return out
}

export function buildCardSections(
  tableCards: TableRow[],
  model: SemanticModelDetail | null,
): Map<string, CardSection> {
  const sections = new Map<string, CardSection>()
  const baseSchema = model?.base_schema ?? ''
  const activeJoins = (model?.joins ?? []).filter((join) => join.is_active !== false)
  const calculatedDimensions = (model?.dimensions ?? []).filter(
    (dimension) =>
      dimension.is_active !== false &&
      Boolean(dimension.calculated_expression?.trim() ?? dimension.calculated_expr),
  )
  const metrics = (model?.metrics ?? []).filter((metric) => metric.is_active !== false)

  for (const table of tableCards) {
    const key = tableKey(table.schema_name, table.table_name)
    const relatedTables: string[] = []
    for (const join of activeJoins) {
      const fromKey = tableKey(join.from_schema ?? baseSchema, join.from_table)
      const toKey = tableKey(join.to_schema ?? baseSchema, join.to_table)
      if (fromKey === key) {
        relatedTables.push(join.to_table)
      } else if (toKey === key) {
        relatedTables.push(join.from_table)
      }
    }

    let calcFieldCount = 0
    for (const dimension of calculatedDimensions) {
      if (
        columnRefMatchesTable(dimension.column_ref, table.schema_name, table.table_name, baseSchema)
      ) {
        calcFieldCount++
      }
    }
    for (const metric of metrics) {
      if (
        columnRefMatchesTable(metric.expression, table.schema_name, table.table_name, baseSchema)
      ) {
        calcFieldCount++
      }
    }

    sections.set(key, { calcFieldCount, relatedTables })
  }

  return sections
}

// Y offset (within a card) of a relationship row's center. Prefers the
// DOM-measured offsets (exact regardless of CSS rounding); the constant-based
// fallback covers the first paint before measurement lands.
export function relRowCenterY(layout: CardLayout, relIdx: number, measured?: number[]) {
  const measuredY = measured?.[relIdx]
  if (measuredY !== undefined) {
    return measuredY
  }
  return (
    HEADER_HEIGHT +
    CARD_PAD_Y * 2 +
    layout.visibleRowCount * ROW_HEIGHT +
    CALC_SECTION_HEIGHT +
    REL_SECTION_LABEL_HEIGHT +
    relIdx * ROW_HEIGHT +
    ROW_HEIGHT / 2
  )
}

// Maps each active join to the index of the relationship row it occupies on
// its from- and to-cards. Must mirror buildCardSections' push order exactly:
// each card's relationship rows are the active joins touching it, in join
// order; a self-join occupies a single row.
export function buildJoinRelIndexes(
  joins: SemanticJoin[],
  baseSchema: string,
): Map<string, { from: number; to: number }> {
  const counts = new Map<string, number>()
  const out = new Map<string, { from: number; to: number }>()
  for (const join of joins) {
    if (join.is_active === false) {
      continue
    }
    const fromKey = tableKey(join.from_schema ?? baseSchema, join.from_table)
    const toKey = tableKey(join.to_schema ?? baseSchema, join.to_table)
    const fromIdx = counts.get(fromKey) ?? 0
    counts.set(fromKey, fromIdx + 1)
    let toIdx = fromIdx
    if (toKey !== fromKey) {
      toIdx = counts.get(toKey) ?? 0
      counts.set(toKey, toIdx + 1)
    }
    out.set(join.id, { from: fromIdx, to: toIdx })
  }
  return out
}

// cardinalityMarkers maps a relationship to the crow's-foot style end labels
// drawn next to the join line's endpoints: '*' on the many side, '1' on the
// one side (from → x1/y1, to → x2/y2).
export function cardinalityMarkers(relationship: SemanticJoin['relationship']): {
  from: string
  to: string
} {
  switch (relationship) {
    case 'many_to_one':
      return { from: '*', to: '1' }
    case 'one_to_many':
      return { from: '1', to: '*' }
    case 'many_to_many':
      return { from: '*', to: '*' }
    default:
      return { from: '1', to: '1' }
  }
}

// Anchor at the cards' relationship rows (always visible, unaffected by the
// scrollable column list); fall back to the join-column rows when the join
// has no relationship-row index. Returns null when the fallback's join
// column isn't present in the card's column set.
function computeJoinAnchorY(
  join: SemanticJoin,
  fromKey: string,
  toKey: string,
  fromPos: Pt,
  toPos: Pt,
  fromLayout: CardLayout,
  toLayout: CardLayout,
  scrollTops?: Map<string, number>,
  relIndexes?: Map<string, { from: number; to: number }>,
  relRowOffsets?: Map<string, number[]>,
): { fromY: number; toY: number } | null {
  const relIdx = relIndexes?.get(join.id)
  if (relIdx) {
    return {
      fromY: fromPos.y + relRowCenterY(fromLayout, relIdx.from, relRowOffsets?.get(fromKey)),
      toY: toPos.y + relRowCenterY(toLayout, relIdx.to, relRowOffsets?.get(toKey)),
    }
  }
  const fromIdx = fromLayout.columnIndex.get(join.from_column)
  const toIdx = toLayout.columnIndex.get(join.to_column)
  if (fromIdx === undefined || toIdx === undefined) {
    return null
  }
  return {
    fromY: fromPos.y + rowCenterY(fromIdx, fromLayout, scrollTops?.get(fromKey) ?? 0),
    toY: toPos.y + rowCenterY(toIdx, toLayout, scrollTops?.get(toKey) ?? 0),
  }
}

// Builds the orthogonal elbow path between two anchor points: a side lane
// when the cards sit roughly in the same column (both edges route right),
// otherwise a stubbed elbow with a shared midpoint X between the two cards.
function elbowJoinPath(fromPos: Pt, toPos: Pt, fromY: number, toY: number): JoinPath {
  const fromCenterX = fromPos.x + CARD_WIDTH / 2
  const toCenterX = toPos.x + CARD_WIDTH / 2
  const sameColumn = Math.abs(fromCenterX - toCenterX) < CARD_WIDTH * 0.65
  if (sameColumn) {
    const lane = Math.max(fromPos.x, toPos.x) + CARD_WIDTH + 28
    const x1 = fromPos.x + CARD_WIDTH
    const x2 = toPos.x + CARD_WIDTH
    const d = `M ${x1} ${fromY} L ${lane} ${fromY} L ${lane} ${toY} L ${x2} ${toY}`
    return { x1, y1: fromY, x2, y2: toY, d }
  }

  const fromLeft = fromCenterX > toCenterX
  const x1 = fromLeft ? fromPos.x : fromPos.x + CARD_WIDTH
  const x2 = fromLeft ? toPos.x + CARD_WIDTH : toPos.x

  const stub = 18
  const sx = fromLeft ? x1 - stub : x1 + stub
  const tx = fromLeft ? x2 + stub : x2 - stub
  const midX = (sx + tx) / 2
  const d = `M ${x1} ${fromY} L ${sx} ${fromY} L ${midX} ${fromY} L ${midX} ${toY} L ${tx} ${toY} L ${x2} ${toY}`
  return { x1, y1: fromY, x2, y2: toY, d }
}

export function computeJoinPath(
  join: SemanticJoin,
  baseSchema: string,
  positions: Record<string, Pt>,
  cardLayouts: Map<string, CardLayout>,
  scrollTops?: Map<string, number>,
  relIndexes?: Map<string, { from: number; to: number }>,
  relRowOffsets?: Map<string, number[]>,
): JoinPath | null {
  const fromKey = tableKey(join.from_schema ?? baseSchema, join.from_table)
  const toKey = tableKey(join.to_schema ?? baseSchema, join.to_table)
  const fromPos = positions[fromKey]
  const toPos = positions[toKey]
  const fromLayout = cardLayouts.get(fromKey)
  const toLayout = cardLayouts.get(toKey)
  if (!fromPos || !toPos || !fromLayout || !toLayout) {
    return null
  }

  const anchors = computeJoinAnchorY(
    join,
    fromKey,
    toKey,
    fromPos,
    toPos,
    fromLayout,
    toLayout,
    scrollTops,
    relIndexes,
    relRowOffsets,
  )
  if (!anchors) {
    return null
  }
  return elbowJoinPath(fromPos, toPos, anchors.fromY, anchors.toY)
}
