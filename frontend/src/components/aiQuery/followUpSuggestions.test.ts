import { describe, expect, it } from 'vitest'

import type { AIQueryResponse, SuggestedFollowUp } from '../../types/ai'
import {
  buildFallbackFollowUps,
  filterFollowUpSuggestions,
  localizeFollowUps,
} from './followUpSuggestions'

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
  it('builds contextual follow-ups for time series results', () => {
    const response: AIQueryResponse = { result: metricResult() }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result.some((s) => s.kind === 'comparison')).toBe(true)
    expect(result.some((s) => s.kind === 'explain')).toBe(true)
  })

  it('offers a top-N of the actual dimension for categorical results', () => {
    const response: AIQueryResponse = {
      result: metricResult({
        columns: [
          { name: 'region', semantic_type: 'dimension' },
          { name: 'revenue', semantic_type: 'metric' },
        ],
      }),
    }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result.some((s) => s.kind === 'breakdown')).toBe(true)
    expect(result.some((s) => s.kind === 'chart')).toBe(true)
  })

  it('suggests period comparison and breakdown for single-value results', () => {
    const response: AIQueryResponse = {
      result: { columns: [{ name: 'count', semantic_type: 'metric' }], rows: [[1127]] },
    }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result.some((s) => s.kind === 'comparison')).toBe(true)
    expect(result.some((s) => s.kind === 'breakdown')).toBe(true)
  })

  it('returns no fallback for empty results', () => {
    const response: AIQueryResponse = {
      result: { columns: [{ name: 'revenue', semantic_type: 'metric' }], rows: [] },
    }

    const result = buildFallbackFollowUps({ response, priorQuestions: [], t })

    expect(result).toEqual([])
  })
})

describe('localizeFollowUps', () => {
  it('replaces label and question of known deterministic ids with i18n copy', () => {
    const suggestions: SuggestedFollowUp[] = [
      {
        id: 'drilldown-detail',
        kind: 'drilldown',
        label: 'See more detail',
        question: 'Break this result down into more detail',
      },
      {
        id: 'breakdown-by-dimension',
        kind: 'breakdown',
        label: 'Break down by category',
        question: 'Break this result down by a category',
      },
    ]

    // Identity t returns the key, proving the i18n key was used (not the
    // server's English text).
    const result = localizeFollowUps(suggestions, t)

    expect(result[0]?.label).toBe('ai_query.followups_more_detail_label')
    expect(result[0]?.question).toBe('ai_query.followups_more_detail_question')
    expect(result[1]?.label).toBe('ai_query.followups_breakdown_category_label')
    expect(result[1]?.question).toBe('ai_query.followups_breakdown_category_question')
  })

  it('leaves unknown (e.g. AI-rewritten) suggestions untouched', () => {
    const suggestions: SuggestedFollowUp[] = [
      {
        id: 'ai-rewrite-42',
        kind: 'breakdown',
        label: 'Break tweets down by author',
        question: 'Break yesterday tweet count down by author',
      },
    ]

    const result = localizeFollowUps(suggestions, t)

    expect(result[0]).toEqual(suggestions[0])
  })
})
