import { apiFetch } from './apiClient'

export interface PublicShareStatus {
  active: boolean
  created_at?: string
  expires_at?: string | null
}

export interface CreatedPublicShare {
  token: string
  url_path: string
  created_at: string
}

// Authenticated endpoints: no `token: ''` override, so these use the global
// bearer token set by AuthProvider (only a signed-in dashboard editor may
// manage a share), unlike the anonymous publicDashboard.ts client.
export function getDashboardPublicShare(id: string): Promise<PublicShareStatus> {
  return apiFetch<PublicShareStatus>(
    'GET',
    `/api/dashboards/${encodeURIComponent(id)}/public-share`,
  )
}

export function createDashboardPublicShare(id: string): Promise<CreatedPublicShare> {
  return apiFetch<CreatedPublicShare>(
    'POST',
    `/api/dashboards/${encodeURIComponent(id)}/public-share`,
    {},
  )
}

export function revokeDashboardPublicShare(id: string): Promise<void> {
  return apiFetch<void>('DELETE', `/api/dashboards/${encodeURIComponent(id)}/public-share`)
}
