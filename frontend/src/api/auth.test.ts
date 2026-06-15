import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TokenResponse } from '../types/auth'
import { apiFetch } from './apiClient'
import { apiRefresh, registerResponseHasSession } from './auth'

vi.mock('./apiClient', () => ({
  apiFetch: vi.fn(),
}))

const apiFetchMock = vi.mocked(apiFetch)

afterEach(() => {
  vi.clearAllMocks()
})

describe('registerResponseHasSession', () => {
  it('treats anti-enumeration verification responses as unauthenticated', () => {
    expect(registerResponseHasSession({ verification_pending: true })).toBe(false)
  })

  it('accepts token-bearing register responses as authenticated sessions', () => {
    expect(registerResponseHasSession({ access_token: 'token', roles: [] })).toBe(true)
  })
})

describe('apiRefresh', () => {
  it('coalesces concurrent cookie-backed refresh calls', async () => {
    const response: TokenResponse = {
      access_token: 'access',
      user_id: 'user-1',
      email: 'user@example.com',
      roles: [],
    }
    apiFetchMock.mockResolvedValue(response)

    await expect(Promise.all([apiRefresh(), apiRefresh()])).resolves.toEqual([response, response])

    expect(apiFetchMock).toHaveBeenCalledTimes(1)
    expect(apiFetchMock).toHaveBeenCalledWith('POST', '/api/auth/refresh', {})
  })

  it('does not coalesce explicit legacy refresh-token calls', async () => {
    const response: TokenResponse = {
      access_token: 'access',
      user_id: 'user-1',
      email: 'user@example.com',
      roles: [],
    }
    apiFetchMock.mockResolvedValue(response)

    await expect(Promise.all([apiRefresh('legacy'), apiRefresh('legacy')])).resolves.toEqual([
      response,
      response,
    ])

    expect(apiFetchMock).toHaveBeenCalledTimes(2)
    expect(apiFetchMock).toHaveBeenNthCalledWith(1, 'POST', '/api/auth/refresh', {
      refresh_token: 'legacy',
    })
    expect(apiFetchMock).toHaveBeenNthCalledWith(2, 'POST', '/api/auth/refresh', {
      refresh_token: 'legacy',
    })
  })
})
