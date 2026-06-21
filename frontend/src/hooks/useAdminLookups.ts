import { listUsers, listWorkspaces } from '../api/admin'
import { apiFetch } from '../api/apiClient'
import type { Datasource } from '../types/metadata'
import { useFetch } from './useFetch'

export function useAdminLookups(token: string) {
  const { data, loading, error } = useFetch(
    async () => {
      const [uRes, dsData, wsRes] = await Promise.all([
        listUsers(token),
        apiFetch<Datasource[]>('GET', '/api/datasources', undefined, { token }),
        listWorkspaces(token),
      ])
      return {
        users: uRes.users,
        datasources: dsData,
        workspaces: wsRes.workspaces,
      }
    },
    [token],
    { enabled: !!token },
  )

  return {
    users: data?.users ?? [],
    datasources: data?.datasources ?? [],
    workspaces: data?.workspaces ?? [],
    loading,
    error,
  }
}
