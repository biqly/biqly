import { useMemo } from 'react'
import { useAIJobs, type TrackedAIJob } from '../hooks/useAIJobs'
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
  return job.kind === 'describe' ? DESCRIBE_PHASES : PIPELINE_PHASES
}

function phaseIndex(phases: readonly PipelinePhase[], phase: string): number {
  const i = phases.indexOf(phase as PipelinePhase)
  return i === -1 ? 0 : i
}

function phaseKey(phase: PipelinePhase): TranslationKey {
  return `ai_jobs.phase_${phase}` as TranslationKey
}

function phaseLabelKey(job: TrackedAIJob, phase: PipelinePhase): TranslationKey {
  if (job.kind === 'describe' && phase === 'generating') {
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
}: {
  job: TrackedAIJob
  expanded: boolean
  onDismiss: () => void
}) {
  const t = useT()
  const active = isActive(job)
  const kindLabel =
    job.kind === 'describe'
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
        {!active && (
          <button type="button" className="btn btn-sm btn-ghost" onClick={onDismiss} aria-label={t('ai_jobs.dismiss')}>
            ×
          </button>
        )}
      </header>
      {expanded && (
        <div className="ai-job-card__body">
          <JobPipeline job={job} />
          {job.status === 'failed' && job.error_message && (
            <p className="ai-job-card__error" role="alert">
              {job.error_message}
            </p>
          )}
          {job.status === 'succeeded' && <p className="ai-job-card__ok">{t('ai_jobs.completed')}</p>}
        </div>
      )}
    </article>
  )
}

export default function AIJobTracker() {
  const t = useT()
  const { jobs, expanded, setExpanded, minimized, setMinimized, dismissJob } = useAIJobs()

  const visible = useMemo(() => jobs.filter((j) => isActive(j) || j.status === 'failed'), [jobs])
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
      <div className="ai-job-panel__list">
        {(expanded ? jobs : jobs.slice(0, 1)).map((job) => (
          <JobCard
            key={job.id}
            job={job}
            expanded={expanded || jobs.length === 1}
            onDismiss={() => dismissJob(job.id)}
          />
        ))}
      </div>
    </section>
  )
}
