import { describe, expect, it } from 'vitest'

import { ApiError } from '../api/apiClient'
import type { TranslationKey } from '../i18n'
import { errorMessage, friendlyErrorMessage } from './error'

// Regression: ISSUE-003 — raw API error strings ("too_many_requests",
// "internal server error") leaked into user-facing banners and toasts.
// Found by /qa on 2026-07-03
// Report: .gstack/qa-reports/qa-report-localhost-2026-07-03.md
const t = (key: TranslationKey) => `i18n:${key}`

describe('errorMessage', () => {
  it('returns the message for Error instances and String() otherwise', () => {
    expect(errorMessage(new Error('boom'))).toBe('boom')
    expect(errorMessage('plain')).toBe('plain')
  })
})

describe('friendlyErrorMessage', () => {
  it('maps 429 to the rate-limit message instead of the raw code', () => {
    const err = new ApiError('too_many_requests', 429)
    expect(friendlyErrorMessage(t, err)).toBe('i18n:common.error_too_many_requests')
  })

  it('maps 5xx to the server-error message', () => {
    const err = new ApiError('internal server error', 500)
    expect(friendlyErrorMessage(t, err)).toBe('i18n:common.error_server')
  })

  it('maps status 0 (network/timeout) to the network message', () => {
    const err = new ApiError('Request timed out', 0)
    expect(friendlyErrorMessage(t, err)).toBe('i18n:common.error_network')
  })

  it('passes through meaningful 4xx server messages verbatim', () => {
    const err = new ApiError('current password is incorrect', 400)
    expect(friendlyErrorMessage(t, err)).toBe('current password is incorrect')
  })

  it('passes through plain errors and falls back on empty messages', () => {
    expect(friendlyErrorMessage(t, new Error('boom'))).toBe('boom')
    expect(friendlyErrorMessage(t, new Error(''))).toBe('i18n:common.unknown_error')
  })
})
