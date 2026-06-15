import { afterEach, describe, expect, it, vi } from 'vitest'

import type { PasswordPolicy, TokenResponse } from '../types/auth'
import { apiFetch } from './apiClient'
import {
  apiRefresh,
  firstUserSetupRequiredFromPolicy,
  registerResponseHasSession,
  selfSignupEnabledFromPolicy,
} from './auth'

vi.mock('./apiClient', () => ({
  apiFetch: vi.fn(),
}))

const apiFetchMock = vi.mocked(apiFetch)
const basePolicy: PasswordPolicy = {
  min_length: 8,
  max_length: 128,
  require_upper: true,
  require_lower: true,
  require_digit: true,
  require_special: true,
  min_score: 2,
  self_signup_enabled: true,
}

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

describe('password policy helpers', () => {
  it('keeps signup available during first-user setup even when self signup is disabled', () => {
    expect(
      selfSignupEnabledFromPolicy({
        ...basePolicy,
        self_signup_enabled: false,
        first_user_setup_required: true,
      }),
    ).toBe(true)
  })

  it('defaults first-user setup to false when the backend omits the field', () => {
    expect(firstUserSetupRequiredFromPolicy(basePolicy)).toBe(false)
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
