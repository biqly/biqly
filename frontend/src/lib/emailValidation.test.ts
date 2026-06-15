import { describe, expect, it } from 'vitest'

import { isValidEmailFormat } from './emailValidation'

describe('isValidEmailFormat', () => {
  it.each([
    'user@example.com',
    'User@Example.com',
    '  user@example.com  ',
    'foo+bar@example.com',
    'f.o.o@example.com',
  ])('accepts %s', (email) => {
    expect(isValidEmailFormat(email)).toBe(true)
  })

  it.each([
    '',
    '   ',
    'example.com',
    'user@@example.com',
    '@example.com',
    'user@',
    'fsfasdf@',
    'user<@example.com',
    'user>@example.com',
    'user@example',
    'user@.com',
    'user@example.',
  ])('rejects %s', (email) => {
    expect(isValidEmailFormat(email)).toBe(false)
  })
})
