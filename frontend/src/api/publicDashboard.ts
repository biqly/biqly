import type { DashboardWidget } from '../components/dashboard/DashboardWidgetRenderer'
import type { QueryResultPayload } from '../types/ai'
import { apiFetch } from './apiClient'

export interface PublicDashboard {
  id: string
  name: string
  description?: string
  widgets: DashboardWidget[]
}

// token: '' forces anonymous requests: the public page must behave identically
// for signed-in and signed-out visitors, never leaking a globally set bearer
// token onto these endpoints.
export function getPublicDashboard(token: string): Promise<PublicDashboard> {
  return apiFetch<PublicDashboard>(
    'GET',
    `/api/public/dashboards/${encodeURIComponent(token)}`,
    undefined,
    { token: '' },
  )
}

export function runPublicWidget(token: string, widgetId: string): Promise<QueryResultPayload> {
  return apiFetch<QueryResultPayload>(
    'POST',
    `/api/public/widget-query/${encodeURIComponent(token)}/${encodeURIComponent(widgetId)}`,
    {},
    { token: '' },
  )
}
