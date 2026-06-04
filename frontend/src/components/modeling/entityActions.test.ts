import { describe, expect, it } from 'vitest'

import {
  activeEntities,
  inactiveEntities,
  reactivateDimensionPayload,
  reactivateJoinPayload,
  reactivateMetricPayload,
  renameDimensionPayload,
  renameMetricPayload,
} from './entityActions'

describe('modeling entity actions', () => {
  const entities = [
    { id: 'active-default' },
    { id: 'active-explicit', is_active: true },
    { id: 'inactive', is_active: false },
  ]

  it('keeps entities active unless explicitly deactivated', () => {
    expect(activeEntities(entities).map((entity) => entity.id)).toEqual([
      'active-default',
      'active-explicit',
    ])
  })

  it('returns only explicitly deactivated entities as inactive', () => {
    expect(inactiveEntities(entities).map((entity) => entity.id)).toEqual(['inactive'])
  })

  it('builds complete reactivation payloads for joins', () => {
    expect(
      reactivateJoinPayload({
        id: 'join-1',
        name: 'orders_customers',
        from_table: 'orders',
        from_column: 'customer_id',
        to_table: 'customers',
        to_column: 'id',
        join_type: 'LEFT',
        relationship: 'many_to_one',
      }),
    ).toEqual({
      name: 'orders_customers',
      from_schema: '',
      from_table: 'orders',
      from_column: 'customer_id',
      to_schema: '',
      to_table: 'customers',
      to_column: 'id',
      join_type: 'LEFT',
      relationship: 'many_to_one',
      is_active: true,
    })
  })

  it('preserves dimension and metric fields when changing active state or label', () => {
    const dimension = {
      id: 'dim-1',
      name: 'city',
      column_ref: 'customers.city',
      type: 'text',
      calculated_expression: 'revenue - cost',
      calculated_expr: {
        type: 'binary' as const,
        op: 'subtract',
        left: { type: 'column_ref' as const, column: 'revenue' },
        right: { type: 'column_ref' as const, column: 'cost' },
      },
    }
    const metric = {
      id: 'metric-1',
      name: 'revenue',
      expression: 'orders.total',
      aggregation: 'sum',
      expr: { type: 'column_ref' as const, column: 'total' },
    }

    expect(reactivateDimensionPayload(dimension)).toMatchObject({
      label: '',
      synonyms: [],
      description: '',
      is_active: true,
      calculated_expression: 'revenue - cost',
      calculated_expr: dimension.calculated_expr,
    })
    expect(renameDimensionPayload(dimension, 'City')).toMatchObject({
      label: 'City',
      is_active: undefined,
      calculated_expression: 'revenue - cost',
      calculated_expr: dimension.calculated_expr,
    })
    expect(reactivateMetricPayload(metric)).toMatchObject({
      label: '',
      format: '',
      synonyms: [],
      description: '',
      is_active: true,
      expr: metric.expr,
    })
    expect(renameMetricPayload(metric, 'Revenue')).toMatchObject({
      label: 'Revenue',
      is_active: undefined,
      expr: metric.expr,
    })
  })
})
