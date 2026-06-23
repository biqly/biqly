import { describe, expect, it } from 'vitest'

import type { ColumnRow, SemanticJoin, TableRow } from '../../types/semantic'
import { buildMetadataModel, metadataTableKey } from './metadataModel'

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

function column(table_name: string, column_name: string, data_type: string): ColumnRow {
  return {
    id: `${table_name}.${column_name}`,
    schema_name: 'public',
    table_name,
    column_name,
    data_type,
    nullable: true,
    description: null,
    is_primary_key: column_name === 'id',
    is_foreign_key: false,
    referenced_table: null,
    referenced_column: null,
  }
}

describe('metadata query model', () => {
  it('builds a transient semantic model from selected metadata tables and joins', () => {
    const join: SemanticJoin = {
      id: 'j1',
      name: '',
      from_schema: 'public',
      from_table: 'orders',
      from_column: 'customer_id',
      to_schema: 'public',
      to_table: 'customers',
      to_column: 'id',
      join_type: 'LEFT',
      relationship: 'many_to_one',
    }

    const model = buildMetadataModel({
      datasourceId: 'ds-1',
      baseTableKey: metadataTableKey('public', 'orders'),
      tables,
      columnsByTable: {
        'public.orders': [
          column('orders', 'id', 'integer'),
          column('orders', 'total_amount', 'numeric'),
        ],
        'public.customers': [
          column('customers', 'id', 'integer'),
          column('customers', 'country', 'text'),
        ],
      },
      joins: [join],
    })

    expect(model?.base_table).toBe('orders')
    expect(model?.dimensions?.map((d) => d.column_ref)).toContain('public.customers.country')
    expect(model?.metrics?.map((m) => m.name)).toContain('sum__public__orders__total_amount')
    expect(model?.joins?.[0]).toMatchObject({
      name: 'orders_customer_id_customers_id',
      from_table: 'orders',
      to_table: 'customers',
    })
  })
})
