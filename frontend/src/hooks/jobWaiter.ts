export type JobCallbacks<TResult = unknown> = {
  onComplete?: (result: TResult) => void
  onError?: (message: string) => void
}

export type JobWaiterHandle = {
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
    if (settled) return
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
