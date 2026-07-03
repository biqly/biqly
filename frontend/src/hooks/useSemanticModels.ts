import { useCallback, useEffect, useRef, useState } from 'react'

import type { SemanticModelSummary } from '../types/semantic'
import { useApi } from './useApi'

export function useSemanticModels(datasourceId: string | null, options?: { all?: boolean }) {
  const { get, loading, error } = useApi()
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  // Guards against out-of-order responses when datasource changes quickly.
  const reqIdRef = useRef(0)

  const reload = useCallback(() => {
    const reqId = ++reqIdRef.current
    const apply = (data: SemanticModelSummary[] | null) => {
      if (reqId === reqIdRef.current) {
        setModels(data ?? [])
      }
    }
    if (options?.all) {
      void get<SemanticModelSummary[]>('/api/semantic/models').then(apply)
      return
    }
    if (!datasourceId) {
      void Promise.resolve().then(() => apply([]))
      return
    }
    void get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
    ).then(apply)
  }, [get, datasourceId, options?.all])

  useEffect(() => {
    reload()
  }, [reload])

  return { models, loading, error, reload, setModels }
}
