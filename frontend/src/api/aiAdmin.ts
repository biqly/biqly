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

/** "environment" when env defaults apply; "database" when ai_runtime_config overrides. */
export type RuntimeConfigSource = 'environment' | 'database'

export interface AmbiguityAdminConfig {
  check_enabled: boolean
  confidence_threshold: number
  max_options: number
  tiered_enabled: boolean
  max_llm_tier_per_question: number
  db_override: boolean
  source?: RuntimeConfigSource
  /** Per-knob source map, e.g. { tiered_enabled: 'database' }. */
  sources?: Record<string, RuntimeConfigSource>
}

export interface PIIAdminConfig {
  /** Read-only env kill switch (BI_PII_ENABLED) — not editable via this API. */
  enabled: boolean
  detection_threshold: number
  db_override: boolean
  source?: RuntimeConfigSource
  sources?: Record<string, RuntimeConfigSource>
}

export interface MemoryAdminConfig {
  recall_enabled: boolean
  recall_limit: number
  db_override: boolean
  source?: RuntimeConfigSource
  sources?: Record<string, RuntimeConfigSource>
}

export interface AIAdminRuntimeConfig {
  ambiguity: AmbiguityAdminConfig
  pii: PIIAdminConfig
  memory: MemoryAdminConfig
}

/**
 * PUT body. Each provided domain REPLACES that domain's stored override row:
 * omitted fields inside a provided domain fall back to environment defaults,
 * and an empty object clears the domain's overrides entirely.
 */
export interface AIAdminRuntimeConfigUpdate {
  ambiguity?: {
    check_enabled?: boolean
    confidence_threshold?: number
    max_options?: number
    tiered_enabled?: boolean
    max_llm_tier_per_question?: number
  }
  pii?: {
    detection_threshold?: number
  }
  memory?: {
    recall_enabled?: boolean
    recall_limit?: number
  }
}

export const getAIAdminConfig = () =>
  apiFetch<AIAdminRuntimeConfig>('GET', `${AI_API_BASE}/admin/config`, undefined, adminOpts)

export const updateAIAdminConfig = (update: AIAdminRuntimeConfigUpdate) =>
  apiFetch<AIAdminRuntimeConfig>('PUT', `${AI_API_BASE}/admin/config`, update, adminOpts)
