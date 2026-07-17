import { useMemo, useState } from 'react'

import { jobIsActive, type TrackedAIJob, useAIJobs } from '../../hooks/useAIJobs'
import { useJobNow } from '../../hooks/useJobNow'
import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import {
  aiJobCardBodyClass,
  aiJobCardCancelClass,
  aiJobCardCancelledClass,
  aiJobCardClass,
  aiJobCardErrorClass,
  aiJobCardHeadClass,
  aiJobCardHintClass,
  aiJobCardMetaClass,
  aiJobCardProgressClass,
  aiJobCardProgressFillClass,
  aiJobCardTitleClass,
  aiJobCardTitlesClass,
  aiJobCardTotalClass,
  aiJobCardTotalTimeClass,
} from '../ai/aiJobsClasses'
import { JobPhaseSteps } from '../ai/JobPhaseSteps'
import {
  describeBatchQueueLine,
  describeBatchScopeLine,
  formatJobDuration,
  jobKindLabel,
  jobTotalMs,
} from '../ai/jobProgressUtils'

type Translate = (key: TranslationKey, params?: Record<string, string | number>) => string

function jobCardModifier(job: TrackedAIJob): '' | 'active' | 'failed' | 'done' {
  if (jobIsActive(job)) {
    return 'active'
  }
  if (job.status === 'failed') {
    return 'failed'
  }
  if (job.status === 'succeeded') {
    return 'done'
  }
  return ''
}

function jobStatusLabel(job: TrackedAIJob, t: Translate): string | null {
  if (job.status === 'succeeded') {
    return t('ai_jobs.completed')
  }
  if (job.status === 'failed') {
    return t('ai_jobs.failed')
  }
  if (job.status === 'cancelled') {
    return t('ai_jobs.cancelled')
  }
  return null
}

function StripJobHints({ job }: { job: TrackedAIJob }) {
  const t = useT()
  const active = jobIsActive(job)
  const queueLine = describeBatchQueueLine(job)
  const scopeLine = describeBatchScopeLine(job)
  return (
    <>
      {scopeLine && (
        <p className={aiJobCardHintClass}>{t('ai_jobs.scope_schemas', { schemas: scopeLine })}</p>
      )}
      {/* phase_message duplicates the "Current: <table>" line for batch
          describes ("describing public.x") — show it only when there is no
          structured queue info. */}
      {job.phase_message && active && !queueLine?.current && (
        <p className={aiJobCardHintClass}>{job.phase_message}</p>
      )}
      {queueLine?.current && (
        <p className={aiJobCardHintClass}>
          {t('ai_jobs.queue_current', { table: queueLine.current })}
        </p>
      )}
      {queueLine?.next && (
        <p className={aiJobCardHintClass}>{t('ai_jobs.queue_next', { tables: queueLine.next })}</p>
      )}
      {job.status === 'failed' && job.error_message && (
        <p className={aiJobCardErrorClass} role="alert">
          {job.error_message}
        </p>
      )}
      {job.status === 'cancelled' && (
        <p className={aiJobCardCancelledClass}>
          {(job.error_message ?? job.phase_message) || t('ai_jobs.cancelled')}
        </p>
      )}
      {active && job.status === 'queued' && (
        <p className={aiJobCardHintClass}>{t('ai_jobs.stuck_hint')}</p>
      )}
    </>
  )
}

function StripJobCard({
  job,
  now,
  onCancel,
  cancelling,
  onDismiss,
}: {
  job: TrackedAIJob
  now: number
  onCancel: () => void
  cancelling: boolean
  onDismiss: () => void
}) {
  const t = useT()
  const active = jobIsActive(job)
  const kindLabel = jobKindLabel(job, t)
  const total = jobTotalMs(job, now)
  const statusLabel = jobStatusLabel(job, t)

  const metaParts = [kindLabel]
  if (active) {
    metaParts.push(`${job.progress_pct}%`)
  } else if (statusLabel) {
    metaParts.push(statusLabel)
  }

  return (
    <article className={aiJobCardClass(jobCardModifier(job))}>
      <header className={aiJobCardHeadClass}>
        <span className={aiJobCardTitlesClass}>
          <strong className={aiJobCardTitleClass}>{job.questionPreview ?? kindLabel}</strong>
          <span className={aiJobCardMetaClass}>{metaParts.join(' · ')}</span>
        </span>
        {active ? (
          <button
            type="button"
            className={cn(
              buttonClass('ghost', { size: 'sm', autoWidth: true }),
              aiJobCardCancelClass,
            )}
            onClick={onCancel}
            disabled={cancelling}
          >
            {cancelling ? t('ai_jobs.cancelling') : t('ai_jobs.cancel')}
          </button>
        ) : (
          <button
            type="button"
            className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
            onClick={onDismiss}
            aria-label={t('ai_jobs.dismiss')}
          >
            ×
          </button>
        )}
      </header>
      {active && (
        <div
          className={aiJobCardProgressClass}
          role="progressbar"
          aria-valuenow={job.progress_pct}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          <span
            className={aiJobCardProgressFillClass}
            style={{ width: `${Math.min(100, Math.max(0, job.progress_pct))}%` }}
          />
        </div>
      )}
      <div className={aiJobCardBodyClass}>
        <JobPhaseSteps job={job} now={now} />
        {total != null && (
          <p className={aiJobCardTotalClass}>
            <span>{t('ai_jobs.total_label')}</span>
            <span className={aiJobCardTotalTimeClass}>{formatJobDuration(total, t)}</span>
          </p>
        )}
        <StripJobHints job={job} />
      </div>
    </article>
  )
}

/** In-page live tracker for metadata describe jobs (single and bulk),
 * replacing the old floating bottom-right tray on this page. */
export function MetadataAIJobsStrip() {
  const t = useT()
  const { jobs, cancelJob, dismissJob } = useAIJobs()
  const [cancellingId, setCancellingId] = useState<string | null>(null)

  const metadataJobs = useMemo(
    () =>
      jobs.filter(
        (j) =>
          j.kind === 'describe' || j.kind === 'describe_batch' || j.kind === 'describe_relations',
      ),
    [jobs],
  )
  const activeCount = useMemo(() => metadataJobs.filter(jobIsActive).length, [metadataJobs])
  const now = useJobNow(activeCount > 0)

  if (metadataJobs.length === 0) {
    return null
  }

  return (
    <section className={cardClass()} aria-live="polite">
      <header className="mb-3 flex items-center gap-2">
        <span
          className={cn(
            'size-[0.55rem] shrink-0 rounded-full',
            activeCount > 0
              ? 'bg-emerald-400 motion-safe:animate-[ai-job-pulse_1.6s_ease_infinite]'
              : 'bg-border-strong',
          )}
          aria-hidden="true"
        />
        <h3 className="m-0 text-[0.9rem]">{t('ai_jobs.inline_title')}</h3>
        {activeCount > 0 && (
          <span className="text-foreground-muted text-[0.78rem]">
            {t('ai_jobs.panel_sub_active', { count: activeCount })}
          </span>
        )}
      </header>
      <div className="grid gap-[0.65rem]">
        {metadataJobs.map((job) => (
          <StripJobCard
            key={job.id}
            job={job}
            now={now}
            cancelling={cancellingId === job.id}
            onCancel={() => {
              setCancellingId(job.id)
              void cancelJob(job.id).finally(() => setCancellingId(null))
            }}
            onDismiss={() => dismissJob(job.id)}
          />
        ))}
      </div>
    </section>
  )
}
