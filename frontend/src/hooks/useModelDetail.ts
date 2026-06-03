import { useCallback, useEffect, useState } from 'react'

import type { SemanticModelDetail } from '../types/semantic'
import { useApi } from './useApi'

interface Options {
  includeInactive?: boolean
}

export function useModelDetail(modelId: string | null, options?: Options) {
  const { get, loading, error } = useApi()
  const [model, setModel] = useState<SemanticModelDetail | null>(null)

  const reload = useCallback(() => {
    if (!modelId) {
      setModel(null)
      return
    }
    const query = options?.includeInactive ? '?include_inactive=true' : ''
    get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(modelId)}${query}`).then(
      (data) => {
        setModel(data)
      },
    )
  }, [get, modelId, options?.includeInactive])

  useEffect(() => {
    reload()
  }, [reload])

  return { model, loading, error, reload, setModel }
}
