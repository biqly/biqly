import type { AIJob } from '../types/ai'

export interface JobCallbacks<TResult = unknown> {
  /** Fired once the job row exists server-side (before completion). */
  onEnqueued?: (job: AIJob) => void
  onComplete?: (result: TResult) => void
  onError?: (message: string) => void
}

export interface JobWaiterHandle {
  settleComplete: (result: unknown) => void
  settleError: (message: string) => void
  settleDismiss: () => void
}

export function createJobWaiter<TResult>(
  resolve: (value: TResult | null) => void,
  callbacks?: JobCallbacks<TResult>,
): JobWaiterHandle {
  let settled = false
  const once = (run: () => void) => {
    if (settled) {
      return
    }
    settled = true
    run()
  }
  return {
    settleComplete: (result) =>
      once(() => {
        callbacks?.onComplete?.(result as TResult)
        resolve(result as TResult)
      }),
    settleError: (message) =>
      once(() => {
        callbacks?.onError?.(message)
        resolve(null)
      }),
    settleDismiss: () =>
      once(() => {
        resolve(null)
      }),
  }
}
