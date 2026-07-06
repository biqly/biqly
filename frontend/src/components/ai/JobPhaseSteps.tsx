import type { TrackedAIJob } from '../../hooks/useAIJobs'
import { jobIsActive } from '../../hooks/useAIJobs'
import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'
import {
  aiJobPipelineClass,
  aiJobPipelineDotClass,
  aiJobPipelineLabelClass,
  aiJobPipelineStepClass,
  aiJobPipelineTimeClassForState,
} from './aiJobsClasses'
import { formatJobDuration } from './jobProgressUtils'

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

function phaseLabelKey(job: TrackedAIJob, phase: PipelinePhase): TranslationKey {
  if ((job.kind === 'describe' || job.kind === 'describe_batch') && phase === 'generating') {
    return 'ai_jobs.phase_describing'
  }
  return `ai_jobs.phase_${phase}`
}

/** Live pipeline step list for a tracked AI job (per-phase durations tick while running). */
export function JobPhaseSteps({ job, now }: { job: TrackedAIJob; now: number }) {
  const t = useT()
  const phases = phasesForJob(job)
  const current = phaseIndex(phases, job.phase)
  const done = job.status === 'succeeded'
  const failed = job.status === 'failed'
  const active = jobIsActive(job)

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
        let duration: string | null = recorded != null ? formatJobDuration(recorded, t) : null
        if (state === 'current' && active && job.phaseEnteredAt != null && now > 0) {
          duration = formatJobDuration(now - job.phaseEnteredAt, t)
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
