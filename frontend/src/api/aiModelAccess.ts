import { apiFetch } from './apiClient'

const AUTH_API_BASE = '/api/auth'

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

const adminOpts = { useAdminKey: true as const }

export async function listAIModelAccess(token: string): Promise<AIModelAccessGrants> {
  return apiFetch<AIModelAccessGrants>('GET', `${AUTH_API_BASE}/admin/ai-model-access`, undefined, {
    token,
    ...adminOpts,
  })
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
