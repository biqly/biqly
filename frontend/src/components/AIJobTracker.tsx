import '../styles/ai-jobs.css'

import { useCallback, useEffect, useMemo, useState } from 'react'

import { jobQuestionPreview, type TrackedAIJob, useAIJobs } from '../hooks/useAIJobs'
import { type TranslationKey, useT } from '../i18n'
import { jobKindLabel } from './AIJobTrackerUtils'

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
  const start = Date.parse(job.started_at ?? job.created_at)
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

function JobPipeline({ job, now }: { job: TrackedAIJob; now: number }) {
  const t = useT()
  const phases = phasesForJob(job)
  const current = phaseIndex(phases, job.phase)
  const done = job.status === 'succeeded'
  const failed = job.status === 'failed'
  const active = isActive(job)

  return (
    <ol className="ai-job-pipeline" aria-label={t('ai_jobs.pipeline_aria')}>
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
          <li key={phase} className={`ai-job-pipeline__step ai-job-pipeline__step--${state}`}>
            <span className="ai-job-pipeline__dot" aria-hidden="true" />
            <span className="ai-job-pipeline__label">{t(phaseLabelKey(job, phase))}</span>
            {duration && <span className="ai-job-pipeline__time">{duration}</span>}
          </li>
        )
      })}
    </ol>
  )
}

function JobCardBody({ job, now }: { job: TrackedAIJob; now: number }) {
  const t = useT()
  const queueLine = describeBatchQueueLine(job)
  const scopeLine = describeBatchScopeLine(job)
  const active = isActive(job)
  const total = jobTotalMs(job, now)

  return (
    <div className="ai-job-card__body">
      <JobPipeline job={job} now={now} />
      {total != null && (
        <p className="ai-job-card__total">
          <span>{t('ai_jobs.total_label')}</span>
          <span className="ai-job-card__total-time">{formatDuration(total, t)}</span>
        </p>
      )}
      {scopeLine && (
        <p className="ai-job-card__hint">{t('ai_jobs.scope_schemas', { schemas: scopeLine })}</p>
      )}
      {job.phase_message && active && <p className="ai-job-card__hint">{job.phase_message}</p>}
      {queueLine?.current && (
        <p className="ai-job-card__hint">
          {t('ai_jobs.queue_current', { table: queueLine.current })}
        </p>
      )}
      {queueLine?.next && (
        <p className="ai-job-card__hint">{t('ai_jobs.queue_next', { tables: queueLine.next })}</p>
      )}
      {job.status === 'failed' && job.error_message && (
        <p className="ai-job-card__error" role="alert">
          {job.error_message}
        </p>
      )}
      {job.status === 'cancelled' && (
        <p className="ai-job-card__cancelled">
          {(job.error_message ?? job.phase_message) || t('ai_jobs.cancelled')}
        </p>
      )}
      {active && job.status === 'queued' && (
        <p className="ai-job-card__hint">{t('ai_jobs.stuck_hint')}</p>
      )}
    </div>
  )
}

function jobCardModifier(job: TrackedAIJob): string {
  if (isActive(job)) {
    return ' ai-job-card--active'
  }
  if (job.status === 'failed') {
    return ' ai-job-card--failed'
  }
  if (job.status === 'succeeded') {
    return ' ai-job-card--done'
  }
  return ''
}

function JobCard({
  job,
  now,
  open,
  onToggle,
  onDismiss,
  onCancel,
  cancelling,
}: {
  job: TrackedAIJob
  now: number
  open: boolean
  onToggle: () => void
  onDismiss: () => void
  onCancel?: () => void
  cancelling?: boolean
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
    <article className={`ai-job-card${jobCardModifier(job)}`}>
      <header className="ai-job-card__head">
        <button
          type="button"
          className="ai-job-card__toggle"
          onClick={onToggle}
          aria-expanded={open}
        >
          <span className="ai-job-card__chevron" aria-hidden="true" />
          <span className="ai-job-card__titles">
            <strong className="ai-job-card__title">{job.questionPreview ?? kindLabel}</strong>
            <span className="ai-job-card__meta">{metaParts.join(' · ')}</span>
          </span>
        </button>
        <div className="ai-job-card__actions">
          {active && onCancel && (
            <button
              type="button"
              className="btn btn-sm btn-ghost ai-job-card__cancel"
              onClick={onCancel}
              disabled={cancelling}
            >
              {cancelling ? t('ai_jobs.cancelling') : t('ai_jobs.cancel')}
            </button>
          )}
          {!active && (
            <button
              type="button"
              className="btn btn-sm btn-ghost"
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
          className="ai-job-card__progress"
          role="progressbar"
          aria-valuenow={job.progress_pct}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          <span style={{ width: `${Math.min(100, Math.max(0, job.progress_pct))}%` }} />
        </div>
      )}
      {open && <JobCardBody job={job} now={now} />}
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
    return <p className="ai-job-panel__stale-empty">{t('ai_jobs.manage_stale_empty')}</p>
  }
  return (
    <div className="ai-job-panel__stale">
      <p className="ai-job-panel__stale-head">
        {t('ai_jobs.manage_stale_count', { count: staleJobs.length })}
      </p>
      <ul className="ai-job-panel__stale-list">
        {staleJobs.map((job) => (
          <li key={job.id} className="ai-job-panel__stale-item">
            <span>
              {job.questionPreview ?? job.kind} · {job.status} · {job.progress_pct}%
            </span>
            <button
              type="button"
              className="btn btn-sm btn-ghost"
              disabled={busy}
              onClick={() => onCancelIds([job.id])}
            >
              {t('ai_jobs.cancel')}
            </button>
          </li>
        ))}
      </ul>
      <button type="button" className="btn btn-sm btn-ghost" disabled={busy} onClick={onCancelAll}>
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
    return (
      <button
        type="button"
        className={`ai-job-fab${failedCount > 0 ? ' ai-job-fab--alert' : ''}`}
        onClick={() => setMinimized(false)}
        aria-expanded="false"
        aria-label={t('ai_jobs.fab_aria', { count: activeCount })}
      >
        <span
          className={`ai-job-fab__pulse${activeCount === 0 ? ' ai-job-fab__pulse--idle' : ''}`}
          aria-hidden="true"
        />
        <span className="ai-job-fab__label">
          {activeCount > 0
            ? t('ai_jobs.fab_running', { count: activeCount })
            : t('ai_jobs.fab_done')}
        </span>
        {activeCount > 0 && primary && (
          <span className="ai-job-fab__pct">{primary.progress_pct}%</span>
        )}
      </button>
    )
  }

  return (
    <section
      className="ai-job-panel"
      aria-live="polite"
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          setMinimized(true)
        }
      }}
    >
      <header className="ai-job-panel__head">
        <div>
          <h2 className="ai-job-panel__title">{t('ai_jobs.panel_title')}</h2>
          <p className="ai-job-panel__sub">
            {activeCount > 0
              ? t('ai_jobs.panel_sub_active', { count: activeCount })
              : t('ai_jobs.panel_sub_idle')}
          </p>
        </div>
        <div className="ai-job-panel__actions">
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
          >
            {expanded ? t('ai_jobs.tools_hide') : t('ai_jobs.tools')}
          </button>
          <button
            type="button"
            className="btn btn-sm btn-ghost ai-job-panel__minimize"
            onClick={() => setMinimized(true)}
            aria-label={t('ai_jobs.minimize')}
            title={t('ai_jobs.minimize')}
          >
            —
          </button>
        </div>
      </header>
      {expanded && (
        <div className="ai-job-panel__manage">
          {activeCount > 0 && (
            <button
              type="button"
              className="btn btn-sm btn-ghost"
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
              className="btn btn-sm btn-ghost"
              disabled={manageBusy}
              onClick={dismissFinishedJobs}
            >
              {t('ai_jobs.clear_done')}
            </button>
          )}
          <button
            type="button"
            className="btn btn-sm btn-ghost"
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
      <div className="ai-job-panel__list">
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
          />
        ))}
      </div>
    </section>
  )
}
