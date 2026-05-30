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
    expect(reactivateJoinPayload({
      id: 'join-1',
      name: 'orders_customers',
      from_table: 'orders',
      from_column: 'customer_id',
      to_table: 'customers',
      to_column: 'id',
      join_type: 'LEFT',
      relationship: 'many_to_one',
    })).toEqual({
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
    const dimension = { id: 'dim-1', name: 'city', column_ref: 'customers.city', type: 'text' }
    const metric = { id: 'metric-1', name: 'revenue', expression: 'orders.total', aggregation: 'sum' }

    expect(reactivateDimensionPayload(dimension)).toMatchObject({ label: '', synonyms: [], description: '', is_active: true })
    expect(renameDimensionPayload(dimension, 'City')).toMatchObject({ label: 'City', is_active: undefined })
    expect(reactivateMetricPayload(metric)).toMatchObject({ label: '', format: '', synonyms: [], description: '', is_active: true })
    expect(renameMetricPayload(metric, 'Revenue')).toMatchObject({ label: 'Revenue', is_active: undefined })
  })
})
