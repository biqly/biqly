import { describe, expect, it } from 'vitest'

import type { ColumnRow, TableRow } from '../../types/semantic'
import { publishModelRequest, suggestedJoinToPayload } from './types'
import {
  buildJoinPayload,
  canSaveJoinForm,
  columnsAreJoinCompatible,
  defaultJoinForm,
  joinName,
  patchJoinForm,
} from './utils'

const tables: TableRow[] = [
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
    data_type: 'bigint',
    nullable: false,
    description: null,
    is_primary_key: true,
    is_foreign_key: false,
    referenced_table: null,
    referenced_column: null,
  },
]

const model = {
  id: 'm1',
  datasource_id: 'ds1',
  name: 'orders_model',
  base_schema: 'public',
  base_table: 'orders',
  status: 'draft',
  dimensions: [],
  metrics: [],
  joins: [],
}

describe('join creation helpers', () => {
  it('builds a default join form from model base table', () => {
    const form = defaultJoinForm(tables, columns, model)
    expect(form.fromTable).toBe('public.orders')
    expect(form.toTable).toBe('public.customers')
    expect(form.fromColumn).toBe('customer_id')
    expect(form.toColumn).toBe('id')
  })

  it('treats integer and bigint columns as compatible', () => {
    expect(columnsAreJoinCompatible(columns[0], columns[1])).toBe(true)
  })

  it('generates stable join names', () => {
    expect(
      joinName({
        fromTable: 'public.orders',
        fromColumn: 'customer_id',
        toTable: 'public.customers',
        toColumn: 'id',
        joinType: 'LEFT',
        relationship: 'many_to_one',
      }),
    ).toBe('orders_customer_id_to_customers_id')
  })

  it('builds API payloads for manual joins', () => {
    const payload = buildJoinPayload({
      fromTable: 'public.orders',
      fromColumn: 'customer_id',
      toTable: 'public.customers',
      toColumn: 'id',
      joinType: 'LEFT',
      relationship: 'many_to_one',
    })
    expect(payload).toEqual({
      name: 'orders_customer_id_to_customers_id',
      from_schema: 'public',
      from_table: 'orders',
      from_column: 'customer_id',
      to_schema: 'public',
      to_table: 'customers',
      to_column: 'id',
      join_type: 'LEFT',
      relationship: 'many_to_one',
    })
  })

  it('validates join forms before save', () => {
    const form = defaultJoinForm(tables, columns, model)
    expect(canSaveJoinForm(model, form, columns)).toBe(true)
    expect(canSaveJoinForm(null, form, columns)).toBe(false)
  })

  it('re-selects compatible target columns when source changes', () => {
    const next = patchJoinForm(
      {
        fromTable: 'public.orders',
        fromColumn: 'customer_id',
        toTable: 'public.customers',
        toColumn: 'id',
        joinType: 'LEFT',
        relationship: 'many_to_one',
      },
      { fromColumn: 'customer_id', toTable: 'public.orders' },
      columns,
    )
    expect(next.toTable).toBe('public.orders')
    expect(next.toColumn).toBe('customer_id')
  })

  it('maps suggested FK joins to API payloads', () => {
    expect(
      suggestedJoinToPayload({
        name: 'fk_orders_customers',
        from_schema: 'public',
        from_table: 'orders',
        from_column: 'customer_id',
        to_schema: 'public',
        to_table: 'customers',
        to_column: 'id',
      }),
    ).toMatchObject({
      name: 'fk_orders_customers',
      join_type: 'LEFT',
      relationship: 'many_to_one',
    })
  })
})

describe('model publish flow', () => {
  it('builds the publish endpoint request', () => {
    expect(publishModelRequest('model-123')).toEqual({
      url: '/api/semantic/models/model-123/publish',
      body: { published_by: 'modeling-ui' },
    })
  })
})
