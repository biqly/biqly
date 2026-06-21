import { type DependencyList, useEffect, useState } from 'react'

import { errorMessage } from '../utils/error'

export interface UseFetchOptions {
  enabled?: boolean
}

export function useFetch<T>(
  fetcher: (signal: AbortSignal) => Promise<T>,
  deps: DependencyList = [],
  options?: UseFetchOptions,
) {
  const enabled = options?.enabled ?? true
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(enabled)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!enabled) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setData(null)
      setLoading(false)
      setError(null)
      return
    }

    const controller = new AbortController()
    let active = true

    const runFetch = async () => {
      setLoading(true)
      setError(null)
      try {
        const result = await fetcher(controller.signal)
        if (active) {
          setData(result)
        }
      } catch (err) {
        if (active) {
          if (err instanceof Error && err.name === 'AbortError') {
            return
          }
          setError(errorMessage(err))
        }
      } finally {
        if (active) {
          setLoading(false)
        }
      }
    }

    void runFetch()

    return () => {
      active = false
      controller.abort()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled])

  return {
    data,
    loading,
    error,
    setData,
  }
}
