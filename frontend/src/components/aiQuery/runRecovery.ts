import type { RunStep } from '../../types/ai'

export interface RunRecovery {
  /** A step failed at some point during the run. */
  hadFailure: boolean
  /** The run recovered — a failure was followed by a successful continuation
   * (the agent replanned/retried rather than giving up). */
  recovered: boolean
  /** Highest retry attempt reached (0 when nothing retried). */
  maxAttempt: number
  /** Number of distinct planner passes — >1 means the agent replanned. */
  plannerPasses: number
  /** The run ended in a terminal failure. */
  failedTerminal: boolean
}

function isPlannerKind(kind: string): boolean {
  return kind === 'planner'
}

function isTerminalFailKind(kind: string): boolean {
  return kind === 'fail'
}

/**
 * Summarizes the retry/replan/recovery shape of a run from its step list, so
 * the UI can show "recovered after a failed attempt" instead of leaving the
 * user to infer it from scattered red rows (spec §12). Pure and defensive:
 * a running or empty trace simply reports no failure.
 */
export function analyzeRunRecovery(steps: RunStep[]): RunRecovery {
  let hadFailure = false
  let recovered = false
  let maxAttempt = 0
  let plannerPasses = 0
  let failedTerminal = false
  let seenFailure = false

  for (const step of steps) {
    if (typeof step.attempt === 'number') {
      maxAttempt = Math.max(maxAttempt, step.attempt)
    }
    if (isPlannerKind(step.kind)) {
      plannerPasses += 1
    }
    if (step.status === 'failed') {
      hadFailure = true
      seenFailure = true
      failedTerminal = isTerminalFailKind(step.kind)
    } else if (step.status === 'ok') {
      // A successful step after an earlier failure means the run kept going.
      if (seenFailure && !isTerminalFailKind(step.kind)) {
        recovered = true
      }
      failedTerminal = false
    }
  }

  // A run that ended on a terminal fail never "recovered", regardless of
  // intermediate successes.
  if (failedTerminal) {
    recovered = false
  }

  return { hadFailure, recovered, maxAttempt, plannerPasses, failedTerminal }
}

/** True when the recovery summary is worth surfacing as a banner. */
export function hasRecoveryStory(recovery: RunRecovery): boolean {
  return recovery.recovered || recovery.maxAttempt > 1 || recovery.plannerPasses > 1
}
