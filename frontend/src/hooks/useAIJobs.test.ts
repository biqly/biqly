import { describe, expect, it } from 'vitest'
import type { AIJob } from '../types/ai'
import { fetchJSON, jobQuestionPreview, trackedJobFromAIJob } from './useAIJobs'

function job(overrides: Partial<AIJob>): AIJob {
  return {
    id: 'job-1',
    client_session_id: 'session-1',
    kind: 'query',
    status: 'running',
    phase: 'queued',
    phase_message: '',
    progress_pct: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('jobQuestionPreview', () => {
  it('uses schema and table for describe jobs', () => {
    expect(jobQuestionPreview('describe', { schema: 'sales', table: 'orders' })).toBe('sales.orders')
  })

  it('summarizes describe batch jobs by table count', () => {
    expect(jobQuestionPreview('describe_batch', { tables: [{}, {}, {}] })).toBe('3 tables')
  })

  it('truncates long natural-language questions', () => {
    const preview = jobQuestionPreview('query', { question: 'x'.repeat(100) })
    expect(preview).toHaveLength(78)
    expect(preview.endsWith('…')).toBe(true)
  })
})

describe('trackedJobFromAIJob', () => {
  it('adds the shared question preview to tracked jobs', () => {
    expect(trackedJobFromAIJob(job({ request_json: { question: 'total revenue' } })).questionPreview).toBe(
      'total revenue',
    )
  })
})

describe('fetchJSON', () => {
  it('returns a visible error for invalid JSON responses', async () => {
    const originalFetch = globalThis.fetch
    globalThis.fetch = (() =>
      Promise.resolve(new Response('<html>nope</html>', { status: 200 }))) as typeof fetch

    try {
      expect(await fetchJSON('/api/test')).toEqual({
        data: null,
        status: 200,
        error: 'Invalid JSON response from /api/test',
      })
    } finally {
      globalThis.fetch = originalFetch
    }
  })
})
