import { useEffect, useState, useCallback } from 'react'
import { useApi } from './useApi'
import type { Datasource } from '../types/metadata'

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
