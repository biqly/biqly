// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'

import { setGlobalAccessToken } from './apiClient'
import { importDbtProject } from './dbt'

describe('importDbtProject', () => {
  afterEach(() => {
    setGlobalAccessToken(null)
    vi.restoreAllMocks()
  })

  it('uploads dbt artifacts as multipart form data with the selected datasource', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'X-CSRF-Token': 'csrf-1' }),
      })
      .mockResolvedValueOnce({
        ok: true,
        headers: new Headers(),
        text: () => Promise.resolve('{"imported_models":[],"skipped":[],"warnings":[]}'),
      })
    vi.stubGlobal('fetch', fetchMock)
    setGlobalAccessToken('token-1')

    await importDbtProject({
      datasourceId: 'warehouse',
      manifest: new File(['{}'], 'manifest.json', { type: 'application/json' }),
      catalog: new File(['{}'], 'catalog.json', { type: 'application/json' }),
    })

    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/catalog/dbt/import?datasource_id=warehouse',
      expect.objectContaining({ credentials: 'same-origin', method: 'POST' }),
    )
    const [, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    expect(init.body).toBeInstanceOf(FormData)
    const body = init.body as FormData
    expect(body.get('manifest')).toBeInstanceOf(File)
    expect(body.get('catalog')).toBeInstanceOf(File)
    const headers = init.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer token-1')
    expect(headers.has('Content-Type')).toBe(false)
  })
})
