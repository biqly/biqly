import { useCallback, useEffect, useState } from 'react'

import type { Datasource } from '../types/metadata'
import { useApi } from './useApi'

export function useDatasources() {
  const { get, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])

  const reload = useCallback(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      setDatasources(data ?? [])
    })
  }, [get])

  useEffect(() => {
    reload()
  }, [reload])

  return { datasources, loading, error, reload, setDatasources }
}
