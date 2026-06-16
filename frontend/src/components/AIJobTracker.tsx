import { useCallback, useEffect, useMemo, useState } from 'react'

import { jobQuestionPreview, type TrackedAIJob, useAIJobs } from '../hooks/useAIJobs'
import { type TranslationKey, useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { cn } from '../lib/cn'
import type { AIQueueStatus } from '../types/ai'
import {
  aiJobCardActionsClass,
  aiJobCardBodyClass,
  aiJobCardCancelClass,
  aiJobCardCancelledClass,
  aiJobCardChevronClass,
  aiJobCardClass,
  aiJobCardErrorClass,
  aiJobCardHeadClass,
  aiJobCardHintClass,
  aiJobCardMetaClass,
  aiJobCardProgressClass,
  aiJobCardProgressFillClass,
  aiJobCardTitleClass,
  aiJobCardTitlesClass,
  aiJobCardToggleClass,
  aiJobCardTotalClass,
  aiJobCardTotalTimeClass,
  aiJobFabAlertClass,
  aiJobFabClass,
  aiJobFabPctClass,
  aiJobFabPulseAlertClass,
  aiJobFabPulseClass,
  aiJobFabPulseIdleClass,
  aiJobPanelActionsClass,
  aiJobPanelClass,
  aiJobPanelHeadClass,
  aiJobPanelListClass,
  aiJobPanelManageClass,
  aiJobPanelStaleClass,
  aiJobPanelStaleEmptyClass,
  aiJobPanelStaleHeadClass,
  aiJobPanelStaleItemClass,
  aiJobPanelStaleListClass,
  aiJobPanelSubClass,
  aiJobPanelTitleClass,
  aiJobPipelineClass,
  aiJobPipelineDotClass,
  aiJobPipelineLabelClass,
  aiJobPipelineStepClass,
  aiJobPipelineTimeClassForState,
} from './ai/aiJobsClasses'
import { jobKindLabel, queuePositionLine } from './AIJobTrackerUtils'

const PIPELINE_PHASES = [
  'queued',
  'routing',
  'generating',
  'validating',
  'compiling',
  'executing',
] as const

const DESCRIBE_PHASES = ['queued', 'sampling', 'generating', 'applying'] as const

const EMBED_PHASES = ['queued', 'fetching', 'embedding', 'completing'] as const

type PipelinePhase =
  | (typeof PIPELINE_PHASES)[number]
  | (typeof DESCRIBE_PHASES)[number]
  | (typeof EMBED_PHASES)[number]

type Translate = (key: TranslationKey, params?: Record<string, string | number>) => string

function phasesForJob(job: TrackedAIJob): readonly PipelinePhase[] {
  if (job.kind === 'describe' || job.kind === 'describe_batch') {
    return DESCRIBE_PHASES
  }
  if (job.kind === 'embed_metadata') {
    return EMBED_PHASES
  }
  return PIPELINE_PHASES
}

function phaseIndex(phases: readonly PipelinePhase[], phase: string): number {
  const i = phases.indexOf(phase as PipelinePhase)
  return i === -1 ? 0 : i
}

function phaseKey(phase: PipelinePhase): TranslationKey {
  return `ai_jobs.phase_${phase}`
}

function phaseLabelKey(job: TrackedAIJob, phase: PipelinePhase): TranslationKey {
  if ((job.kind === 'describe' || job.kind === 'describe_batch') && phase === 'generating') {
    return 'ai_jobs.phase_describing'
  }
  return phaseKey(phase)
}

function isActive(job: TrackedAIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

/** Ticking clock for live durations; updates only while a job is active. */
function useNowWhile(active: boolean): number {
  const [now, setNow] = useState(0)
  useEffect(() => {
    if (!active) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNow(Date.now())
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [active])
  return now
}

function formatDuration(ms: number, t: Translate): string {
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

function jobTotalMs(job: TrackedAIJob, now: number): number | null {
  // From created_at, not started_at, so queue wait is included and the total
  // matches the per-step breakdown (which starts at "queued").
  const start = Date.parse(job.created_at)
  if (Number.isNaN(start)) {
    return null
  }
  if (isActive(job)) {
    return now > 0 ? now - start : null
  }
  const end = Date.parse(job.finished_at ?? job.updated_at)
  return Number.isNaN(end) ? null : end - start
}

function jobStatusKey(job: TrackedAIJob): TranslationKey | null {
  if (job.status === 'succeeded') {
    return 'ai_jobs.completed'
  }
  if (job.status === 'failed') {
    return 'ai_jobs.failed'
  }
  if (job.status === 'cancelled') {
    return 'ai_jobs.cancelled'
  }
  return null
}

function describeBatchScopeLine(job: TrackedAIJob): string | null {
  if (job.kind !== 'describe_batch' || !job.scope_schemas?.length) {
    return null
  }
  return job.scope_schemas.join(', ')
}

function describeBatchQueueLine(
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

function jobCardModifier(job: TrackedAIJob): '' | 'active' | 'failed' | 'done' {
  if (isActive(job)) {
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

function JobPipeline({ job, now }: { job: TrackedAIJob; now: number }) {
  const t = useT()
  const phases = phasesForJob(job)
  const current = phaseIndex(phases, job.phase)
  const done = job.status === 'succeeded'
  const failed = job.status === 'failed'
  const active = isActive(job)

  return (
    <ol className={aiJobPipelineClass} aria-label={t('ai_jobs.pipeline_aria')}>
      {phases.map((phase, idx) => {
        let state: 'done' | 'current' | 'pending' | 'failed' = 'pending'
        if (failed && idx === current) {
          state = 'failed'
        } else if (done || idx < current) {
          state = 'done'
        } else if (idx === current) {
          state = 'current'
        }
        const recorded = job.phaseTimings?.[phase]
        let duration: string | null = recorded != null ? formatDuration(recorded, t) : null
        if (state === 'current' && active && job.phaseEnteredAt != null && now > 0) {
          duration = formatDuration(now - job.phaseEnteredAt, t)
        }
        return (
          <li key={phase} className={aiJobPipelineStepClass(state)}>
            <span className={aiJobPipelineDotClass(state)} aria-hidden="true" />
            <span className={aiJobPipelineLabelClass}>{t(phaseLabelKey(job, phase))}</span>
            {duration && <span className={aiJobPipelineTimeClassForState(state)}>{duration}</span>}
          </li>
        )
      })}
    </ol>
  )
}

function JobCardBody({
  job,
  now,
  queueStatus,
}: {
  job: TrackedAIJob
  now: number
  queueStatus: AIQueueStatus | null
}) {
  const t = useT()
  const queueLine = describeBatchQueueLine(job)
  const scopeLine = describeBatchScopeLine(job)
  const queuePosition = queuePositionLine(job, queueStatus, t)
  const active = isActive(job)
  const total = jobTotalMs(job, now)

  return (
    <div className={aiJobCardBodyClass}>
      <JobPipeline job={job} now={now} />
      {total != null && (
        <p className={aiJobCardTotalClass}>
          <span>{t('ai_jobs.total_label')}</span>
          <span className={aiJobCardTotalTimeClass}>{formatDuration(total, t)}</span>
        </p>
      )}
      {scopeLine && (
        <p className={aiJobCardHintClass}>{t('ai_jobs.scope_schemas', { schemas: scopeLine })}</p>
      )}
      {job.phase_message && active && <p className={aiJobCardHintClass}>{job.phase_message}</p>}
      {queueLine?.current && (
        <p className={aiJobCardHintClass}>
          {t('ai_jobs.queue_current', { table: queueLine.current })}
        </p>
      )}
      {queueLine?.next && (
        <p className={aiJobCardHintClass}>{t('ai_jobs.queue_next', { tables: queueLine.next })}</p>
      )}
      {queuePosition && <p className={aiJobCardHintClass}>{queuePosition}</p>}
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
    </div>
  )
}

function JobCard({
  job,
  now,
  open,
  onToggle,
  onDismiss,
  onCancel,
  cancelling,
  queueStatus,
}: {
  job: TrackedAIJob
  now: number
  open: boolean
  onToggle: () => void
  onDismiss: () => void
  onCancel?: () => void
  cancelling?: boolean
  queueStatus: AIQueueStatus | null
}) {
  const t = useT()
  const active = isActive(job)
  const kindLabel = jobKindLabel(job, t)
  const statusKey = jobStatusKey(job)
  const total = jobTotalMs(job, now)

  const metaParts = [kindLabel]
  if (active) {
    metaParts.push(`${job.progress_pct}%`)
  } else if (statusKey) {
    metaParts.push(t(statusKey))
  }
  if (total != null) {
    metaParts.push(formatDuration(total, t))
  }

  return (
    <article className={aiJobCardClass(jobCardModifier(job))}>
      <header className={aiJobCardHeadClass}>
        <button
          type="button"
          className={aiJobCardToggleClass}
          onClick={onToggle}
          aria-expanded={open}
        >
          <span className={aiJobCardChevronClass} aria-hidden="true" />
          <span className={aiJobCardTitlesClass}>
            <strong className={aiJobCardTitleClass}>{job.questionPreview ?? kindLabel}</strong>
            <span className={aiJobCardMetaClass}>{metaParts.join(' · ')}</span>
          </span>
        </button>
        <div className={aiJobCardActionsClass}>
          {active && onCancel && (
            <button
              type="button"
              className={cn(legacyButtonClass('btn btn-sm btn-ghost'), aiJobCardCancelClass)}
              onClick={onCancel}
              disabled={cancelling}
            >
              {cancelling ? t('ai_jobs.cancelling') : t('ai_jobs.cancel')}
            </button>
          )}
          {!active && (
            <button
              type="button"
              className={legacyButtonClass('btn btn-sm btn-ghost')}
              onClick={onDismiss}
              aria-label={t('ai_jobs.dismiss')}
            >
              ×
            </button>
          )}
        </div>
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
      {open && <JobCardBody job={job} now={now} queueStatus={queueStatus} />}
    </article>
  )
}

function StaleJobsPanel({
  staleJobs,
  onCancelIds,
  onCancelAll,
  busy,
}: {
  staleJobs: TrackedAIJob[]
  onCancelIds: (ids: string[]) => void
  onCancelAll: () => void
  busy: boolean
}) {
  const t = useT()
  if (!staleJobs.length) {
    return <p className={aiJobPanelStaleEmptyClass}>{t('ai_jobs.manage_stale_empty')}</p>
  }
  return (
    <div className={aiJobPanelStaleClass}>
      <p className={aiJobPanelStaleHeadClass}>
        {t('ai_jobs.manage_stale_count', { count: staleJobs.length })}
      </p>
      <ul className={aiJobPanelStaleListClass}>
        {staleJobs.map((job) => (
          <li key={job.id} className={aiJobPanelStaleItemClass}>
            <span>
              {job.questionPreview ?? job.kind} · {job.status} · {job.progress_pct}%
            </span>
            <button
              type="button"
              className={legacyButtonClass('btn btn-sm btn-ghost')}
              disabled={busy}
              onClick={() => onCancelIds([job.id])}
            >
              {t('ai_jobs.cancel')}
            </button>
          </li>
        ))}
      </ul>
      <button
        type="button"
        className={legacyButtonClass('btn btn-sm btn-ghost')}
        disabled={busy}
        onClick={onCancelAll}
      >
        {t('ai_jobs.manage_cancel_all_stale')}
      </button>
    </div>
  )
}

export default function AIJobTracker() {
  const t = useT()
  const {
    jobs,
    expanded,
    setExpanded,
    minimized,
    setMinimized,
    dismissJob,
    dismissFinishedJobs,
    cancelJob,
    cancelAllActiveJobs,
    listStaleJobs,
    cancelJobIds,
    queueStatus,
  } = useAIJobs()
  const [cancellingId, setCancellingId] = useState<string | null>(null)
  const [manageBusy, setManageBusy] = useState(false)
  const [showStale, setShowStale] = useState(false)
  const [staleJobs, setStaleJobs] = useState<TrackedAIJob[]>([])
  const [cardOverrides, setCardOverrides] = useState<Record<string, boolean>>({})

  const loadStale = useCallback(async () => {
    const list = await listStaleJobs(15)
    setStaleJobs(
      list.map((job) => ({
        ...job,
        questionPreview:
          job.request_json && typeof job.request_json === 'object'
            ? jobQuestionPreview(job.kind, job.request_json)
            : job.kind,
      })),
    )
  }, [listStaleJobs])

  const toggleStale = useCallback(async () => {
    if (showStale) {
      setShowStale(false)
      return
    }
    setShowStale(true)
    await loadStale()
  }, [showStale, loadStale])

  const activeCount = useMemo(() => jobs.filter(isActive).length, [jobs])
  const failedCount = useMemo(() => jobs.filter((j) => j.status === 'failed').length, [jobs])
  const finishedCount = jobs.length - activeCount
  const primary = jobs.find(isActive) ?? jobs[0]
  const now = useNowWhile(activeCount > 0 && !minimized)

  if (!jobs.length) {
    return null
  }

  if (minimized) {
    const pulseClass = [
      aiJobFabPulseClass,
      failedCount > 0 ? aiJobFabPulseAlertClass : activeCount === 0 ? aiJobFabPulseIdleClass : '',
    ]
      .filter(Boolean)
      .join(' ')

    return (
      <button
        type="button"
        className={cn(aiJobFabClass, failedCount > 0 && aiJobFabAlertClass)}
        onClick={() => setMinimized(false)}
        aria-expanded="false"
        aria-label={t('ai_jobs.fab_aria', { count: activeCount })}
      >
        <span className={pulseClass} aria-hidden="true" />
        <span>
          {activeCount > 0
            ? t('ai_jobs.fab_running', { count: activeCount })
            : t('ai_jobs.fab_done')}
        </span>
        {activeCount > 0 && primary && (
          <span className={aiJobFabPctClass}>{primary.progress_pct}%</span>
        )}
      </button>
    )
  }

  return (
    <section
      className={aiJobPanelClass}
      aria-live="polite"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          setMinimized(true)
        }
      }}
    >
      <header className={aiJobPanelHeadClass}>
        <div>
          <h2 className={aiJobPanelTitleClass}>{t('ai_jobs.panel_title')}</h2>
          <p className={aiJobPanelSubClass}>
            {activeCount > 0
              ? t('ai_jobs.panel_sub_active', { count: activeCount })
              : t('ai_jobs.panel_sub_idle')}
          </p>
        </div>
        <div className={aiJobPanelActionsClass}>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
          >
            {expanded ? t('ai_jobs.tools_hide') : t('ai_jobs.tools')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            onClick={() => setMinimized(true)}
            aria-label={t('ai_jobs.minimize')}
            title={t('ai_jobs.minimize')}
          >
            —
          </button>
        </div>
      </header>
      {expanded && (
        <div className={aiJobPanelManageClass}>
          {activeCount > 0 && (
            <button
              type="button"
              className={legacyButtonClass('btn btn-sm btn-ghost')}
              disabled={manageBusy}
              onClick={() => {
                setManageBusy(true)
                void cancelAllActiveJobs().finally(() => setManageBusy(false))
              }}
            >
              {t('ai_jobs.manage_cancel_all_active')}
            </button>
          )}
          {finishedCount > 0 && (
            <button
              type="button"
              className={legacyButtonClass('btn btn-sm btn-ghost')}
              disabled={manageBusy}
              onClick={dismissFinishedJobs}
            >
              {t('ai_jobs.clear_done')}
            </button>
          )}
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-ghost')}
            disabled={manageBusy}
            onClick={() => {
              setManageBusy(true)
              void toggleStale().finally(() => setManageBusy(false))
            }}
          >
            {showStale ? t('ai_jobs.manage_stale_hide') : t('ai_jobs.manage_stale')}
          </button>
          {showStale && (
            <StaleJobsPanel
              staleJobs={staleJobs}
              busy={manageBusy}
              onCancelIds={(ids) => {
                setManageBusy(true)
                void cancelJobIds(ids)
                  .then(() => loadStale())
                  .finally(() => setManageBusy(false))
              }}
              onCancelAll={() => {
                setManageBusy(true)
                void cancelJobIds(staleJobs.map((j) => j.id))
                  .then(() => loadStale())
                  .finally(() => setManageBusy(false))
              }}
            />
          )}
        </div>
      )}
      <div className={aiJobPanelListClass}>
        {jobs.map((job) => (
          <JobCard
            key={job.id}
            job={job}
            now={now}
            open={cardOverrides[job.id] ?? (isActive(job) || job.status === 'failed')}
            onToggle={() =>
              setCardOverrides((prev) => ({
                ...prev,
                [job.id]: !(prev[job.id] ?? (isActive(job) || job.status === 'failed')),
              }))
            }
            onDismiss={() => dismissJob(job.id)}
            onCancel={
              isActive(job)
                ? () => {
                    setCancellingId(job.id)
                    void cancelJob(job.id).finally(() => setCancellingId(null))
                  }
                : undefined
            }
            cancelling={cancellingId === job.id}
            queueStatus={queueStatus}
          />
        ))}
      </div>
    </section>
  )
}
