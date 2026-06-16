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

  it('unwraps generation_trace from metadata', () => {
    const nested = {
      result: {
        sql: 'SELECT 1',
        confidence: 0.8,
      },
      metadata: {
        generation_trace: {
          routed_table: 'orders',
          route_confidence: 0.91,
          ambiguity_result: 'passed',
          columns_resolved: [{ term: 'revenue', resolved: 'sum(orders.total_amount)' }],
        },
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.generation_trace?.routed_table).toBe('orders')
    expect(flat?.generation_trace?.columns_resolved?.[0]?.resolved).toBe('sum(orders.total_amount)')
  })

  it('filters non-string clarification_options entries', () => {
    const nested = {
      result: { confidence: 0 },
      clarification: {
        needs_clarification: true,
        clarification_question: 'Pick one',
        clarification_options: ['orders', 42, null, 'customers', { bad: true }],
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.clarification_options).toEqual(['orders', 'customers'])
  })
})
