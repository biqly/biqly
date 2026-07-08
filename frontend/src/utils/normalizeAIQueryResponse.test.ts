import { describe, expect, it } from 'vitest'

import type { AgentResultEvent } from '../types/agent'
import { normalizeAgentResultEvent, normalizeAIQueryResponse } from './normalizeAIQueryResponse'

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

  it('unwraps the server-synthesized natural-language answer', () => {
    const nested = {
      result: {
        sql: 'SELECT COUNT(*) FROM tweets',
        confidence: 0.9,
        answer: 'Geçen hafta 5.658 tweet atılmıştır.',
        result: { columns: [{ name: 'count', type: 'number' }], rows: [[5658]] },
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.answer).toBe('Geçen hafta 5.658 tweet atılmıştır.')
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

  it('unwraps run_steps from metadata', () => {
    const nested = {
      result: {
        sql: 'SELECT 1',
        confidence: 0.8,
      },
      metadata: {
        run_steps: [
          { seq: 1, kind: 'table_route', status: 'ok', duration_ms: 12 },
          {
            seq: 2,
            kind: 'llm_generate',
            status: 'failed',
            attempt: 1,
            duration_ms: 850,
            detail: 'provider timeout',
          },
          'not-a-step',
        ],
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.run_steps).toHaveLength(2)
    expect(flat?.run_steps?.[0]?.kind).toBe('table_route')
    expect(flat?.run_steps?.[1]?.status).toBe('failed')
    expect(flat?.run_steps?.[1]?.detail).toBe('provider timeout')
  })

  it('unwraps suggested_followups from the nested result', () => {
    const nested = {
      result: {
        sql: 'SELECT 1',
        confidence: 0.8,
        suggested_followups: [
          { id: 'f1', kind: 'trend', label: 'Trend', question: 'How did this trend over time?' },
        ],
      },
    }
    const flat = normalizeAIQueryResponse(nested)
    expect(flat?.suggested_followups).toHaveLength(1)
    expect(flat?.suggested_followups?.[0]?.id).toBe('f1')
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

describe('normalizeAgentResultEvent', () => {
  it('maps a full agent result payload onto the flat AIQueryResponse shape', () => {
    const event: AgentResultEvent = {
      type: 'result',
      run_id: 'run-1',
      result: {
        sql: 'SELECT COUNT(*) FROM tweets',
        confidence: 0.9,
        logical_query: { datasource_id: 'ds1', model_id: 'm1', select: [], limit: 100 },
        result: { columns: [{ name: 'count', type: 'number' }], rows: [[3]] },
        answer: '3 tweets found.',
        suggested_followups: [
          { id: 'f1', kind: 'trend', label: 'Trend', question: 'How did this trend?' },
        ],
      },
    }
    const flat = normalizeAgentResultEvent(event)
    expect(flat?.sql).toBe('SELECT COUNT(*) FROM tweets')
    expect(flat?.confidence).toBe(0.9)
    expect(flat?.result?.rows).toEqual([[3]])
    expect(flat?.answer).toBe('3 tweets found.')
    expect(flat?.suggested_followups).toHaveLength(1)
    // run_id isn't part of AgentResultPayload — it's carried on the envelope
    // event, so the adapter must backfill it after normalizing.
    expect(flat?.run_id).toBe('run-1')
  })

  it('falls back to the top-level answer/confidence when result is absent', () => {
    const event: AgentResultEvent = {
      type: 'result',
      run_id: 'run-2',
      answer: 'Hello there.',
      confidence: 0.5,
    }
    const flat = normalizeAgentResultEvent(event)
    expect(flat?.answer).toBe('Hello there.')
    expect(flat?.confidence).toBe(0.5)
    expect(flat?.run_id).toBe('run-2')
  })

  it('does not overwrite a run_id already present on the result payload', () => {
    const event: AgentResultEvent = {
      type: 'result',
      run_id: 'run-envelope',
      result: {
        confidence: 1,
        sql: 'SELECT 1',
        // AIResult itself has no run_id field today, but guard against the
        // adapter clobbering one if the backend ever adds it there too.
      },
    }
    const flat = normalizeAgentResultEvent(event)
    expect(flat?.run_id).toBe('run-envelope')
  })

  it('survives a second normalizeAIQueryResponse pass (AssistantMessageCard re-normalizes on every render)', () => {
    const event: AgentResultEvent = {
      type: 'result',
      run_id: 'run-3',
      result: { confidence: 1, sql: 'SELECT 1', answer: 'One.' },
    }
    const onceFlat = normalizeAgentResultEvent(event)
    expect(onceFlat?.run_id).toBe('run-3')

    // AssistantMessageCard does `normalizeAIQueryResponse(message.ai_response)`
    // on every render — message.ai_response IS onceFlat here. Re-running it
    // must not drop run_id (regression: it previously did, since run_id was
    // only ever assigned by assignMetadataFields, reached solely via the
    // nested-envelope unwrap branch — the flat passthrough branch this
    // already-flat object takes never copied it).
    const twiceFlat = normalizeAIQueryResponse(onceFlat)
    expect(twiceFlat?.run_id).toBe('run-3')
  })
})
