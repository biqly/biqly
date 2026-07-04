import { describe, expect, it } from 'vitest'

import type { SemanticModelDetail } from '../../types/semantic'
import { buildSuggestedQuestions } from './suggestedQuestions'

const t = (k: string, v?: Record<string, string | number>) => `${k}|${JSON.stringify(v)}`

function model(overrides: Partial<SemanticModelDetail>): SemanticModelDetail {
  return {
    id: 'm1',
    datasource_id: 'ds1',
    name: 'sales',
    base_schema: 'public',
    base_table: 'orders',
    status: 'published',
    ...overrides,
  }
}

describe('buildSuggestedQuestions', () => {
  it('yields multiple categories including trend for metrics + cat dim + date dim', () => {
    const result = buildSuggestedQuestions(
      model({
        metrics: [{ id: 'me1', name: 'revenue', aggregation: 'sum', expression: 'amount' }],
        dimensions: [
          { id: 'd1', name: 'country', column_ref: 'country', type: 'text' },
          { id: 'd2', name: 'order_date', column_ref: 'order_date', type: 'date' },
        ],
      }),
      t,
    )
    const categories = result.map((r) => r.category)
    expect(categories).toContain('aggregation')
    expect(categories).toContain('segmentation')
    expect(categories).toContain('trend')
    expect(categories).toContain('comparison')
  })

  it('yields only aggregation when there are metrics but no dimensions', () => {
    const result = buildSuggestedQuestions(
      model({
        metrics: [{ id: 'me1', name: 'revenue', aggregation: 'sum', expression: 'amount' }],
      }),
      t,
    )
    expect(result).toHaveLength(1)
    expect(result[0]?.category).toBe('aggregation')
  })

  it('returns [] for a null model', () => {
    expect(buildSuggestedQuestions(null, t)).toEqual([])
  })

  it('returns [] when the model has no metrics', () => {
    const result = buildSuggestedQuestions(
      model({
        dimensions: [{ id: 'd1', name: 'country', column_ref: 'country', type: 'text' }],
      }),
      t,
    )
    expect(result).toEqual([])
  })

  it('caps the total at 6', () => {
    const result = buildSuggestedQuestions(
      model({
        metrics: [
          { id: 'me1', name: 'revenue', aggregation: 'sum', expression: 'amount' },
          { id: 'me2', name: 'orders', aggregation: 'count', expression: '*' },
        ],
        dimensions: [
          { id: 'd1', name: 'country', column_ref: 'country', type: 'text' },
          { id: 'd2', name: 'segment', column_ref: 'segment', type: 'text' },
          { id: 'd3', name: 'order_date', column_ref: 'order_date', type: 'date' },
        ],
      }),
      t,
    )
    expect(result.length).toBeLessThanOrEqual(6)
  })

  it('ignores inactive metrics and dimensions', () => {
    const result = buildSuggestedQuestions(
      model({
        metrics: [
          {
            id: 'me1',
            name: 'revenue',
            aggregation: 'sum',
            expression: 'amount',
            is_active: false,
          },
        ],
      }),
      t,
    )
    expect(result).toEqual([])
  })

  it('prefers label over name and humanizes underscores', () => {
    const result = buildSuggestedQuestions(
      model({
        metrics: [
          {
            id: 'me1',
            name: 'total_revenue',
            label: 'Net Revenue',
            aggregation: 'sum',
            expression: 'amount',
          },
        ],
      }),
      t,
    )
    expect(result[0]?.text).toContain('Net Revenue')
  })
})
