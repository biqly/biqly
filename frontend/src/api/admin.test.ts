import { afterEach, describe, expect, it, vi } from 'vitest'
import { listAuditLog } from './admin'

describe('admin API', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists audit log entries with filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify({
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
      })),
    })
    vi.stubGlobal('fetch', fetchMock)

    const entries = await listAuditLog('token-1', {
      userID: 'user-1',
      action: 'login',
      limit: 25,
    })

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/admin/audit-log?user_id=user-1&action=login&limit=25',
      { credentials: 'same-origin', headers: { Authorization: 'Bearer token-1' } },
    )
    expect(entries).toEqual([
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
    ])
  })
})
