import { buildQueryString } from '../utils/query'
import { apiFetch } from './apiClient'

export interface QueryAuditEvent {
  id: string
  user_id: string
  event_type: 'query.executed' | 'query.failed'
  datasource_id: string
  model_id: string
  details?: QueryAuditDetails
  timestamp: string
}

export interface QueryAuditRowFilter {
  field: string
  operator: string
  value?: unknown
}

export interface QueryAuditDetails {
  channel?: string
  history_id?: string
  fingerprint?: string
  row_count?: number
  duration_ms?: number
  row_filters?: QueryAuditRowFilter[]
  masked_columns?: string[]
  hidden_columns?: string[]
  error?: string
}

export interface QueryAuditHistory {
  id: string
  datasource_id: string
  model_id: string | null
  user_id: string | null
  logical_query: unknown
  compiled_sql: string | null
  sql_args: string | null
  status: string
  row_count: number | null
  duration_ms: number | null
  error_message: string | null
  fingerprint?: string
  created_at: string
}

export interface QueryAuditDetail {
  history: QueryAuditHistory
  audit: QueryAuditEvent | null
}

export interface QueryAuditListParams {
  page?: number
  pageSize?: number
  search?: string
}

export async function listQueryAudit(
  params: QueryAuditListParams = {},
): Promise<{ entries: QueryAuditEvent[]; total: number }> {
  return apiFetch<{ entries: QueryAuditEvent[]; total: number }>(
    'GET',
    `/api/audit/query${buildQueryString({
      page: params.page,
      page_size: params.pageSize,
      search: params.search ?? undefined,
    })}`,
  )
}

export async function getQueryAuditDetail(historyID: string): Promise<QueryAuditDetail> {
  return apiFetch<QueryAuditDetail>('GET', `/api/audit/query/${encodeURIComponent(historyID)}`)
}
