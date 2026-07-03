import { useCallback, useEffect, useRef, useState } from 'react'

import type { SemanticModelDetail } from '../types/semantic'
import { useApi } from './useApi'

interface Options {
  includeInactive?: boolean
}

export function useModelDetail(modelId: string | null, options?: Options) {
  const { get, loading, error } = useApi()
  const [model, setModel] = useState<SemanticModelDetail | null>(null)
  // Guards against out-of-order responses: when the caller switches models
  // quickly, a slower earlier response must not overwrite the newer one.
  const reqIdRef = useRef(0)

  const reload = useCallback(() => {
    const reqId = ++reqIdRef.current
    if (!modelId) {
      void Promise.resolve().then(() => {
        if (reqId === reqIdRef.current) {
          setModel(null)
        }
      })
      return
    }
    const query = options?.includeInactive ? '?include_inactive=true' : ''
    void get<SemanticModelDetail>(
      `/api/semantic/models/${encodeURIComponent(modelId)}${query}`,
    ).then((data) => {
      if (reqId === reqIdRef.current) {
        setModel(data)
      }
    })
  }, [get, modelId, options?.includeInactive])

  useEffect(() => {
    reload()
  }, [reload])

  return { model, loading, error, reload, setModel }
}
