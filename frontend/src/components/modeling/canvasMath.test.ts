import { describe, expect, it } from 'vitest'

import type { ColumnRow, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  applyDragDelta,
  applyKeyboardMove,
  buildCardLayouts,
  buildCardSections,
  buildJoinRelIndexes,
  cardHeight,
  cardinalityMarkers,
  computeCanvasBounds,
  computeJoinPath,
  continuousZoomScale,
  exceedsDragThreshold,
  keyboardDeltaFromKey,
  layoutCompact,
  layoutHierarchic,
  layoutInitialPositions,
  panViewport,
  relRowCenterY,
  resolveOverlaps,
  zoomStep,
  zoomViewportAtPoint,
} from './canvasMath'
import {
  CALC_SECTION_HEIGHT,
  CARD_BOTTOM_PAD,
  CARD_PAD_Y,
  GRID_Y,
  HEADER_HEIGHT,
  MAX_SCALE,
  MIN_SCALE,
  REL_SECTION_LABEL_HEIGHT,
  ROW_HEIGHT,
} from './constants'
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
      {
        id: 'd2',
        name: 'rev',
        column_ref: '',
        type: 'number',
        calculated_expression: 'x',
        column_ref_calc: '',
      },
    ],
    metrics: [{ id: 'm1', name: 'total', expression: 'orders.amount', aggregation: 'sum' }],
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
      base + CALC_SECTION_HEIGHT + REL_SECTION_LABEL_HEIGHT + 2 * ROW_HEIGHT + CARD_BOTTOM_PAD,
    )
  })
})

describe('canvas drag-and-drop math', () => {
  it('scales mouse delta by viewport zoom', () => {
    expect(applyDragDelta({ x: 100, y: 50 }, 20, 10, 2)).toEqual({ x: 110, y: 55 })
  })

  it('clamps card positions at zero', () => {
    expect(applyDragDelta({ x: 4, y: 2 }, -40, -20, 1)).toEqual({ x: 0, y: 0 })
  })

  it('distinguishes clicks from card drags', () => {
    expect(exceedsDragThreshold(4, 4)).toBe(false)
    expect(exceedsDragThreshold(5, 0)).toBe(true)
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

describe('canvas layout', () => {
  it('lays out cards in a grid', () => {
    const tables = [
      {
        id: 't1',
        schema_name: 'public',
        table_name: 'orders',
        table_type: 'BASE TABLE',
        description: null,
      },
      {
        id: 't2',
        schema_name: 'public',
        table_name: 'customers',
        table_type: 'BASE TABLE',
        description: null,
      },
    ]
    const joinColumns = new Map<string, Set<string>>()
    const layouts = buildCardLayouts(tables, columns, joinColumns, 10, new Map())
    const positions = layoutInitialPositions(tables, layouts)
    expect(positions[tableKey('public', 'orders')]).toEqual({ x: 40, y: 40 })
    expect(positions[tableKey('public', 'customers')]!.x).toBeGreaterThan(
      positions[tableKey('public', 'orders')]!.x,
    )
  })

  it('draws join paths between compatible columns', () => {
    const layouts = new Map<string, CardLayout>([
      [
        tableKey('public', 'orders'),
        {
          columnsShown: [columns[0]!],
          columnIndex: new Map([['customer_id', 0]]),
          height: 120,
          visibleRowCount: 1,
          calcFieldCount: 0,
          relatedTables: [],
        },
      ],
      [
        tableKey('public', 'customers'),
        {
          columnsShown: [columns[1]!],
          columnIndex: new Map([['id', 0]]),
          height: 120,
          visibleRowCount: 1,
          calcFieldCount: 0,
          relatedTables: [],
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

describe('buildCardLayouts column window', () => {
  it('lists every column but clamps the window to the column limit', () => {
    const wide = mkTable('public', 'wide')
    const manyColumns: ColumnRow[] = Array.from({ length: 12 }, (_, i) => ({
      id: `w${i}`,
      schema_name: 'public',
      table_name: 'wide',
      column_name: `col_${String(i).padStart(2, '0')}`,
      data_type: 'text',
      nullable: true,
      description: null,
      is_primary_key: false,
      is_foreign_key: false,
      referenced_table: null,
      referenced_column: null,
    }))
    const layouts = buildCardLayouts([wide], manyColumns, new Map(), 10, new Map())
    const layout = layouts.get(tableKey('public', 'wide'))!
    expect(layout.columnsShown).toHaveLength(12)
    expect(layout.visibleRowCount).toBe(10)
    expect(layout.height).toBe(cardHeight(10, { calcFieldCount: 0, relatedTables: [] }))
  })
})

describe('computeJoinPath with a scrolled column list', () => {
  const layout = (colName: string): CardLayout => ({
    columnsShown: [],
    columnIndex: new Map([[colName, 0]]),
    height: 120,
    visibleRowCount: 1,
    calcFieldCount: 0,
    relatedTables: [],
  })
  const join = {
    id: 'j1',
    name: 'orders_customer',
    from_table: 'orders',
    from_column: 'customer_id',
    to_table: 'customers',
    to_column: 'id',
    join_type: 'LEFT',
    relationship: 'many_to_one',
  } as const
  const layouts = new Map<string, CardLayout>([
    [tableKey('public', 'orders'), layout('customer_id')],
    [tableKey('public', 'customers'), layout('id')],
  ])
  const positions = {
    [tableKey('public', 'orders')]: { x: 40, y: 40 },
    [tableKey('public', 'customers')]: { x: 420, y: 40 },
  }

  it('anchors to the row center when the list is not scrolled', () => {
    const path = computeJoinPath(join, 'public', positions, layouts, new Map())
    expect(path!.y1).toBeCloseTo(40 + HEADER_HEIGHT + CARD_PAD_Y + ROW_HEIGHT / 2, 5)
  })

  it('clamps the anchor to the list window when the row is scrolled out', () => {
    const scrollTops = new Map([[tableKey('public', 'orders'), 100]])
    const path = computeJoinPath(join, 'public', positions, layouts, scrollTops)
    expect(path!.y1).toBeCloseTo(40 + HEADER_HEIGHT, 5)
    // The unscrolled side is unaffected.
    expect(path!.y2).toBeCloseTo(40 + HEADER_HEIGHT + CARD_PAD_Y + ROW_HEIGHT / 2, 5)
  })
})

describe('relationship-row join anchors', () => {
  const layout = (colName: string, relatedTables: string[]): CardLayout => ({
    columnsShown: [],
    columnIndex: new Map([[colName, 0]]),
    height: 220,
    visibleRowCount: 3,
    calcFieldCount: 0,
    relatedTables,
  })
  const join = {
    id: 'j1',
    name: 'orders_customer',
    from_table: 'orders',
    from_column: 'customer_id',
    to_table: 'customers',
    to_column: 'id',
    join_type: 'LEFT',
    relationship: 'many_to_one',
  } as const
  const layouts = new Map<string, CardLayout>([
    [tableKey('public', 'orders'), layout('customer_id', ['customers'])],
    [tableKey('public', 'customers'), layout('id', ['orders'])],
  ])
  const positions = {
    [tableKey('public', 'orders')]: { x: 40, y: 40 },
    [tableKey('public', 'customers')]: { x: 420, y: 40 },
  }
  const relIndexes = buildJoinRelIndexes([join], 'public')

  it('buildJoinRelIndexes mirrors buildCardSections push order', () => {
    const second = { ...join, id: 'j2', from_table: 'orders', to_table: 'products' }
    const idx = buildJoinRelIndexes([join, second], 'public')
    expect(idx.get('j1')).toEqual({ from: 0, to: 0 })
    // orders' second join occupies its second relationship row; products' first.
    expect(idx.get('j2')).toEqual({ from: 1, to: 0 })
  })

  it('anchors at the relationship row using the constant fallback', () => {
    const path = computeJoinPath(join, 'public', positions, layouts, undefined, relIndexes)
    const expected =
      40 +
      HEADER_HEIGHT +
      CARD_PAD_Y * 2 +
      3 * ROW_HEIGHT +
      CALC_SECTION_HEIGHT +
      REL_SECTION_LABEL_HEIGHT +
      ROW_HEIGHT / 2
    expect(path!.y1).toBeCloseTo(expected, 5)
    expect(path!.y2).toBeCloseTo(expected, 5)
  })

  it('prefers DOM-measured relationship row offsets over constants', () => {
    const measured = new Map([[tableKey('public', 'orders'), [187.5]]])
    const path = computeJoinPath(
      join,
      'public',
      positions,
      layouts,
      undefined,
      relIndexes,
      measured,
    )
    expect(path!.y1).toBeCloseTo(40 + 187.5, 5)
    // Unmeasured side falls back to constants.
    expect(path!.y2).toBeCloseTo(
      40 + relRowCenterY(layouts.get(tableKey('public', 'customers'))!, 0),
      5,
    )
  })

  it('ignores scroll offsets when anchored to relationship rows', () => {
    const scrollTops = new Map([[tableKey('public', 'orders'), 100]])
    const unscrolled = computeJoinPath(join, 'public', positions, layouts, undefined, relIndexes)
    const scrolled = computeJoinPath(join, 'public', positions, layouts, scrollTops, relIndexes)
    expect(scrolled!.y1).toBeCloseTo(unscrolled!.y1, 5)
  })
})

describe('layout presets', () => {
  const layoutOf = (height: number): CardLayout => ({
    columnsShown: [],
    columnIndex: new Map(),
    height,
    visibleRowCount: 1,
    calcFieldCount: 0,
    relatedTables: [],
  })

  it('layoutCompact fills the shortest column first', () => {
    const tables = [
      mkTable('public', 'a'),
      mkTable('public', 'b'),
      mkTable('public', 'c'),
      mkTable('public', 'd'),
      mkTable('public', 'e'),
    ]
    const layouts = new Map<string, CardLayout>([
      [tableKey('public', 'a'), layoutOf(600)],
      [tableKey('public', 'b'), layoutOf(100)],
      [tableKey('public', 'c'), layoutOf(100)],
      [tableKey('public', 'd'), layoutOf(100)],
      [tableKey('public', 'e'), layoutOf(100)],
    ])
    const pos = layoutCompact(tables, layouts, 2)
    // 'a' (600) goes to column 0; the remaining short cards stack in column 1
    // until it grows past column 0's height.
    expect(pos[tableKey('public', 'a')]!.x).not.toBe(pos[tableKey('public', 'b')]!.x)
    expect(pos[tableKey('public', 'b')]!.x).toBe(pos[tableKey('public', 'c')]!.x)
    expect(pos[tableKey('public', 'c')]!.y).toBeGreaterThan(pos[tableKey('public', 'b')]!.y)
  })

  it('layoutHierarchic layers tables by join distance from the base table', () => {
    const tables = [mkTable('public', 'orders'), mkTable('public', 'customers')]
    const layouts = new Map<string, CardLayout>([
      [tableKey('public', 'orders'), layoutOf(200)],
      [tableKey('public', 'customers'), layoutOf(200)],
    ])
    const joins = [
      {
        id: 'j1',
        name: 'j',
        from_table: 'orders',
        from_column: 'customer_id',
        to_table: 'customers',
        to_column: 'id',
        join_type: 'LEFT',
        relationship: 'many_to_one',
      } as const,
    ]
    const pos = layoutHierarchic(
      tables,
      layouts,
      [...joins],
      'public',
      tableKey('public', 'orders'),
    )
    expect(pos[tableKey('public', 'orders')]!.x).toBeLessThan(
      pos[tableKey('public', 'customers')]!.x,
    )
  })

  it('layoutHierarchic parks unconnected tables in a trailing column', () => {
    const tables = [
      mkTable('public', 'orders'),
      mkTable('public', 'customers'),
      mkTable('public', 'lonely'),
    ]
    const layouts = new Map<string, CardLayout>([
      [tableKey('public', 'orders'), layoutOf(200)],
      [tableKey('public', 'customers'), layoutOf(200)],
      [tableKey('public', 'lonely'), layoutOf(200)],
    ])
    const joins = [
      {
        id: 'j1',
        name: 'j',
        from_table: 'orders',
        from_column: 'customer_id',
        to_table: 'customers',
        to_column: 'id',
        join_type: 'LEFT',
        relationship: 'many_to_one',
      } as const,
    ]
    const pos = layoutHierarchic(
      tables,
      layouts,
      [...joins],
      'public',
      tableKey('public', 'orders'),
    )
    expect(pos[tableKey('public', 'lonely')]!.x).toBeGreaterThan(
      pos[tableKey('public', 'customers')]!.x,
    )
  })
})

describe('cardinalityMarkers', () => {
  it('places * on the many side and 1 on the one side', () => {
    expect(cardinalityMarkers('many_to_one')).toEqual({ from: '*', to: '1' })
    expect(cardinalityMarkers('one_to_many')).toEqual({ from: '1', to: '*' })
    expect(cardinalityMarkers('one_to_one')).toEqual({ from: '1', to: '1' })
    expect(cardinalityMarkers('many_to_many')).toEqual({ from: '*', to: '*' })
  })
})

describe('resolveOverlaps', () => {
  const layoutWithHeight = (height: number): CardLayout => ({
    columnsShown: [],
    columnIndex: new Map(),
    height,
    visibleRowCount: 0,
    calcFieldCount: 0,
    relatedTables: [],
  })
  const layouts = new Map<string, CardLayout>([
    ['a', layoutWithHeight(200)],
    ['b', layoutWithHeight(150)],
    ['c', layoutWithHeight(100)],
  ])

  it('pushes an overlapping lower card below the upper card', () => {
    const resolved = resolveOverlaps({ a: { x: 40, y: 40 }, b: { x: 60, y: 120 } }, layouts)
    expect(resolved.a).toEqual({ x: 40, y: 40 })
    expect(resolved.b!.x).toBe(60)
    expect(resolved.b!.y).toBe(40 + 200 + GRID_Y / 2)
  })

  it('returns the input untouched when nothing overlaps', () => {
    const positions = { a: { x: 40, y: 40 }, b: { x: 400, y: 40 } }
    expect(resolveOverlaps(positions, layouts)).toBe(positions)
  })

  it('cascades pushes through stacked overlaps', () => {
    const resolved = resolveOverlaps(
      { a: { x: 40, y: 40 }, b: { x: 40, y: 100 }, c: { x: 40, y: 160 } },
      layouts,
    )
    expect(resolved.b!.y).toBeGreaterThanOrEqual(resolved.a!.y + 200)
    expect(resolved.c!.y).toBeGreaterThanOrEqual(resolved.b!.y + 150)
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
    const tables = [
      {
        id: 't1',
        schema_name: 'public',
        table_name: 'orders',
        table_type: 'BASE TABLE',
        description: null,
      },
    ]
    const layouts = buildCardLayouts(tables, columns.slice(0, 1), new Map(), 10, new Map())
    const positions = layoutInitialPositions(tables, layouts)
    const bounds = computeCanvasBounds(tables, positions, layouts)
    expect(bounds.width).toBeGreaterThan(300)
    expect(bounds.height).toBeGreaterThan(100)
  })
})
