import { describe, expect, it } from 'vitest'

import { columnTypeIcon } from './columnTypeIcon'

describe('columnTypeIcon', () => {
  it('maps numeric types to the 123 glyph', () => {
    expect(columnTypeIcon('bigint')).toEqual({ kind: 'number', glyph: '123' })
    expect(columnTypeIcon('numeric(10,2)').kind).toBe('number')
  })

  it('maps text types to the A-Z glyph', () => {
    expect(columnTypeIcon('text')).toEqual({ kind: 'text', glyph: 'A-Z' })
    expect(columnTypeIcon('character varying').kind).toBe('text')
  })

  it('maps boolean, date and timestamp', () => {
    expect(columnTypeIcon('boolean').kind).toBe('boolean')
    expect(columnTypeIcon('date').kind).toBe('date')
    expect(columnTypeIcon('timestamp with time zone').kind).toBe('timestamp')
  })

  it('detects json and arrays and falls back to other', () => {
    expect(columnTypeIcon('jsonb').kind).toBe('json')
    expect(columnTypeIcon('text[]').kind).toBe('array')
    expect(columnTypeIcon('geometry').kind).toBe('other')
  })
})
