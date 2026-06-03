import { describe, expect, it } from 'vitest'

import type { Datasource } from '../../types/metadata'
import { buildDatasourceAccessView } from './accessView'

const datasources: Datasource[] = [
  { id: 'ds-open', name: 'Orders', type: 'postgres' },
  { id: 'ds-locked', name: 'Finance', type: 'postgres' },
]

describe('buildDatasourceAccessView', () => {
  it('shows only accessible datasources when access ids are known', () => {
    expect(buildDatasourceAccessView(datasources, ['ds-open'])).toEqual([
      {
        datasource: datasources[0],
        access: 'allowed',
      },
    ])
  })

  it('keeps datasources visible while access ids are unavailable', () => {
    expect(buildDatasourceAccessView(datasources, null)).toEqual([
      { datasource: datasources[0], access: 'unknown' },
      { datasource: datasources[1], access: 'unknown' },
    ])
  })
})
