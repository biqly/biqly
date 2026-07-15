import { afterEach, describe, expect, it, vi } from 'vitest'

import { setGlobalAccessToken } from './apiClient'
import { getPublicDashboard, runPublicWidget } from './publicDashboard'

describe('public dashboard API', () => {
  afterEach(() => {
    setGlobalAccessToken(null)
    vi.restoreAllMocks()
  })

  it('fetches a public dashboard by token with no Authorization header, even when a global token is set', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      headers: new Headers(),
      text: () =>
        Promise.resolve(JSON.stringify({ id: 'dash-1', name: 'Dashboard 1', widgets: [] })),
    })
    vi.stubGlobal('fetch', fetchMock)
    setGlobalAccessToken('some-token')

    const result = await getPublicDashboard('tok-abc')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/public/dashboards/tok-abc',
      expect.objectContaining({ credentials: 'same-origin', method: 'GET' }),
    )
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = init.headers as Headers
    expect(headers.has('Authorization')).toBe(false)
    expect(result).toEqual({ id: 'dash-1', name: 'Dashboard 1', widgets: [] })
  })

  it('encodes the token in the URL', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      headers: new Headers(),
      text: () => Promise.resolve(JSON.stringify({ id: 'dash-1', name: 'D', widgets: [] })),
    })
    vi.stubGlobal('fetch', fetchMock)

    await getPublicDashboard('tok/with space')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/public/dashboards/tok%2Fwith%20space',
      expect.anything(),
    )
  })

  it('runs a public widget query with no Authorization header, even when a global token is set', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'X-CSRF-Token': 'csrf-1' }),
      })
      .mockResolvedValueOnce({
        ok: true,
        headers: new Headers(),
        text: () => Promise.resolve(JSON.stringify({ columns: [], rows: [] })),
      })
    vi.stubGlobal('fetch', fetchMock)
    setGlobalAccessToken('some-token')

    const result = await runPublicWidget('tok-abc', 'widget-1')

    expect(fetchMock).toHaveBeenLastCalledWith(
      '/api/public/widget-query/tok-abc/widget-1',
      expect.objectContaining({ credentials: 'same-origin', method: 'POST' }),
    )
    const [, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = init.headers as Headers
    expect(headers.has('Authorization')).toBe(false)
    expect(result).toEqual({ columns: [], rows: [] })
  })
})
