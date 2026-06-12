import { describe, expect, it } from 'vitest'

import type { AIQueryResponse } from '../types/ai'
import { buildResultSummary } from './priorTurnSummary'

describe('buildResultSummary', () => {
  it('summarizes up to five result rows with column labels', () => {
    const response: AIQueryResponse = {
      result: {
        columns: [
          { name: 'tweet_day', semantic_type: 'dimension' },
          { name: 'tweet_count', semantic_type: 'metric' },
        ],
        rows: [
          ['May 20, 2026', 2932],
          ['May 21, 2026', 1440],
          ['May 22, 2026', 991],
          ['May 23, 2026', 812],
        ],
      },
    }

    expect(buildResultSummary(response)).toContain('tweet_day=May 23, 2026, tweet_count=812')
    expect(buildResultSummary(response)).not.toContain('and 1 more')
  })

  it('summarizes clarification, preview, empty, error, and missing responses', () => {
    expect(buildResultSummary({ needs_clarification: true })).toBe('clarification needed')
    expect(buildResultSummary({ sql: 'select * from tweets where created_at >= now()' })).toBe(
      'SQL generated: select * from tweets where created_at >= now()',
    )
    expect(buildResultSummary({ result: { columns: [], rows: [] } })).toBe('no results')
    expect(buildResultSummary({ warnings: ['bad metric'] })).toBe('warning: bad metric')
    expect(buildResultSummary(null)).toBe('no response')
  })

  it('summarizes large result sets within the 300 character budget', () => {
    const response: AIQueryResponse = {
      result: {
        columns: [{ name: 'tweet_day' }, { name: 'tweet_count' }],
        rows: Array.from({ length: 1000 }, (_, index) => [
          index === 0 ? 'May 20, 2026' : `May ${index + 1}, 2026`,
          index === 0 ? 2932 : 1000 - index,
        ]),
      },
    }

    const summary = buildResultSummary(response)
    expect(summary.length).toBeLessThanOrEqual(300)
    expect(summary).toContain('May 20, 2026')
    expect(summary).toContain('and 997 more')
    expect(summary).toContain('max tweet_count=2932')
  })
})
