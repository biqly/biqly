import { describe, expect, it } from 'vitest'
import type { ColumnRow } from '../../types/semantic'
import {
  applyDragDelta,
  applyKeyboardMove,
  buildCardLayouts,
  computeCanvasBounds,
  computeJoinPath,
  keyboardDeltaFromKey,
  layoutInitialPositions,
  panViewport,
  zoomStep,
  zoomViewportAtPoint,
} from './canvasMath'
import type { CardLayout } from './types'
import { tableKey } from './utils'

const columns: ColumnRow[] = [
  {
    id: 'c1',
    schema_name: 'public',
    table_name: 'orders',
    column_name: 'customer_id',
    data_type: 'integer',
    nullable: false,
    description: null,
    is_primary_key: false,
    is_foreign_key: true,
    referenced_table: 'customers',
    referenced_column: 'id',
  },
  {
    id: 'c2',
    schema_name: 'public',
    table_name: 'customers',
    column_name: 'id',
    data_type: 'integer',
    nullable: false,
    description: null,
    is_primary_key: true,
    is_foreign_key: false,
    referenced_table: null,
    referenced_column: null,
  },
]

describe('canvas drag-and-drop math', () => {
  it('scales mouse delta by viewport zoom', () => {
    expect(applyDragDelta({ x: 100, y: 50 }, 20, 10, 2)).toEqual({ x: 110, y: 55 })
  })

  it('clamps card positions at zero', () => {
    expect(applyDragDelta({ x: 4, y: 2 }, -40, -20, 1)).toEqual({ x: 0, y: 0 })
  })

  it('moves cards with arrow keys and shift step', () => {
    expect(applyKeyboardMove({ x: 10, y: 20 }, 1, 0, false)).toEqual({ x: 18, y: 20 })
    expect(applyKeyboardMove({ x: 10, y: 20 }, 0, -1, true)).toEqual({ x: 10, y: 0 })
  })

  it('maps arrow keys to direction deltas', () => {
    expect(keyboardDeltaFromKey('ArrowLeft')).toEqual({ dx: -1, dy: 0 })
    expect(keyboardDeltaFromKey('Enter')).toBeNull()
  })

  it('pans the viewport with canvas drag', () => {
    expect(panViewport({ scale: 1, tx: 0, ty: 0 }, 12, -8)).toEqual({ scale: 1, tx: 12, ty: -8 })
  })
})

describe('canvas layout', () => {
  it('lays out cards in a grid', () => {
    const tables = [
      { id: 't1', schema_name: 'public', table_name: 'orders', table_type: 'BASE TABLE', description: null },
      { id: 't2', schema_name: 'public', table_name: 'customers', table_type: 'BASE TABLE', description: null },
    ]
    const joinColumns = new Map<string, Set<string>>()
    const layouts = buildCardLayouts(tables, columns, joinColumns, 10)
    const positions = layoutInitialPositions(tables, layouts)
    expect(positions[tableKey('public', 'orders')]).toEqual({ x: 40, y: 40 })
    expect(positions[tableKey('public', 'customers')]!.x).toBeGreaterThan(positions[tableKey('public', 'orders')]!.x)
  })

  it('draws join paths between compatible columns', () => {
    const layouts = new Map<string, CardLayout>([
      [
        tableKey('public', 'orders'),
        {
          columnsShown: [columns[0]!],
          columnIndex: new Map([['customer_id', 0]]),
          height: 120,
          hiddenCount: 0,
        },
      ],
      [
        tableKey('public', 'customers'),
        {
          columnsShown: [columns[1]!],
          columnIndex: new Map([['id', 0]]),
          height: 120,
          hiddenCount: 0,
        },
      ],
    ])
    const positions = {
      [tableKey('public', 'orders')]: { x: 40, y: 40 },
      [tableKey('public', 'customers')]: { x: 420, y: 40 },
    }
    const path = computeJoinPath(
      {
        id: 'j1',
        name: 'orders_customer',
        from_table: 'orders',
        from_column: 'customer_id',
        to_table: 'customers',
        to_column: 'id',
        join_type: 'LEFT',
        relationship: 'many_to_one',
      },
      'public',
      positions,
      layouts,
    )
    expect(path).not.toBeNull()
    expect(path!.d).toContain('M ')
  })
})

describe('viewport zoom', () => {
  it('steps through discrete zoom levels', () => {
    expect(zoomStep(1, 1)).toBe(1.1)
    expect(zoomStep(1, -1)).toBe(0.9)
  })

  it('keeps the cursor anchor while zooming', () => {
    const next = zoomViewportAtPoint({ scale: 1, tx: 0, ty: 0 }, 100, 100, 2)
    expect(next.scale).toBe(2)
    expect(next.tx).toBe(-100)
    expect(next.ty).toBe(-100)
  })

  it('computes canvas bounds from card positions', () => {
    const tables = [{ id: 't1', schema_name: 'public', table_name: 'orders', table_type: 'BASE TABLE', description: null }]
    const layouts = buildCardLayouts(tables, columns.slice(0, 1), new Map(), 10)
    const positions = layoutInitialPositions(tables, layouts)
    const bounds = computeCanvasBounds(tables, positions, layouts)
    expect(bounds.width).toBeGreaterThan(300)
    expect(bounds.height).toBeGreaterThan(100)
  })
})
