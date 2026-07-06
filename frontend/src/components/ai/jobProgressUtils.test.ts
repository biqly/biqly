import { describe, expect, it } from 'vitest'

import type { TrackedAIJob } from '../../hooks/useAIJobsUtils'
import type { AIQueueStatus } from '../../types/ai'
import { queuePositionLine } from './jobProgressUtils'

function job(overrides: Partial<TrackedAIJob> = {}): TrackedAIJob {
  return {
    id: 'job-1',
    client_session_id: 'session-1',
    kind: 'query',
    status: 'queued',
    phase: 'queued',
    phase_message: '',
    progress_pct: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

const translate = (key: string, params?: Record<string, string | number>) => {
  if (key !== 'ai_jobs.queue_position') {
    return key
  }
  return `Sırada ${params?.position}. sıradasınız`
}

describe('queuePositionLine', () => {
  it('describes the caller queue position while the job is queued', () => {
    const status: AIQueueStatus = {
      total_pending: 4,
      my_position: 3,
      my_job_id: 'job-1',
      my_job_status: 'queued',
    }

    expect(queuePositionLine(job(), status, translate)).toBe('Sırada 3. sıradasınız')
  })

  it('hides the queue position for non-queued jobs or missing positions', () => {
    const status: AIQueueStatus = {
      total_pending: 4,
      my_job_id: 'job-1',
      my_job_status: 'running',
    }

    expect(queuePositionLine(job({ status: 'running', phase: 'routing' }), status, translate)).toBe(
      null,
    )
    expect(queuePositionLine(job(), status, translate)).toBe(null)
  })
})
