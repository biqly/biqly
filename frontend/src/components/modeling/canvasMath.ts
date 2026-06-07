import type { ColumnRow, SemanticJoin, TableRow } from '../../types/semantic'
import {
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
  ROW_HEIGHT,
  ZOOM_STEPS,
} from './constants'
import type { CardLayout, JoinPath, Pt, Viewport } from './types'
import { columnOptions, compareColumns, tableKey } from './utils'

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

export const cardHeight = (count: number) => HEADER_HEIGHT + count * ROW_HEIGHT + CARD_PAD_Y * 2
export const rowCenterY = (idx: number) =>
  HEADER_HEIGHT + CARD_PAD_Y + idx * ROW_HEIGHT + ROW_HEIGHT / 2

export function applyDragDelta(startPos: Pt, dx: number, dy: number, scale: number): Pt {
  return {
    x: Math.max(0, startPos.x + dx / scale),
    y: Math.max(0, startPos.y + dy / scale),
  }
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
    out.set(key, {
      columnsShown: cols,
      columnIndex: idx,
      height: cardHeight(rowCount),
      hiddenCount: hidden,
    })
  }
  return out
}

export function computeJoinPath(
  join: SemanticJoin,
  baseSchema: string,
  positions: Record<string, Pt>,
  cardLayouts: Map<string, CardLayout>,
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

  const fromIdx = fromLayout.columnIndex.get(join.from_column)
  const toIdx = toLayout.columnIndex.get(join.to_column)
  if (fromIdx === undefined || toIdx === undefined) {
    return null
  }
  const fromY = fromPos.y + rowCenterY(fromIdx)
  const toY = toPos.y + rowCenterY(toIdx)

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
