import type { AIJob } from '../types/ai'

export interface JobCallbacks<TResult = unknown> {
  /** Fired once the job row exists server-side (before completion). */
  onEnqueued?: (job: AIJob) => void
  onComplete?: (result: TResult) => void
  onError?: (message: string) => void
  parseResult?: (result: unknown) => TResult
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
  const parse = callbacks?.parseResult
  return {
    settleComplete: (result) =>
      once(() => {
        let value: TResult
        try {
          value = parse ? parse(result) : (result as TResult)
        } catch (err) {
          const message = err instanceof Error ? err.message : 'Invalid job result'
          callbacks?.onError?.(message)
          resolve(null)
          return
        }
        callbacks?.onComplete?.(value)
        resolve(value)
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
