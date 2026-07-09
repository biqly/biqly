import { describe, expect, it } from 'vitest'

import { dbtManifestModelNames } from './dbtManifestPreview'

describe('dbtManifestModelNames', () => {
  it('returns enabled dbt model aliases in alphabetical order', () => {
    expect(
      dbtManifestModelNames({
        nodes: {
          'model.project.orders': {
            resource_type: 'model',
            name: 'orders',
            alias: 'fct_orders',
          },
          'model.project.customers': {
            resource_type: 'model',
            name: 'customers',
            config: { enabled: true },
          },
          'model.project.disabled': {
            resource_type: 'model',
            name: 'disabled',
            config: { enabled: false },
          },
          'test.project.orders_not_null': {
            resource_type: 'test',
            name: 'orders_not_null',
          },
        },
      }),
    ).toEqual(['customers', 'fct_orders'])
  })

  it('returns no models for a malformed manifest shape', () => {
    expect(dbtManifestModelNames({ nodes: [] })).toEqual([])
  })
})
