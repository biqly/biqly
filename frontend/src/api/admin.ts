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
} from '../types/auth'
import { csrfFetch } from './csrf'

const AUTH_API_BASE = '/api/auth'

async function handle<T>(res: Response): Promise<T> {
  const text = await res.text()
  let data: any
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: text }
    }
  }
  if (!res.ok) {
    throw new Error(data?.error || data?.message || `HTTP ${res.status}`)
  }
  return data as T
}

function authHeaders(token: string) {
  return { Authorization: `Bearer ${token}` }
}

// === RBAC admin ===

export async function listRoles(token: string): Promise<Role[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/roles`, { headers: authHeaders(token) })
  return handle<Role[]>(res)
}

export async function listPermissions(token: string): Promise<Permission[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/permissions`, { headers: authHeaders(token) })
  return handle<Permission[]>(res)
}

export async function assignRole(
  token: string,
  userID: string,
  roleID: string,
  scopeType?: string,
  scopeID?: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/users/${userID}/roles`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ role_id: roleID, scope_type: scopeType, scope_id: scopeID }),
  })
  await handle<unknown>(res)
}

export async function removeRole(token: string, userID: string, roleID: string): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/users/${userID}/roles/${roleID}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === Audit log ===

export interface AuditLogFilters {
  userID?: string
  action?: string
  limit?: number
}

export async function listAuditLog(token: string, filters: AuditLogFilters = {}): Promise<AuditLogEntry[]> {
  const params = new URLSearchParams()
  if (filters.userID) params.set('user_id', filters.userID)
  if (filters.action) params.set('action', filters.action)
  if (filters.limit) params.set('limit', String(filters.limit))
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/audit-log${suffix}`, { headers: authHeaders(token) })
  const data = await handle<{ entries: AuditLogEntry[] }>(res)
  return data.entries || []
}

// === Datasource access ===

export async function listDatasourceAccess(token: string): Promise<DatasourceAccess[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/datasource-access`, { headers: authHeaders(token) })
  return handle<DatasourceAccess[]>(res)
}

export async function grantDatasourceAccess(
  token: string,
  userID: string,
  datasourceID: string,
  level: 'read' | 'write' | 'admin',
): Promise<DatasourceAccess> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/datasource-access`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ user_id: userID, datasource_id: datasourceID, access_level: level }),
  })
  return handle<DatasourceAccess>(res)
}

export async function updateDatasourceAccess(
  token: string,
  id: string,
  level: 'read' | 'write' | 'admin',
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/datasource-access/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ access_level: level }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function revokeDatasourceAccess(
  token: string,
  userID: string,
  datasourceID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/datasource-access`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ user_id: userID, datasource_id: datasourceID }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function getMyDatasources(token: string): Promise<string[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me/datasources`, { headers: authHeaders(token) })
  const data = await handle<{ datasource_ids: string[] }>(res)
  return data.datasource_ids || []
}

// === Workspaces ===

export async function listWorkspaces(token: string): Promise<Workspace[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces`, { headers: authHeaders(token) })
  return handle<Workspace[]>(res)
}

export async function createWorkspace(
  token: string,
  name: string,
  description?: string,
): Promise<Workspace> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ name, description }),
  })
  return handle<Workspace>(res)
}

export async function deleteWorkspace(token: string, id: string): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${id}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function listWorkspaceMembers(
  token: string,
  workspaceID: string,
): Promise<WorkspaceMember[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members`, { headers: authHeaders(token) })
  return handle<WorkspaceMember[]>(res)
}

export async function addWorkspaceMember(
  token: string,
  workspaceID: string,
  userID: string,
  roleID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ user_id: userID, role_id: roleID }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === AI queue ===

export async function getAIQueueStatus(token: string, clientSessionID?: string): Promise<AIQueueStatus> {
  const url = clientSessionID
    ? `/api/ai/jobs/queue/status?client_session_id=${encodeURIComponent(clientSessionID)}`
    : '/api/ai/jobs/queue/status'
  const res = await fetch(url, { headers: authHeaders(token) })
  return handle<AIQueueStatus>(res)
}

// === User management admin ===

export async function listUsers(token: string): Promise<AuthUser[]> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users`, { headers: authHeaders(token) })
  return handle<AuthUser[]>(res)
}

export async function getUserDetail(token: string, id: string): Promise<AuthUser> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users/${id}`, { headers: authHeaders(token) })
  return handle<AuthUser>(res)
}

export async function getUserRoles(token: string, id: string): Promise<UserRoleInfo[]> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users/${id}/roles`, { headers: authHeaders(token) })
  return handle<UserRoleInfo[]>(res)
}

export async function updateUserActiveStatus(token: string, id: string, isActive: boolean): Promise<void> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ is_active: isActive }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function requestDatasourceAccess(token: string, datasourceID: string): Promise<{ success: boolean }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me/datasources/${datasourceID}/request-access`, {
    method: 'POST',
    headers: authHeaders(token),
  })
  return handle<{ success: boolean }>(res)
}

// === Workspace detail & update ===

export async function getWorkspace(token: string, id: string): Promise<Workspace> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${id}`, { headers: authHeaders(token) })
  return handle<Workspace>(res)
}

export async function updateWorkspace(
  token: string,
  id: string,
  name: string,
  description?: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ name, description }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === Workspace member management ===

export async function removeWorkspaceMember(
  token: string,
  workspaceID: string,
  userID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members/${userID}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function updateWorkspaceMemberRole(
  token: string,
  workspaceID: string,
  userID: string,
  roleID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members/${userID}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ role_id: roleID }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === Workspace datasources ===

export async function listWorkspaceDatasources(
  token: string,
  workspaceID: string,
): Promise<WorkspaceDatasource[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/datasources`, { headers: authHeaders(token) })
  return handle<WorkspaceDatasource[]>(res)
}

export async function attachWorkspaceDatasource(
  token: string,
  workspaceID: string,
  datasourceID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/datasources`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ datasource_id: datasourceID }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function detachWorkspaceDatasource(
  token: string,
  workspaceID: string,
  datasourceID: string,
): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/datasources/${datasourceID}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === AI history ===

export async function listAIHistory(
  token: string,
  opts: { limit?: number; showAll?: boolean } = {},
): Promise<AIHistoryEntry[]> {
  const params = new URLSearchParams()
  if (opts.limit) params.set('limit', String(opts.limit))
  if (opts.showAll) params.set('show_all', 'true')
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/ai/history${suffix}`, { headers: authHeaders(token) })
  const data = await handle<{ entries: AIHistoryEntry[] }>(res)
  return data.entries || []
}

export async function getAIHistoryDetail(token: string, id: string): Promise<AIHistoryEntry> {
  const res = await fetch(`/api/ai/history/detail?id=${encodeURIComponent(id)}`, { headers: authHeaders(token) })
  return handle<AIHistoryEntry>(res)
}

// === Sharing ===

export async function listShares(
  token: string,
  resourceType?: string,
): Promise<ResourceShare[]> {
  const params = new URLSearchParams()
  if (resourceType) params.set('resource_type', resourceType)
  const suffix = params.toString() ? `?${params.toString()}` : ''
  const res = await csrfFetch(`${AUTH_API_BASE}/shares${suffix}`, { headers: authHeaders(token) })
  return handle<ResourceShare[]>(res)
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
  const res = await csrfFetch(`${AUTH_API_BASE}/shares`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify(body),
  })
  return handle<ResourceShare>(res)
}

export async function deleteShare(token: string, shareID: string): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/shares/${shareID}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}
