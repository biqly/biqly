import { useEffect, useState } from 'react'

import { listUsers, listWorkspaces } from '../api/admin'
import { apiFetch } from '../api/apiClient'
import type { AuthUser, Workspace } from '../types/auth'
import type { Datasource } from '../types/metadata'
import { errorMessage } from '../utils/error'

export function useAdminLookups(token: string) {
  const [users, setUsers] = useState<AuthUser[]>([])
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      if (!token) {
        return
      }
      setLoading(true)
      try {
        const [uRes, dsData, wsRes] = await Promise.all([
          listUsers(token),
          apiFetch<Datasource[]>('GET', '/api/datasources', undefined, { token }),
          listWorkspaces(token),
        ])
        if (!cancelled) {
          setUsers(uRes.users)
          setDatasources(dsData)
          setWorkspaces(wsRes.workspaces)
          setError(null)
        }
      } catch (err) {
        if (!cancelled) {
          setError(errorMessage(err))
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [token])

  return { users, datasources, workspaces, loading, error }
}
