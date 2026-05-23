import { describe, expect, it } from 'vitest'
import { plainTextFromHTML } from './plainText'

describe('plainTextFromHTML', () => {
  it('strips tags and normalizes whitespace', () => {
    expect(plainTextFromHTML('<div>HTTP <strong>500</strong></div>\n<p>Bad gateway</p>')).toBe(
      'HTTP 500 Bad gateway',
    )
  })

  it('returns trimmed plain text when no tags are present', () => {
    expect(plainTextFromHTML('  invalid JSON response  ')).toBe('invalid JSON response')
  })
})
