import type { TrackedAIJob } from '../hooks/useAIJobsUtils'
import type { TranslationKey } from '../i18n/locale'
import type { AIQueueStatus } from '../types/ai'

type JobT = (key: TranslationKey, params?: Record<string, string | number>) => string

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
