import { describe, expect, it } from 'vitest'

import { buildDescribeBatchRequest } from './bulkDescribeRunner'

describe('buildDescribeBatchRequest', () => {
  it('preserves the selected metadata description locale for background jobs', () => {
    expect(
      buildDescribeBatchRequest({
        datasourceId: 'ds-1',
        targets: [
          { schema_name: 'public', table_name: 'orders', description: null },
          { schema_name: 'sales', table_name: 'customers', description: 'Existing' },
        ],
        sampleSize: 25,
        locale: 'tr',
        skipExisting: true,
      }),
    ).toEqual({
      datasource_id: 'ds-1',
      tables: [
        { schema: 'public', table: 'orders' },
        { schema: 'sales', table: 'customers' },
      ],
      locale: 'tr',
      sample_size: 25,
      auto_apply: true,
      skip_existing: true,
    })
  })
})
