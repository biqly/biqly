import { apiFetch } from './apiClient'
import { ADMIN_OPTS, AUTH_API_BASE } from './constants'

export interface AIProviderWorkspaceGrant {
  workspace_id: string
  provider_id: string
  granted_by?: string
  granted_at: string
}

export interface AIModelWorkspaceGrant {
  workspace_id: string
  model_id: string
  granted_by?: string
  granted_at: string
}

export interface AIProviderRoleGrant {
  role_id: string
  provider_id: string
  granted_by?: string
  granted_at: string
}

export interface AIModelRoleGrant {
  role_id: string
  model_id: string
  granted_by?: string
  granted_at: string
}

export interface AIModelAccessGrants {
  provider_workspaces: AIProviderWorkspaceGrant[]
  model_workspaces: AIModelWorkspaceGrant[]
  provider_roles: AIProviderRoleGrant[]
  model_roles: AIModelRoleGrant[]
}

/** Go encodes empty slices as JSON null; normalize before iteration. */
type AIModelAccessGrantsRaw = {
  [K in keyof AIModelAccessGrants]: AIModelAccessGrants[K] | null | undefined
}

function normalizeAIModelAccessGrants(raw: AIModelAccessGrantsRaw): AIModelAccessGrants {
  return {
    provider_workspaces: raw.provider_workspaces ?? [],
    model_workspaces: raw.model_workspaces ?? [],
    provider_roles: raw.provider_roles ?? [],
    model_roles: raw.model_roles ?? [],
  }
}

const adminOpts = ADMIN_OPTS

const emptyAIModelAccessGrants = (): AIModelAccessGrants => ({
  provider_workspaces: [],
  model_workspaces: [],
  provider_roles: [],
  model_roles: [],
})

export async function listAIModelAccess(token: string): Promise<AIModelAccessGrants> {
  const raw = await apiFetch<AIModelAccessGrantsRaw | null>(
    'GET',
    `${AUTH_API_BASE}/admin/ai-model-access`,
    undefined,
    {
      token,
      ...adminOpts,
    },
  )
  if (!raw) {
    return emptyAIModelAccessGrants()
  }
  return normalizeAIModelAccessGrants(raw)
}

export async function grantProviderWorkspace(
  token: string,
  workspaceID: string,
  providerID: string,
): Promise<void> {
  await apiFetch<void>(
    'POST',
    `${AUTH_API_BASE}/admin/ai-model-access/workspace/provider`,
    { workspace_id: workspaceID, provider_id: providerID },
    { token, ...adminOpts },
  )
}

export async function revokeProviderWorkspace(
  token: string,
  workspaceID: string,
  providerID: string,
): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `${AUTH_API_BASE}/admin/ai-model-access/workspace/provider`,
    { workspace_id: workspaceID, provider_id: providerID },
    { token, ...adminOpts },
  )
}

export async function grantModelWorkspace(
  token: string,
  workspaceID: string,
  modelID: string,
): Promise<void> {
  await apiFetch<void>(
    'POST',
    `${AUTH_API_BASE}/admin/ai-model-access/workspace/model`,
    { workspace_id: workspaceID, model_id: modelID },
    { token, ...adminOpts },
  )
}

export async function revokeModelWorkspace(
  token: string,
  workspaceID: string,
  modelID: string,
): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `${AUTH_API_BASE}/admin/ai-model-access/workspace/model`,
    { workspace_id: workspaceID, model_id: modelID },
    { token, ...adminOpts },
  )
}

export async function grantProviderRole(
  token: string,
  roleID: string,
  providerID: string,
): Promise<void> {
  await apiFetch<void>(
    'POST',
    `${AUTH_API_BASE}/admin/ai-model-access/role/provider`,
    { role_id: roleID, provider_id: providerID },
    { token, ...adminOpts },
  )
}

export async function revokeProviderRole(
  token: string,
  roleID: string,
  providerID: string,
): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `${AUTH_API_BASE}/admin/ai-model-access/role/provider`,
    { role_id: roleID, provider_id: providerID },
    { token, ...adminOpts },
  )
}

export async function grantModelRole(
  token: string,
  roleID: string,
  modelID: string,
): Promise<void> {
  await apiFetch<void>(
    'POST',
    `${AUTH_API_BASE}/admin/ai-model-access/role/model`,
    { role_id: roleID, model_id: modelID },
    { token, ...adminOpts },
  )
}

export async function revokeModelRole(
  token: string,
  roleID: string,
  modelID: string,
): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `${AUTH_API_BASE}/admin/ai-model-access/role/model`,
    { role_id: roleID, model_id: modelID },
    { token, ...adminOpts },
  )
}
