import { describe, expect, it } from 'vitest'
import type { SemanticDimension } from '../../types/semantic'
import { buildQueryPayload } from './logicalQuery'
import {
  addFilterRow,
  addGroupByRow,
  addHavingRow,
  defaultFilterRow,
  patchFilterRow,
  patchHavingRow,
  removeFilterRow,
  removeGroupByRow,
  removeHavingRow,
  updateGroupByRow,
} from './rowState'
import type { QueryBuilderFormState } from './types'
import { dimOptionsForGroupRow, parseCTEBody } from './utils'

const dimensions: SemanticDimension[] = [
  { id: 'd1', name: 'region', column_ref: 'orders.region', type: 'text' },
  { id: 'd2', name: 'order_date', column_ref: 'orders.order_date', type: 'date' },
  { id: 'd3', name: 'status', column_ref: 'orders.status', type: 'text' },
]

const baseState = (): QueryBuilderFormState => ({
  datasourceId: 'ds-1',
  modelId: 'model-1',
  mode: 'simple',
  selectItems: [{ id: 's1', type: 'dimension', name: 'region' }],
  filters: [],
  groupBy: [],
  having: [],
  orderBy: '',
  orderDir: 'asc',
  limit: 100,
  offset: 0,
  windowFunctions: [],
  ctes: [],
})

describe('filter row state', () => {
  it('adds, updates, and removes filter rows', () => {
    let filters = addFilterRow([])
    expect(filters).toHaveLength(1)
    expect(filters[0]).toMatchObject({ field: '', operator: 'eq', value: '' })

    const rowId = filters[0]!.id
    filters = [patchFilterRow(filters[0], 'field', 'region')]
    filters = [patchFilterRow(filters[0], 'operator', 'contains')]
    filters = [patchFilterRow(filters[0], 'value', 'EMEA')]
    expect(filters[0]).toEqual({
      id: rowId,
      field: 'region',
      operator: 'contains',
      value: 'EMEA',
    })

    filters = addFilterRow(filters)
    expect(filters).toHaveLength(2)
    filters = removeFilterRow(filters, 0)
    expect(filters).toHaveLength(1)
    expect(filters[0]?.field).toBe('')
  })
})

describe('groupBy row state', () => {
  it('adds, updates, and removes groupBy rows', () => {
    let groupBy = addGroupByRow([])
    groupBy = addGroupByRow(groupBy)
    expect(groupBy).toEqual(['', ''])

    groupBy = updateGroupByRow(groupBy, 0, 'region')
    groupBy = updateGroupByRow(groupBy, 1, 'order_date')
    expect(groupBy).toEqual(['region', 'order_date'])

    groupBy = removeGroupByRow(groupBy, 0)
    expect(groupBy).toEqual(['order_date'])
  })

  it('excludes dimensions already chosen in other rows', () => {
    const optionsRow0 = dimOptionsForGroupRow(dimensions, ['region', ''], 0)
    const optionsRow1 = dimOptionsForGroupRow(dimensions, ['region', ''], 1)

    expect(optionsRow0.map((o) => o.value)).toContain('region')
    expect(optionsRow1.map((o) => o.value)).not.toContain('region')
    expect(optionsRow1.map((o) => o.value)).toContain('order_date')
  })
})

describe('having row state', () => {
  it('adds and removes having rows', () => {
    let having = addHavingRow([])
    having = [patchHavingRow(having[0], 'field', 'total_revenue')]
    having = [patchHavingRow(having[0], 'operator', 'gt')]
    having = [patchHavingRow(having[0], 'value', '1000')]
    expect(having[0]).toEqual({ field: 'total_revenue', operator: 'gt', value: '1000' })

    having = addHavingRow(having)
    expect(having).toHaveLength(2)
    having = removeHavingRow(having, 1)
    expect(having).toHaveLength(1)
  })
})

describe('buildQueryPayload', () => {
  it('maps complete simple-mode form state to API payload', () => {
    const payload = buildQueryPayload({
      ...baseState(),
      filters: [
        defaultFilterRow('f1'),
        { id: 'f2', field: 'region', operator: 'eq', value: 'EMEA' },
      ],
      groupBy: ['region', ''],
      orderBy: 'region',
      orderDir: 'desc',
      limit: 50,
    })

    expect(payload).toEqual({
      datasource_id: 'ds-1',
      model_id: 'model-1',
      filters: [{ field: 'region', operator: 'eq', value: 'EMEA' }],
      group_by: [{ field: 'region' }],
      having: undefined,
      order_by: [{ field: 'region', direction: 'desc' }],
      limit: 50,
      offset: undefined,
      select: [{ type: 'dimension', name: 'region' }],
      ctes: undefined,
    })
  })

  it('includes having, offset, window functions, and CTEs in advanced mode', () => {
    const payload = buildQueryPayload({
      ...baseState(),
      mode: 'advanced',
      selectItems: [
        { id: 's1', type: 'metric', name: 'total_revenue' },
        { id: 's2', type: 'dimension', name: '' },
      ],
      filters: [{ id: 'f1', field: 'status', operator: 'eq', value: 'shipped' }],
      groupBy: ['region'],
      having: [{ field: 'total_revenue', operator: 'gt', value: '500' }],
      orderBy: 'total_revenue',
      orderDir: 'desc',
      limit: 200,
      offset: 10,
      windowFunctions: [
        {
          func: 'ROW_NUMBER',
          field: 'total_revenue',
          partition_by: 'region, status',
          order_by: 'order_date',
        },
      ],
      ctes: [
        {
          name: 'base',
          query: JSON.stringify({ select: [{ type: 'dimension', name: 'region' }], limit: 10 }),
        },
        { name: '', query: '{}' },
      ],
    })

    expect(payload.offset).toBe(10)
    expect(payload.having).toEqual([{ field: 'total_revenue', operator: 'gt', value: '500' }])
    expect(payload.select).toEqual([
      { type: 'metric', name: 'total_revenue' },
      {
        type: 'window',
        name: 'total_revenue',
        window: {
          aggregation: 'row_number',
          expression: 'total_revenue',
          partition_by: ['region', 'status'],
          order_by: [{ field: 'order_date', direction: 'asc' }],
        },
      },
    ])
    expect(payload.ctes).toEqual([
      {
        name: 'base',
        select: [{ type: 'dimension', name: 'region' }],
        limit: 10,
      },
    ])
  })

  it('simulates add/remove filter and groupBy before building payload', () => {
    let filters = addFilterRow([])
    filters = [patchFilterRow(filters[0], 'field', 'region')]
    filters = [patchFilterRow(filters[0], 'value', 'APAC')]
    filters = addFilterRow(filters)
    filters = removeFilterRow(filters, 1)

    let groupBy = addGroupByRow([])
    groupBy = updateGroupByRow(groupBy, 0, 'region')

    const payload = buildQueryPayload({
      ...baseState(),
      filters,
      groupBy,
    })

    expect(payload.filters).toEqual([{ field: 'region', operator: 'eq', value: 'APAC' }])
    expect(payload.group_by).toEqual([{ field: 'region' }])
  })
})

describe('parseCTEBody', () => {
  it('parses JSON CTE body and strips name', () => {
    expect(parseCTEBody('{"name":"ignored","limit":5}')).toEqual({ limit: 5 })
    expect(parseCTEBody('')).toEqual({})
    expect(parseCTEBody('not-json')).toEqual({})
  })
})
