import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiFetch } from './apiClient'
import { compileQuery, dryRunQuery } from './query'

vi.mock('./apiClient', () => ({ apiFetch: vi.fn() }))

const apiFetchMock = vi.mocked(apiFetch)

describe('query API client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('posts a logical query to compile and returns the compiled SQL', async () => {
    apiFetchMock.mockResolvedValue({ sql: 'SELECT 1', args: [] })

    await expect(
      compileQuery({ datasource_id: 'ds-1', model_id: 'model-1', select: [] }),
    ).resolves.toEqual({
      sql: 'SELECT 1',
      args: [],
    })

    expect(apiFetchMock).toHaveBeenCalledWith('POST', '/api/query/compile', {
      datasource_id: 'ds-1',
      model_id: 'model-1',
      select: [],
    })
  })

  it('posts a logical query to dry-run and returns its validation fingerprint', async () => {
    apiFetchMock.mockResolvedValue({ sql: 'SELECT 1', args: [], fingerprint: 'fp-1' })

    await expect(
      dryRunQuery({ datasource_id: 'ds-1', model_id: 'model-1', select: [] }),
    ).resolves.toEqual({
      sql: 'SELECT 1',
      args: [],
      fingerprint: 'fp-1',
    })

    expect(apiFetchMock).toHaveBeenCalledWith('POST', '/api/query/dry-run', {
      datasource_id: 'ds-1',
      model_id: 'model-1',
      select: [],
    })
  })
})
