import { describe, expect, it } from 'vitest'

import type { AIQueryResponse, SuggestedFollowUp } from '../../types/ai'
import { buildFallbackFollowUps, filterFollowUpSuggestions } from './followUpSuggestions'

const t = (k: string) => k

function metricResult(
  overrides: Partial<AIQueryResponse['result']> = {},
): AIQueryResponse['result'] {
  return {
    columns: [
      { name: 'order_date', semantic_type: 'dimension' },
      { name: 'revenue', semantic_type: 'metric' },
    ],
    rows: [
      ['2026-01-01', 100],
      ['2026-01-02', 200],
    ],
    ...overrides,
  }
}

describe('filterFollowUpSuggestions', () => {
  it('keeps backend suggestions and caps them at three', () => {
    const suggestions: SuggestedFollowUp[] = [
      { id: '1', kind: 'trend', label: 'Trend', question: 'How did revenue trend over time?' },
      {
        id: '2',
        kind: 'comparison',
        label: 'Compare',
        question: 'How does revenue compare by region?',
      },
      { id: '3', kind: 'chart', label: 'Chart', question: 'Show this as a chart' },
      { id: '4', kind: 'breakdown', label: 'Breakdown', question: 'Break revenue down by product' },
    ]

    const result = filterFollowUpSuggestions(suggestions, [])

    expect(result).toHaveLength(3)
    expect(result.map((s) => s.id)).toEqual(['1', '2', '3'])
  })

  it('drops suggestions matching prior user questions', () => {
    const suggestions: SuggestedFollowUp[] = [
      { id: '1', kind: 'trend', label: 'Trend', question: 'How did revenue trend over time?' },
      {
        id: '2',
        kind: 'comparison',
        label: 'Compare',
        question: 'How does revenue compare by region?',
      },
    ]

    const result = filterFollowUpSuggestions(suggestions, ['How did revenue trend over time?'])

    expect(result).toHaveLength(1)
    expect(result[0]?.id).toBe('2')
  })

  it('drops a candidate whose normalized text contains a prior question (substring match)', () => {
    const suggestions: SuggestedFollowUp[] = [
      {
        id: '1',
        kind: 'trend',
        label: 'Busiest hour',
        question: 'What was the busiest hour today in detail?',
      },
      {
        id: '2',
        kind: 'comparison',
        label: 'Compare',
        question: 'How does revenue compare by region?',
      },
    ]

    const result = filterFollowUpSuggestions(suggestions, ['What was the busiest hour?'])

    expect(result).toHaveLength(1)
    expect(result[0]?.id).toBe('2')
  })

  it('drops a candidate whose normalized text is contained by a prior question (substring match)', () => {
    const suggestions: SuggestedFollowUp[] = [
      { id: '1', kind: 'trend', label: 'Busiest hour', question: 'What was the busiest hour?' },
      {
        id: '2',
        kind: 'comparison',
        label: 'Compare',
        question: 'How does revenue compare by region?',
      },
    ]

    const result = filterFollowUpSuggestions(suggestions, [
      'What was the busiest hour today in detail?',
    ])

    expect(result).toHaveLength(1)
    expect(result[0]?.id).toBe('2')
  })
})

describe('buildFallbackFollowUps', () => {
  it('builds chart fallback for metric results', () => {
    const response: AIQueryResponse = { result: metricResult() }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result.some((s) => s.kind === 'chart')).toBe(true)
    expect(result.some((s) => s.kind === 'trend')).toBe(true)
  })

  it('returns no fallback for empty results', () => {
    const response: AIQueryResponse = {
      result: { columns: [{ name: 'revenue', semantic_type: 'metric' }], rows: [] },
    }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result).toEqual([])
  })
})
