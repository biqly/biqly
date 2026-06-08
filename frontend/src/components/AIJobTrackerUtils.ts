import type { TrackedAIJob } from '../hooks/useAIJobsUtils'
import type { TranslationKey } from '../i18n/locale'

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
