import { describe, expect, it } from 'vitest'

import type { TFunction } from '../../i18n'
import type { QueryResultPayload } from '../../types/ai'
import { buildResultInsight } from './resultInsight'

// Identity stub: echoes the key + interpolation vars so assertions can inspect
// exactly what buildResultInsight passed to `t`.
const tStub = ((key: string, vars?: Record<string, string | number>) =>
  JSON.stringify({ key, vars })) as unknown as TFunction

function parse(caption: string | null): { key: string; vars?: Record<string, string | number> } {
  if (caption === null) {
    throw new Error('expected a caption, got null')
  }
  return JSON.parse(caption) as { key: string; vars?: Record<string, string | number> }
}

describe('buildResultInsight', () => {
  it('builds a ranked caption picking the max row category', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Country', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [
        ['France', 30],
        ['Germany', 90],
        ['Spain', 10],
      ],
    }
    const { key, vars } = parse(buildResultInsight(result, tStub, 'en-US'))
    expect(key).toBe('ai_query.insight_ranked')
    expect(vars?.top).toBe('Germany')
    expect(vars?.metric).toBe('Revenue')
    expect(vars?.dim).toBe('Country')
    expect(vars?.n).toBe(3)
    expect(vars?.topVal).toBe('90')
    expect(vars?.minVal).toBe('10')
    expect(vars?.maxVal).toBe('90')
  })

  it('builds a single-KPI caption for one metric column and one row', () => {
    const result: QueryResultPayload = {
      columns: [{ name: 'Total Revenue', semantic_type: 'metric' }],
      rows: [[1234.5]],
    }
    const { key, vars } = parse(buildResultInsight(result, tStub, 'en-US'))
    expect(key).toBe('ai_query.insight_single')
    expect(vars?.metric).toBe('Total Revenue')
    expect(vars?.val).toBe('1,234.5')
  })

  it('returns null when there is no metric column', () => {
    const result: QueryResultPayload = {
      columns: [{ name: 'Country', semantic_type: 'dimension' }],
      rows: [['France'], ['Germany']],
    }
    expect(buildResultInsight(result, tStub, 'en-US')).toBeNull()
  })

  it('returns null when there are no rows', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Country', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [],
    }
    expect(buildResultInsight(result, tStub, 'en-US')).toBeNull()
  })

  it('returns null when metric values are all non-numeric', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Country', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [
        ['France', 'n/a'],
        ['Germany', 'unknown'],
      ],
    }
    expect(buildResultInsight(result, tStub, 'en-US')).toBeNull()
  })

  it('returns null for undefined result', () => {
    expect(buildResultInsight(undefined, tStub, 'en-US')).toBeNull()
  })
})
