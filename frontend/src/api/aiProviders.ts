import { apiFetch } from './apiClient'

const AI_API_BASE = '/api/ai'

// Provider/model management endpoints are admin-gated; the admin key is sent
// via the apiClient's useAdminKey option (same scheme as the eval endpoints).
const adminOpts = { useAdminKey: true as const }

export type AIPurpose = 'query' | 'describe' | 'embedding' | 'translation' | 'judge'
export type AIProviderType = 'openai' | 'openai-compatible' | 'anthropic'

export interface AIProvider {
  id: string
  name: string
  provider_type: AIProviderType
  base_url: string
  api_key_masked?: string
  has_api_key: boolean
  is_active: boolean
  http_timeout_seconds: number
  rate_limit_per_minute: number
  model_count: number
  created_at: string
  updated_at: string
}

export interface AIModel {
  id: string
  provider_id: string
  provider_name: string
  provider_type: AIProviderType
  model_id: string
  display_name: string
  purpose: AIPurpose
  max_tokens: number
  temperature: number
  top_p: number
  num_ctx: number
  max_prompt_input_runes: number
  is_default: boolean
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface ConnectionTestResult {
  status: 'connected' | 'error'
  latency_ms?: number
  message?: string
  model?: string
}

export interface CreateProviderPayload {
  name: string
  provider_type: AIProviderType
  base_url: string
  api_key?: string
  is_active?: boolean
  http_timeout_seconds?: number
  rate_limit_per_minute?: number
}

export interface UpdateProviderPayload {
  name: string
  provider_type: AIProviderType
  base_url: string
  api_key?: string | null
  is_active?: boolean
  http_timeout_seconds?: number
  rate_limit_per_minute?: number
}

export interface CreateModelPayload {
  provider_id: string
  model_id: string
  display_name: string
  purpose: AIPurpose
  max_tokens?: number
  temperature?: number
  top_p?: number
  num_ctx?: number
  max_prompt_input_runes?: number
  is_default?: boolean
  is_active?: boolean
}

export interface UpdateModelPayload {
  model_id: string
  display_name: string
  purpose: AIPurpose
  max_tokens?: number
  temperature?: number
  top_p?: number
  num_ctx?: number
  max_prompt_input_runes?: number
  is_active?: boolean
}

export const listProviders = () =>
  apiFetch<AIProvider[]>('GET', `${AI_API_BASE}/providers`, undefined, adminOpts)

export const createProvider = (payload: CreateProviderPayload) =>
  apiFetch<AIProvider>('POST', `${AI_API_BASE}/providers`, payload, adminOpts)

export const updateProvider = (id: string, payload: UpdateProviderPayload) =>
  apiFetch<AIProvider>('PUT', `${AI_API_BASE}/providers/${id}`, payload, adminOpts)

export const deleteProvider = (id: string) =>
  apiFetch<void>('DELETE', `${AI_API_BASE}/providers/${id}`, undefined, adminOpts)

export const testProvider = (id: string, modelID?: string) =>
  apiFetch<ConnectionTestResult>('POST', `${AI_API_BASE}/providers/${id}/test`, { model_id: modelID ?? '' }, adminOpts)

export const listActiveModels = () =>
  apiFetch<AIModel[]>('GET', `${AI_API_BASE}/providers/active-models`, undefined, adminOpts)

export const listModels = (providerID?: string, purpose?: string) => {
  const params = new URLSearchParams()
  if (providerID) params.set('provider_id', providerID)
  if (purpose) params.set('purpose', purpose)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<AIModel[]>('GET', `${AI_API_BASE}/models${suffix}`, undefined, adminOpts)
}

export const createModel = (payload: CreateModelPayload) =>
  apiFetch<{ id: string }>('POST', `${AI_API_BASE}/models`, payload, adminOpts)

export const updateModel = (id: string, payload: UpdateModelPayload) =>
  apiFetch<{ status: string }>('PUT', `${AI_API_BASE}/models/${id}`, payload, adminOpts)

export const deleteModel = (id: string) =>
  apiFetch<void>('DELETE', `${AI_API_BASE}/models/${id}`, undefined, adminOpts)

export const setDefaultModel = (id: string) =>
  apiFetch<{ status: string }>('POST', `${AI_API_BASE}/models/${id}/default`, {}, adminOpts)
