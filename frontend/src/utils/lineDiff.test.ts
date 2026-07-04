import { describe, expect, it } from 'vitest'

import { diffLines } from './lineDiff'

describe('diffLines', () => {
  it('returns all-same for identical texts', () => {
    const out = diffLines('a\nb', 'a\nb')
    expect(out).toEqual([
      { type: 'same', text: 'a' },
      { type: 'same', text: 'b' },
    ])
  })

  it('detects added and removed lines', () => {
    const out = diffLines('a\nb\nc', 'a\nx\nc')
    expect(out).toEqual([
      { type: 'same', text: 'a' },
      { type: 'del', text: 'b' },
      { type: 'add', text: 'x' },
      { type: 'same', text: 'c' },
    ])
  })

  it('handles pure additions and deletions at the end', () => {
    expect(diffLines('a', 'a\nb')).toEqual([
      { type: 'same', text: 'a' },
      { type: 'add', text: 'b' },
    ])
    expect(diffLines('a\nb', 'a')).toEqual([
      { type: 'same', text: 'a' },
      { type: 'del', text: 'b' },
    ])
  })

  it('handles empty inputs', () => {
    expect(diffLines('', '')).toEqual([{ type: 'same', text: '' }])
  })
})
