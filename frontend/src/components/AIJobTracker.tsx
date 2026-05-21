import { useCallback, useMemo, useState } from 'react'
import { jobQuestionPreview, useAIJobs, type TrackedAIJob } from '../hooks/useAIJobs'
import type { AIJobKind } from '../types/ai'
import { useT, type TranslationKey } from '../i18n'

const PIPELINE_PHASES = [
  'queued',
  'routing',
  'generating',
  'validating',
  'compiling',
  'executing',
] as const

const DESCRIBE_PHASES = ['queued', 'sampling', 'generating', 'applying'] as const

type PipelinePhase = (typeof PIPELINE_PHASES)[number] | (typeof DESCRIBE_PHASES)[number]

function phasesForJob(job: TrackedAIJob): readonly PipelinePhase[] {
  return job.kind === 'describe' || job.kind === 'describe_batch' ? DESCRIBE_PHASES : PIPELINE_PHASES
}

function phaseIndex(phases: readonly PipelinePhase[], phase: string): number {
  const i = phases.indexOf(phase as PipelinePhase)
  return i === -1 ? 0 : i
}

function phaseKey(phase: PipelinePhase): TranslationKey {
  return `ai_jobs.phase_${phase}` as TranslationKey
}

function phaseLabelKey(job: TrackedAIJob, phase: PipelinePhase): TranslationKey {
  if ((job.kind === 'describe' || job.kind === 'describe_batch') && phase === 'generating') {
    return 'ai_jobs.phase_describing' as TranslationKey
  }
  return phaseKey(phase)
}

function isActive(job: TrackedAIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

function JobPipeline({ job }: { job: TrackedAIJob }) {
  const t = useT()
  const phases = phasesForJob(job)
  const current = phaseIndex(phases, job.phase)
  const done = job.status === 'succeeded'
  const failed = job.status === 'failed'

  return (
    <ol className="ai-job-pipeline" aria-label={t('ai_jobs.pipeline_aria')}>
      {phases.map((phase, idx) => {
        let state: 'done' | 'current' | 'pending' | 'failed' = 'pending'
        if (failed && idx === current) state = 'failed'
        else if (done || idx < current) state = 'done'
        else if (idx === current) state = 'current'
        return (
          <li key={phase} className={`ai-job-pipeline__step ai-job-pipeline__step--${state}`}>
            <span className="ai-job-pipeline__dot" aria-hidden="true" />
            <span className="ai-job-pipeline__label">{t(phaseLabelKey(job, phase))}</span>
          </li>
        )
      })}
    </ol>
  )
}

function JobCard({
  job,
  expanded,
  onDismiss,
  onCancel,
  cancelling,
}: {
  job: TrackedAIJob
  expanded: boolean
  onDismiss: () => void
  onCancel?: () => void
  cancelling?: boolean
}) {
  const t = useT()
  const active = isActive(job)
  const kindLabel =
    job.kind === 'describe_batch'
      ? t('ai_jobs.kind_describe_batch')
      : job.kind === 'describe'
      ? t('ai_jobs.kind_describe')
      : job.kind === 'run'
      ? t('ai_jobs.kind_run')
      : job.kind === 'preview'
        ? t('ai_jobs.kind_preview')
        : t('ai_jobs.kind_query')

  return (
    <article className={`ai-job-card${active ? ' ai-job-card--active' : ''}`}>
      <header className="ai-job-card__head">
        <div className="ai-job-card__titles">
          <strong className="ai-job-card__title">{job.questionPreview || kindLabel}</strong>
          <span className="ai-job-card__meta">
            {kindLabel} · {job.progress_pct}%
            {job.phase_message ? ` · ${job.phase_message}` : ''}
          </span>
        </div>
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
            <button type="button" className="btn btn-sm btn-ghost" onClick={onDismiss} aria-label={t('ai_jobs.dismiss')}>
              ×
            </button>
          )}
        </div>
      </header>
      {expanded && (
        <div className="ai-job-card__body">
          <JobPipeline job={job} />
          {job.status === 'failed' && job.error_message && (
            <p className="ai-job-card__error" role="alert">
              {job.error_message}
            </p>
          )}
          {job.status === 'cancelled' && (
            <p className="ai-job-card__cancelled">{job.error_message || job.phase_message || t('ai_jobs.cancelled')}</p>
          )}
          {job.status === 'succeeded' && <p className="ai-job-card__ok">{t('ai_jobs.completed')}</p>}
          {active && job.status === 'queued' && (
            <p className="ai-job-card__hint">{t('ai_jobs.stuck_hint')}</p>
          )}
        </div>
      )}
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
      <p className="ai-job-panel__stale-head">{t('ai_jobs.manage_stale_count', { count: staleJobs.length })}</p>
      <ul className="ai-job-panel__stale-list">
        {staleJobs.map((job) => (
          <li key={job.id} className="ai-job-panel__stale-item">
            <span>
              {job.questionPreview || job.kind} · {job.status} · {job.progress_pct}%
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
    cancelJob,
    cancelAllActiveJobs,
    listStaleJobs,
    cancelJobIds,
  } = useAIJobs()
  const [cancellingId, setCancellingId] = useState<string | null>(null)
  const [manageBusy, setManageBusy] = useState(false)
  const [showStale, setShowStale] = useState(false)
  const [staleJobs, setStaleJobs] = useState<TrackedAIJob[]>([])

  const loadStale = useCallback(async () => {
    const list = await listStaleJobs(15)
    setStaleJobs(
      list.map((job) => ({
        ...job,
        questionPreview:
          job.request_json && typeof job.request_json === 'object'
            ? jobQuestionPreview(job.kind as AIJobKind, job.request_json)
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

  const visible = useMemo(
    () => jobs.filter((j) => isActive(j) || j.status === 'failed' || j.status === 'cancelled'),
    [jobs],
  )
  const primary = visible[0] ?? jobs[0]

  if (!jobs.length) return null

  const activeCount = jobs.filter(isActive).length

  if (minimized) {
    return (
      <button
        type="button"
        className="ai-job-fab"
        onClick={() => {
          setMinimized(false)
          setExpanded(false)
        }}
        aria-expanded="false"
        aria-label={t('ai_jobs.fab_aria', { count: activeCount })}
      >
        <span className="ai-job-fab__pulse" aria-hidden="true" />
        <span className="ai-job-fab__label">
          {activeCount > 0 ? t('ai_jobs.fab_running', { count: activeCount }) : t('ai_jobs.fab_done')}
        </span>
        {primary && <span className="ai-job-fab__pct">{primary.progress_pct}%</span>}
      </button>
    )
  }

  return (
    <section className="ai-job-panel" aria-live="polite">
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
            {expanded ? t('ai_jobs.collapse') : t('ai_jobs.expand')}
          </button>
          <button
            type="button"
            className="btn btn-sm btn-ghost"
            onClick={() => setMinimized(true)}
            aria-label={t('ai_jobs.minimize')}
          >
            _
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
        {(expanded ? jobs : jobs.slice(0, 1)).map((job) => (
          <JobCard
            key={job.id}
            job={job}
            expanded={expanded || jobs.length === 1}
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
