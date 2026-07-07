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
    expect(key).toBe('ai_query.insight_ranked_explained')
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
    expect(key).toBe('ai_query.insight_single_explained')
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

  it('returns a richer ranked explanation', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Region', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [
        ['North', 40],
        ['South', 120],
        ['East', 70],
      ],
    }
    const { key, vars } = parse(buildResultInsight(result, tStub, 'en-US'))
    expect(key).toBe('ai_query.insight_ranked_explained')
    expect(vars).toEqual({
      top: 'South',
      metric: 'Revenue',
      topVal: '120',
      minVal: '40',
      maxVal: '120',
      n: 3,
      dim: 'Region',
    })
  })

  it('returns a richer single KPI explanation', () => {
    const result: QueryResultPayload = {
      columns: [{ name: 'Active Users', semantic_type: 'metric' }],
      rows: [[500]],
    }
    const { key, vars } = parse(buildResultInsight(result, tStub, 'en-US'))
    expect(key).toBe('ai_query.insight_single_explained')
    expect(vars).toEqual({ metric: 'Active Users', val: '500' })
  })

  it('does not invent time grain for unknown columns', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Country', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [
        ['France', 30],
        ['Germany', 90],
      ],
    }
    // 'Country' does not clearly indicate any time grain, so the caption
    // must be the ranked sentence alone — a single `t()` call, no
    // time-bucket sentence prepended.
    const { key } = parse(buildResultInsight(result, tStub, 'en-US'))
    expect(key).toBe('ai_query.insight_ranked_explained')
  })

  it('combines a time-bucket explanation with the ranked explanation for a detected grain', () => {
    const result: QueryResultPayload = {
      columns: [
        { name: 'Hour Bucket', semantic_type: 'dimension' },
        { name: 'Revenue', semantic_type: 'metric' },
      ],
      rows: [
        ['00:00', 30],
        ['01:00', 90],
      ],
    }
    const caption = buildResultInsight(result, tStub, 'en-US')
    expect(caption).not.toBeNull()

    // The bucket sentence's `t()` call never emits a literal space (all
    // interpolated values are single tokens), so the first space in the
    // combined caption is exactly the separator between the two sentences.
    const splitAt = caption!.indexOf(' ')
    expect(splitAt).toBeGreaterThan(0)
    const bucket = parse(caption!.slice(0, splitAt))
    const ranked = parse(caption!.slice(splitAt + 1))

    expect(bucket.key).toBe('ai_query.insight_time_bucket_explained')
    expect(bucket.vars?.grain).toBeDefined()
    expect(parse(String(bucket.vars?.grain)).key).toBe('ai_query.time_grain_hour')

    expect(ranked.key).toBe('ai_query.insight_ranked_explained')
    expect(ranked.vars?.dim).toBe('Hour Bucket')
    expect(ranked.vars?.top).toBe('01:00')
  })
})
