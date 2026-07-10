import { describe, expect, it } from 'vitest'

import type { QueryResultPayload } from '../../types/ai'
import { chartTypeOptions, relevantChartTypes } from './chartRelevance'

function payload(columns: QueryResultPayload['columns'], rowCount: number): QueryResultPayload {
  return {
    columns,
    rows: Array.from({ length: rowCount }, (_, i) => columns.map(() => i)),
  }
}

describe('relevantChartTypes', () => {
  it('returns table only for single-value results', () => {
    expect(relevantChartTypes(payload([{ name: 'count', semantic_type: 'metric' }], 1))).toEqual([
      'table',
    ])
  })

  it('prefers line for time series', () => {
    const p = payload(
      [
        { name: 'created_at_day', semantic_type: 'dimension' },
        { name: 'count', semantic_type: 'metric' },
      ],
      30,
    )
    expect(relevantChartTypes(p)).toEqual(['line', 'bar', 'table'])
  })

  it('detects time via column format', () => {
    const p = payload(
      [
        { name: 'periode', format: 'date', semantic_type: 'dimension' },
        { name: 'count', semantic_type: 'metric' },
      ],
      5,
    )
    expect(relevantChartTypes(p)).toEqual(['line', 'bar', 'table'])
  })

  it('offers pie only for small categorical results', () => {
    const cols: QueryResultPayload['columns'] = [
      { name: 'category', semantic_type: 'dimension' },
      { name: 'total', semantic_type: 'metric' },
    ]
    expect(relevantChartTypes(payload(cols, 6))).toEqual(['bar', 'pie', 'table'])
    expect(relevantChartTypes(payload(cols, 40))).toEqual(['bar', 'table'])
  })
})

describe('chartTypeOptions', () => {
  it('keeps the selected type visible even when the heuristic drops it', () => {
    const p = payload(
      [
        { name: 'category', semantic_type: 'dimension' },
        { name: 'total', semantic_type: 'metric' },
      ],
      40,
    )
    expect(chartTypeOptions(p, 'pie')).toEqual(['bar', 'pie', 'table'])
    expect(chartTypeOptions(p, 'bar')).toEqual(['bar', 'table'])
  })
})
