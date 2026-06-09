import { apiFetch } from './apiClient'

const AI_API_BASE = '/api/ai'

// Memory store + runtime config endpoints are admin-gated; the admin key (or a
// super-admin JWT) is sent via the apiClient's useAdminKey option, matching the
// provider/model management endpoints.
const adminOpts = { useAdminKey: true as const }

export interface ConfirmedQuery {
  id: string
  datasource_id: string
  model_id?: string
  user_id?: string
  nl_query: string
  sql_query: string
  semantic_model_hash?: string
  is_active: boolean
  confirmed_at: string
}

export const listConfirmedQueries = (datasourceId: string) =>
  apiFetch<ConfirmedQuery[]>(
    'GET',
    `${AI_API_BASE}/confirmed-queries?datasource_id=${encodeURIComponent(datasourceId)}`,
    undefined,
    adminOpts,
  )

export const deactivateConfirmedQuery = (id: string) =>
  apiFetch<{ status: string }>(
    'POST',
    `${AI_API_BASE}/confirmed-queries/${encodeURIComponent(id)}/deactivate`,
    {},
    adminOpts,
  )

export interface AmbiguityAdminConfig {
  tiered_enabled: boolean
  max_llm_tier_per_question: number
  db_override: boolean
  /** "environment" when env defaults apply; "database" when ai_runtime_config overrides. */
  source?: 'environment' | 'database'
}

export interface AIAdminRuntimeConfig {
  ambiguity: AmbiguityAdminConfig
}

export const getAIAdminConfig = () =>
  apiFetch<AIAdminRuntimeConfig>('GET', `${AI_API_BASE}/admin/config`, undefined, adminOpts)

export const updateAIAdminConfig = (ambiguity: {
  tiered_enabled: boolean
  max_llm_tier_per_question: number
}) => apiFetch<AIAdminRuntimeConfig>('PUT', `${AI_API_BASE}/admin/config`, { ambiguity }, adminOpts)
