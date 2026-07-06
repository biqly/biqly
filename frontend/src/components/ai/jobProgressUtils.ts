import type { TrackedAIJob } from '../../hooks/useAIJobsUtils'
import type { TranslationKey } from '../../i18n/locale'
import type { AIQueueStatus } from '../../types/ai'

type JobT = (key: TranslationKey, params?: Record<string, string | number>) => string

function jobIsActiveStatus(job: TrackedAIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

export function formatJobDuration(ms: number, t: JobT): string {
  const totalSec = Math.max(0, ms) / 1000
  if (totalSec < 60) {
    const s = totalSec < 10 ? totalSec.toFixed(1) : String(Math.round(totalSec))
    return t('ai_jobs.duration_s', { s })
  }
  return t('ai_jobs.duration_m', {
    m: Math.floor(totalSec / 60),
    s: Math.round(totalSec % 60),
  })
}

export function jobTotalMs(job: TrackedAIJob, now: number): number | null {
  // From created_at, not started_at, so queue wait is included and the total
  // matches the per-step breakdown (which starts at "queued").
  const start = Date.parse(job.created_at)
  if (Number.isNaN(start)) {
    return null
  }
  if (jobIsActiveStatus(job)) {
    return now > 0 ? now - start : null
  }
  const end = Date.parse(job.finished_at ?? job.updated_at)
  return Number.isNaN(end) ? null : end - start
}

export function jobKindLabel(job: TrackedAIJob, t: JobT): string {
  switch (job.kind) {
    case 'describe_batch':
      return t('ai_jobs.kind_describe_batch')
    case 'describe':
      return t('ai_jobs.kind_describe')
    case 'embed_metadata':
      return t('ai_jobs.kind_embed_metadata')
    case 'run':
      return t('ai_jobs.kind_run')
    case 'preview':
      return t('ai_jobs.kind_preview')
    default:
      return t('ai_jobs.kind_query')
  }
}

export function queuePositionLine(
  job: TrackedAIJob,
  status: AIQueueStatus | null,
  t: JobT,
): string | null {
  if (!status?.my_position || job.phase !== 'queued') {
    return null
  }
  if (job.status !== 'pending' && job.status !== 'queued') {
    return null
  }
  if (status.my_job_id && status.my_job_id !== job.id) {
    return null
  }
  return t('ai_jobs.queue_position', { position: status.my_position })
}

export function describeBatchScopeLine(job: TrackedAIJob): string | null {
  if (job.kind !== 'describe_batch' || !job.scope_schemas?.length) {
    return null
  }
  return job.scope_schemas.join(', ')
}

export function describeBatchQueueLine(
  job: TrackedAIJob,
): { current: string | null; next: string | null } | null {
  if (job.kind !== 'describe_batch' || !job.progress_json) {
    return null
  }
  const p = job.progress_json
  const current =
    p.current_schema && p.current_table ? `${p.current_schema}.${p.current_table}` : null
  const next = p.pending_preview?.length ? p.pending_preview.join(', ') : null
  if (!current && !next) {
    return null
  }
  return { current, next }
}
