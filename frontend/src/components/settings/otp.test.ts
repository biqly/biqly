import { describe, expect, it } from 'vitest'

import { normalizeOTPCode } from './otp'

describe('OTP code normalization', () => {
  it('keeps only the first six numeric characters', () => {
    expect(normalizeOTPCode('12a3 4567')).toBe('123456')
  })
})
