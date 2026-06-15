import type { TranslationKey } from '../../i18n'

type Translate = (key: TranslationKey, params?: Record<string, string | number>) => string

const abExperimentStatusBadgeBaseClass =
  'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium capitalize'

export function abExperimentStatusBadgeClass(status?: string): string {
  switch (status) {
    case 'draft':
      return `${abExperimentStatusBadgeBaseClass} bg-[#f3f4f6] text-[#374151] dark:bg-zinc-800 dark:text-zinc-300`
    case 'running':
      return `${abExperimentStatusBadgeBaseClass} bg-[#ecfdf5] text-[#065f46] dark:bg-emerald-950/30 dark:text-emerald-400`
    case 'paused':
      return `${abExperimentStatusBadgeBaseClass} bg-[#fffbeb] text-[#92400e] dark:bg-amber-950/30 dark:text-amber-400`
    case 'completed':
      return `${abExperimentStatusBadgeBaseClass} bg-[#eff6ff] text-[#1e40af] dark:bg-blue-950/30 dark:text-blue-400`
    default:
      return abExperimentStatusBadgeBaseClass
  }
}

export function abExperimentStatusLabel(status: string | undefined, t: Translate): string {
  switch (status) {
    case 'draft':
      return t('admin.ab_experiments.status_draft')
    case 'running':
      return t('admin.ab_experiments.status_running')
    case 'paused':
      return t('admin.ab_experiments.status_paused')
    case 'completed':
      return t('admin.ab_experiments.status_completed')
    default:
      return status ?? '—'
  }
}
