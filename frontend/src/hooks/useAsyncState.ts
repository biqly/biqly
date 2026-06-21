import { useCallback, useState } from 'react'

import { errorMessage } from '../utils/error'

export interface AsyncStateOptions {
  useSaving?: boolean
}

export function useAsyncState(options?: AsyncStateOptions) {
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const clearError = useCallback(() => setError(null), [])

  const run = useCallback(
    async <T>(fn: () => Promise<T>): Promise<T | null> => {
      const isSaving = options?.useSaving ?? false
      if (isSaving) {
        setSaving(true)
      } else {
        setLoading(true)
      }
      setError(null)
      try {
        return await fn()
      } catch (err) {
        setError(errorMessage(err))
        return null
      } finally {
        if (isSaving) {
          setSaving(false)
        } else {
          setLoading(false)
        }
      }
    },
    [options?.useSaving],
  )

  return {
    loading,
    saving,
    error,
    setLoading,
    setSaving,
    setError,
    clearError,
    run,
  }
}
