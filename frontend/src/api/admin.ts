import type {
  DatasourceAccess,
  Permission,
  Role,
  Workspace,
  WorkspaceMember,
  WorkspaceDatasource,
  AIQueueStatus,
  AIHistoryEntry,
  AuditLogEntry,
  AuthUser,
  UserRoleInfo,
  ResourceShare,
  PlatformSettings,
} from '../types/auth'
import { normalizeAuthUser } from './auth'
import { apiFetch } from './apiClient'

const AUTH_API_BASE = '/api/auth'

// === RBAC admin ===

export async function listRoles(token: string, page?: number, pageSize?: number): Promise<{ roles: Role[]; total: number }> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<{ roles: Role[]; total: number }>('GET', `${AUTH_API_BASE}/admin/roles${suffix}`, undefined, { token })
}

export async function listPermissions(token: string, page?: number, pageSize?: number): Promise<{ permissions: Permission[]; total: number }> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<{ permissions: Permission[]; total: number }>('GET', `${AUTH_API_BASE}/admin/permissions${suffix}`, undefined, { token })
}

export async function getRolePermissions(token: string, roleID: string): Promise<string[]> {
  const data = await apiFetch<{ permission_ids: string[] }>(
    'GET',
    `${AUTH_API_BASE}/admin/roles/${roleID}/permissions`,
    undefined,
    { token },
  )
  return data.permission_ids ?? []
}

export async function setRolePermissions(token: string, roleID: string, permissionIDs: string[]): Promise<void> {
  await apiFetch<void>(
    'PUT',
    `${AUTH_API_BASE}/admin/roles/${roleID}/permissions`,
    { permission_ids: permissionIDs },
    { token },
  )
}

export async function assignRole(
  token: string,
  userID: string,
  roleID: string,
  scopeType?: string,
  scopeID?: string,
): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/admin/users/${userID}/roles`, {
    role_id: roleID,
    scope_type: scopeType,
    scope_id: scopeID,
  }, { token })
}

export async function removeRole(token: string, userID: string, roleID: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/admin/users/${userID}/roles/${roleID}`, undefined, { token })
}

// === Audit log ===

export interface AuditLogFilters {
  userID?: string
  action?: string
  limit?: number
  page?: number
  pageSize?: number
}

export async function listAuditLog(token: string, filters: AuditLogFilters = {}): Promise<{ entries: AuditLogEntry[]; total: number }> {
  const params = new URLSearchParams()
  if (filters.userID) params.set('user_id', filters.userID)
  if (filters.action) params.set('action', filters.action)
  if (filters.limit) params.set('limit', String(filters.limit))
  if (filters.page) params.set('page', String(filters.page))
  if (filters.pageSize) params.set('page_size', String(filters.pageSize))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const data = await apiFetch<{ entries?: AuditLogEntry[]; total?: number }>('GET', `${AUTH_API_BASE}/admin/audit-log${suffix}`, undefined, { token })
  return { entries: data?.entries || [], total: data?.total || 0 }
}

// === Datasource access ===

export async function listDatasourceAccess(token: string, page?: number, pageSize?: number): Promise<{ access: DatasourceAccess[]; total: number }> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<{ access: DatasourceAccess[]; total: number }>('GET', `${AUTH_API_BASE}/admin/datasource-access${suffix}`, undefined, { token })
}

export async function grantDatasourceAccess(
  token: string,
  userID: string,
  datasourceID: string,
  level: 'read' | 'write' | 'admin',
): Promise<DatasourceAccess> {
  return apiFetch<DatasourceAccess>('POST', `${AUTH_API_BASE}/admin/datasource-access`, {
    user_id: userID,
    datasource_id: datasourceID,
    access_level: level,
  }, { token })
}

export async function updateDatasourceAccess(
  token: string,
  id: string,
  level: 'read' | 'write' | 'admin',
): Promise<void> {
  await apiFetch<void>('PUT', `${AUTH_API_BASE}/admin/datasource-access/${id}`, {
    access_level: level,
  }, { token })
}

export async function revokeDatasourceAccess(
  token: string,
  userID: string,
  datasourceID: string,
): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/admin/datasource-access`, {
    user_id: userID,
    datasource_id: datasourceID,
  }, { token })
}

export async function getMyDatasources(token: string): Promise<string[]> {
  const data = await apiFetch<{ datasource_ids: string[] }>('GET', `${AUTH_API_BASE}/me/datasources`, undefined, { token })
  return data.datasource_ids || []
}

// === Workspaces ===

export async function listWorkspaces(token: string, page?: number, pageSize?: number): Promise<{ workspaces: Workspace[]; total: number }> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (pageSize) params.set('page_size', String(pageSize))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<{ workspaces: Workspace[]; total: number }>('GET', `${AUTH_API_BASE}/workspaces${suffix}`, undefined, { token })
}

export async function createWorkspace(
  token: string,
  name: string,
  description?: string,
): Promise<Workspace> {
  return apiFetch<Workspace>('POST', `${AUTH_API_BASE}/workspaces`, { name, description }, { token })
}

export async function deleteWorkspace(token: string, id: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/workspaces/${id}`, undefined, { token })
}

export async function listWorkspaceMembers(
  token: string,
  workspaceID: string,
): Promise<WorkspaceMember[]> {
  return apiFetch<WorkspaceMember[]>('GET', `${AUTH_API_BASE}/workspaces/${workspaceID}/members`, undefined, { token })
}

export async function addWorkspaceMember(
  token: string,
  workspaceID: string,
  userID: string,
  roleID: string,
): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/workspaces/${workspaceID}/members`, {
    user_id: userID,
    role_id: roleID,
  }, { token })
}

// === AI queue ===

export async function getAIQueueStatus(token: string, clientSessionID?: string): Promise<AIQueueStatus> {
  const url = clientSessionID
    ? `/api/ai/jobs/queue/status?client_session_id=${encodeURIComponent(clientSessionID)}`
    : '/api/ai/jobs/queue/status'
  return apiFetch<AIQueueStatus>('GET', url, undefined, { token })
}

// === User management admin ===

export async function listUsers(token: string, filters: { page?: number; pageSize?: number; search?: string; status?: string } = {}): Promise<{ users: AuthUser[]; total: number }> {
  const params = new URLSearchParams()
  if (filters.page) params.set('page', String(filters.page))
  if (filters.pageSize) params.set('page_size', String(filters.pageSize))
  if (filters.search) params.set('search', filters.search)
  if (filters.status) params.set('status', filters.status)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const data = await apiFetch<{ users: any[]; total: number }>('GET', `${AUTH_API_BASE}/admin/users${suffix}`, undefined, { token })
  return {
    users: (data.users || []).map(normalizeAuthUser),
    total: data.total || 0,
  }
}

export async function getUserDetail(token: string, id: string): Promise<AuthUser> {
  const data = await apiFetch<any>('GET', `${AUTH_API_BASE}/admin/users/${id}`, undefined, { token })
  return normalizeAuthUser(data)
}

export async function getUserRoles(token: string, id: string): Promise<UserRoleInfo[]> {
  return apiFetch<UserRoleInfo[]>('GET', `${AUTH_API_BASE}/admin/users/${id}/roles`, undefined, { token })
}

export async function updateUserActiveStatus(token: string, id: string, isActive: boolean): Promise<void> {
  await apiFetch<void>('PUT', `${AUTH_API_BASE}/admin/users/${id}`, { is_active: isActive }, { token })
}

export async function resendUserVerification(token: string, userId: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('POST', `${AUTH_API_BASE}/admin/users/${encodeURIComponent(userId)}/resend-verification`, undefined, { token })
}

export async function generateMFABypassCode(token: string, userId: string): Promise<{ bypass_code: string }> {
  return apiFetch<{ bypass_code: string }>('POST', `${AUTH_API_BASE}/admin/users/${encodeURIComponent(userId)}/mfa/bypass`, undefined, { token })
}

export async function requestDatasourceAccess(token: string, datasourceID: string): Promise<{ success: boolean }> {
  return apiFetch<{ success: boolean }>('POST', `${AUTH_API_BASE}/me/datasources/${datasourceID}/request-access`, undefined, { token })
}

// === Workspace detail & update ===

export async function getWorkspace(token: string, id: string): Promise<Workspace> {
  return apiFetch<Workspace>('GET', `${AUTH_API_BASE}/workspaces/${id}`, undefined, { token })
}

export async function updateWorkspace(
  token: string,
  id: string,
  name: string,
  description?: string,
  mfaRequired?: boolean,
): Promise<void> {
  await apiFetch<void>('PUT', `${AUTH_API_BASE}/workspaces/${id}`, {
    name,
    description,
    mfa_required: mfaRequired,
  }, { token })
}

// === Workspace member management ===

export async function removeWorkspaceMember(
  token: string,
  workspaceID: string,
  userID: string,
): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/workspaces/${workspaceID}/members/${userID}`, undefined, { token })
}

export async function updateWorkspaceMemberRole(
  token: string,
  workspaceID: string,
  userID: string,
  roleID: string,
): Promise<void> {
  await apiFetch<void>('PUT', `${AUTH_API_BASE}/workspaces/${workspaceID}/members/${userID}`, { role_id: roleID }, { token })
}

// === Workspace datasources ===

export async function listWorkspaceDatasources(
  token: string,
  workspaceID: string,
): Promise<WorkspaceDatasource[]> {
  return apiFetch<WorkspaceDatasource[]>('GET', `${AUTH_API_BASE}/workspaces/${workspaceID}/datasources`, undefined, { token })
}

export async function attachWorkspaceDatasource(
  token: string,
  workspaceID: string,
  datasourceID: string,
): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/workspaces/${workspaceID}/datasources`, { datasource_id: datasourceID }, { token })
}

export async function detachWorkspaceDatasource(
  token: string,
  workspaceID: string,
  datasourceID: string,
): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/workspaces/${workspaceID}/datasources/${datasourceID}`, undefined, { token })
}

// === AI history ===

export async function listAIHistory(
  token: string,
  opts: {
    page?: number
    pageSize?: number
    limit?: number
    showAll?: boolean
    datasourceId?: string
    modelId?: string
    status?: string
    search?: string
  } = {},
): Promise<{ entries: AIHistoryEntry[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.page) params.set('page', String(opts.page))
  if (opts.pageSize) params.set('page_size', String(opts.pageSize))
  if (opts.limit) params.set('limit', String(opts.limit))
  if (opts.showAll) params.set('show_all', 'true')
  else if (opts.showAll === false) params.set('show_all', 'false')
  if (opts.datasourceId) params.set('datasource_id', opts.datasourceId)
  if (opts.modelId) params.set('model_id', opts.modelId)
  if (opts.status) params.set('status', opts.status)
  if (opts.search?.trim()) params.set('search', opts.search.trim())
  const suffix = params.toString() ? `?${params.toString()}` : ''
  return apiFetch<{ entries: AIHistoryEntry[]; total: number }>('GET', `/api/ai/history${suffix}`, undefined, { token })
}

export async function getAIHistoryDetail(token: string, id: string): Promise<AIHistoryEntry> {
  return apiFetch<AIHistoryEntry>('GET', `/api/ai/history/detail?id=${encodeURIComponent(id)}`, undefined, { token })
}

// === Sharing ===

export async function listShares(
  token: string,
  opts: { page?: number; pageSize?: number; resourceType?: string } = {},
): Promise<{ shares: ResourceShare[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.page) params.set('page', String(opts.page))
  if (opts.pageSize) params.set('page_size', String(opts.pageSize))
  if (opts.resourceType) params.set('resource_type', opts.resourceType)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const data = await apiFetch<{ shares: ResourceShare[]; total: number } | null>('GET', `${AUTH_API_BASE}/shares${suffix}`, undefined, { token })
  return data || { shares: [], total: 0 }
}

export async function createShare(
  token: string,
  resourceType: string,
  resourceID: string,
  permission: 'view' | 'execute' | 'edit',
  sharedWith?: string,
  workspaceID?: string,
): Promise<ResourceShare> {
  const body: Record<string, unknown> = { resource_type: resourceType, resource_id: resourceID, permission }
  if (sharedWith) body.shared_with = sharedWith
  if (workspaceID) body.workspace_id = workspaceID
  return apiFetch<ResourceShare>('POST', `${AUTH_API_BASE}/shares`, body, { token })
}

export async function deleteShare(token: string, shareID: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/shares/${shareID}`, undefined, { token })
}

// === Security Policies (Row-Level Security & Field Permissions) ===

export interface PermissionRowFilter {
  field: string
  operator?: string
  value?: any
}

export type PIIAccessLevel = 'raw' | 'masked' | 'hidden'

export interface PIIColumnAccess {
  access: PIIAccessLevel
}

export interface SecurityPolicy {
  id?: string
  user_id: string
  datasource_id: string
  allowed_models: string[]
  denied_fields: string[]
  row_filters: PermissionRowFilter[]
  pii_policy?: Record<string, PIIColumnAccess>
}

export interface PIIColumn {
  column_id: string
  schema: string
  table: string
  column: string
  pii_type: string
  confidence: number | null
  masking_strategy: string | null
  reviewed_by: string | null
}

export interface PIIScanSummary {
  scanned_columns: number
  detected: Record<string, number>
}

export async function listPIIColumns(token: string, datasourceID: string): Promise<PIIColumn[]> {
  return apiFetch<PIIColumn[]>(
    'GET',
    `/api/datasources/${encodeURIComponent(datasourceID)}/pii-columns`,
    undefined,
    { token },
  )
}

export async function scanPII(token: string, datasourceID: string): Promise<PIIScanSummary> {
  return apiFetch<PIIScanSummary>(
    'POST',
    `/api/datasources/${encodeURIComponent(datasourceID)}/scan-pii`,
    undefined,
    { token },
  )
}

export async function updateColumnPII(
  token: string,
  columnID: string,
  body: { pii_type: string; pii_masking_strategy?: string; pii_reviewed_by: string },
): Promise<void> {
  await apiFetch<unknown>(
    'PATCH',
    `/api/metadata/columns/${encodeURIComponent(columnID)}/pii`,
    body,
    { token },
  )
}

export async function deleteColumnPII(token: string, columnID: string, reviewedBy: string): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `/api/metadata/columns/${encodeURIComponent(columnID)}/pii?reviewed_by=${encodeURIComponent(reviewedBy)}`,
    undefined,
    { token },
  )
}

export async function listSecurityPolicies(token: string): Promise<SecurityPolicy[]> {
  return apiFetch<SecurityPolicy[]>('GET', '/api/permissions', undefined, { token })
}

export async function getSecurityPolicyByKeys(
  token: string,
  userID: string,
  datasourceID: string,
): Promise<SecurityPolicy> {
  return apiFetch<SecurityPolicy>(
    'GET',
    `/api/permissions/keys?user_id=${encodeURIComponent(userID)}&datasource_id=${encodeURIComponent(datasourceID)}`,
    undefined,
    { token },
  )
}

export async function upsertSecurityPolicy(token: string, policy: SecurityPolicy): Promise<SecurityPolicy> {
  return apiFetch<SecurityPolicy>('PUT', '/api/permissions', policy, { token })
}

export async function deleteSecurityPolicy(token: string, id: string): Promise<void> {
  await apiFetch<void>('DELETE', `/api/permissions/${id}`, undefined, { token })
}

export async function deleteSecurityPolicyByKeys(
  token: string,
  userID: string,
  datasourceID: string,
): Promise<void> {
  await apiFetch<void>(
    'DELETE',
    `/api/permissions/keys?user_id=${encodeURIComponent(userID)}&datasource_id=${encodeURIComponent(datasourceID)}`,
    undefined,
    { token },
  )
}

export async function getPlatformSettings(token: string): Promise<PlatformSettings> {
  return apiFetch<PlatformSettings>('GET', `${AUTH_API_BASE}/admin/platform-settings`, undefined, { token })
}

export async function updatePlatformSettings(
  token: string,
  selfSignupEnabled: boolean,
): Promise<PlatformSettings> {
  return apiFetch<PlatformSettings>(
    'PUT',
    `${AUTH_API_BASE}/admin/platform-settings`,
    { self_signup_enabled: selfSignupEnabled },
    { token },
  )
}
