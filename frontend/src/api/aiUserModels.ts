import type { AIPurpose } from './aiProviders'
import { apiFetch } from './apiClient'
import { AI_API_BASE } from './constants'

export interface SelectableAIModel {
  id: string
  purpose: AIPurpose
  model_id: string
  display_name: string
  provider_name: string
  provider_type: string
}

export interface UserAIModelsResponse {
  models: SelectableAIModel[]
  preferences: Record<string, string>
  restricted: boolean
  db_managed: boolean
}

export async function fetchUserAIModels(token?: string): Promise<UserAIModelsResponse> {
  return apiFetch<UserAIModelsResponse>('GET', `${AI_API_BASE}/user-models`, undefined, { token })
}

export async function putUserAIPreferences(
  preferences: { purpose: AIPurpose; model_id: string }[],
  token?: string,
): Promise<{ preferences: Record<string, string> }> {
  return apiFetch<{ preferences: Record<string, string> }>(
    'PUT',
    `${AI_API_BASE}/user-preferences`,
    { preferences },
    { token },
  )
}

export async function deleteUserAIPreference(purpose: AIPurpose, token?: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AI_API_BASE}/user-preferences/${purpose}`, undefined, { token })
}
