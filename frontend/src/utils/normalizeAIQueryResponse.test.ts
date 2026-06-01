import { describe, expect, it } from 'vitest'
import { normalizeAIQueryResponse } from './normalizeAIQueryResponse'

describe('normalizeAIQueryResponse', () => {
  it('unwraps nested ai.Response from job result_json', () => {
    const nested = {
      result: {
        sql: 'SELECT COUNT(*) FROM tweets',
        confidence: 0.9,
        logical_query: { datasource_id: 'ds1', model_id: 'm1', select: [], limit: 100 },
        result: {
          columns: [{ name: 'count', type: 'number' }],
          rows: [[3]],
        },
      },
      metadata: {
        model_used: 'mimo-v2.5',
        latency_ms: 1200,
        token_usage: { prompt: 100, completion: 50, total: 150 },
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.sql).toBe('SELECT COUNT(*) FROM tweets')
    expect(flat?.confidence).toBe(0.9)
    expect(flat?.model_used).toBe('mimo-v2.5')
    expect(flat?.latency_ms).toBe(1200)
    expect(flat?.result?.rows).toEqual([[3]])
  })

  it('passes through already-flat responses', () => {
    const flat = {
      sql: 'SELECT 1',
      confidence: 1,
      result: { rows: [[1]], columns: [] },
    }
    expect(normalizeAIQueryResponse(flat)?.sql).toBe('SELECT 1')
  })

  it('merges clarification from nested envelope', () => {
    const nested = {
      result: { warnings: ['pick a table'], confidence: 0 },
      clarification: {
        needs_clarification: true,
        clarification_question: 'Which table?',
        clarification_options: ['orders', 'customers'],
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.needs_clarification).toBe(true)
    expect(flat?.clarification_question).toBe('Which table?')
    expect(flat?.clarification_options).toEqual(['orders', 'customers'])
  })
})
