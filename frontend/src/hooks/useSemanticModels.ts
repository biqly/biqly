import { useEffect, useState, useCallback } from 'react'
import { useApi } from './useApi'
import type { SemanticModelSummary } from '../types/semantic'

export function useSemanticModels(datasourceId: string | null, options?: { all?: boolean }) {
  const { get, loading, error } = useApi()
  const [models, setModels] = useState<SemanticModelSummary[]>([])

  const reload = useCallback(() => {
    if (options?.all) {
      get<SemanticModelSummary[]>('/api/semantic/models').then((data) => {
        setModels(data ?? [])
      })
      return
    }
    if (!datasourceId) {
      setModels([])
      return
    }
    get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`
    ).then((data) => {
      setModels(data ?? [])
    })
  }, [get, datasourceId, options?.all])

  useEffect(() => {
    reload()
  }, [reload])

  return { models, loading, error, reload, setModels }
}
