import { afterEach, describe, expect, it, vi } from 'vitest'

import { listAuditLog } from './admin'

describe('admin API', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists audit log entries with filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: () =>
        Promise.resolve(
          JSON.stringify({
            entries: [
              {
                id: 'audit-1',
                user_id: 'user-1',
                action: 'login',
                resource: 'session',
                resource_id: 'session-1',
                metadata: { method: 'password' },
                ip_address: '192.0.2.1',
                created_at: '2026-05-25T10:00:00Z',
              },
            ],
          }),
        ),
    })
    vi.stubGlobal('fetch', fetchMock)

    const result = await listAuditLog('token-1', {
      userID: 'user-1',
      action: 'login',
      page: 2,
      pageSize: 10,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/admin/audit-log?user_id=user-1&action=login&page=2&page_size=10',
      expect.objectContaining({
        credentials: 'same-origin',
        method: 'GET',
        headers: expect.any(Headers),
      }),
    )
    const headers = fetchMock.mock.calls[0]![1]!.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer token-1')
    expect(result).toEqual({
      entries: [
        {
          id: 'audit-1',
          user_id: 'user-1',
          action: 'login',
          resource: 'session',
          resource_id: 'session-1',
          metadata: { method: 'password' },
          ip_address: '192.0.2.1',
          created_at: '2026-05-25T10:00:00Z',
        },
      ],
      total: 0,
    })
  })
})
