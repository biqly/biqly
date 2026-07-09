import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiFetch } from './apiClient'
import { getFunctionBlocklist, updateFunctionBlocklist } from './functionBlocklist'

vi.mock('./apiClient', () => ({ apiFetch: vi.fn() }))

const apiFetchMock = vi.mocked(apiFetch)

describe('function blocklist API client', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads the blocklist for the selected datasource', async () => {
    apiFetchMock.mockResolvedValue({ defaults: ['pg_read_file'], custom: ['unsafe_function'] })

    await expect(getFunctionBlocklist('warehouse/id')).resolves.toEqual({
      defaults: ['pg_read_file'],
      custom: ['unsafe_function'],
    })

    expect(apiFetchMock).toHaveBeenCalledWith(
      'GET',
      '/api/datasources/warehouse%2Fid/function-blocklist',
    )
  })

  it('saves only custom denied function names', async () => {
    apiFetchMock.mockResolvedValue({ defaults: ['pg_read_file'], custom: ['unsafe_function'] })

    await updateFunctionBlocklist('warehouse', ['unsafe_function'])

    expect(apiFetchMock).toHaveBeenCalledWith(
      'PUT',
      '/api/datasources/warehouse/function-blocklist',
      { custom: ['unsafe_function'] },
    )
  })
})
