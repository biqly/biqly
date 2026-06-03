import { describe, expect, it } from 'vitest'

import { request } from './useApi'

describe('request', () => {
  it('returns a readable error from HTML error responses', async () => {
    const originalFetch = globalThis.fetch
    globalThis.fetch = () =>
      Promise.resolve(new Response('<strong>bad gateway</strong>', { status: 502 }))

    try {
      await expect(request('GET', '/api/test')).resolves.toEqual({
        data: null,
        error: 'HTTP 502: bad gateway',
      })
    } finally {
      globalThis.fetch = originalFetch
    }
  })

  it('rejects successful non-JSON responses', async () => {
    const originalFetch = globalThis.fetch
    globalThis.fetch = () => Promise.resolve(new Response('ok', { status: 200 }))

    try {
      await expect(request('GET', '/api/test')).resolves.toEqual({
        data: null,
        error: 'Expected JSON response from /api/test',
      })
    } finally {
      globalThis.fetch = originalFetch
    }
  })

  it('reports timeout aborts separately from caller aborts', async () => {
    const originalFetch = globalThis.fetch
    globalThis.fetch = (_url, init) =>
      new Promise((_resolve, reject) => {
        init?.signal?.addEventListener('abort', () =>
          reject(new DOMException('aborted', 'AbortError')),
        )
      })

    try {
      await expect(request('GET', '/api/slow', undefined, { timeout: 1 })).resolves.toEqual({
        data: null,
        error: 'Request timed out',
      })

      const controller = new AbortController()
      const pending = request('GET', '/api/slow', undefined, { signal: controller.signal })
      controller.abort()

      await expect(pending).resolves.toEqual({
        data: null,
        error: 'Request aborted',
      })
    } finally {
      globalThis.fetch = originalFetch
    }
  })
})
