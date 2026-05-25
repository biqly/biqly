import type {
  DatasourceAccess,
  Permission,
  Role,
  Workspace,
  WorkspaceMember,
  AIQueueStatus,
} from '../types/auth'

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
  const res = await fetch(`${AUTH_API_BASE}/admin/roles`, { headers: authHeaders(token) })
  return handle<Role[]>(res)
}

export async function listPermissions(token: string): Promise<Permission[]> {
  const res = await fetch(`${AUTH_API_BASE}/admin/permissions`, { headers: authHeaders(token) })
  return handle<Permission[]>(res)
}

export async function assignRole(
  token: string,
  userID: string,
  roleID: string,
  scopeType?: string,
  scopeID?: string,
): Promise<void> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users/${userID}/roles`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ role_id: roleID, scope_type: scopeType, scope_id: scopeID }),
  })
  await handle<unknown>(res)
}

export async function removeRole(token: string, userID: string, roleID: string): Promise<void> {
  const res = await fetch(`${AUTH_API_BASE}/admin/users/${userID}/roles/${roleID}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

// === Datasource access ===

export async function listDatasourceAccess(token: string): Promise<DatasourceAccess[]> {
  const res = await fetch(`${AUTH_API_BASE}/admin/datasource-access`, { headers: authHeaders(token) })
  return handle<DatasourceAccess[]>(res)
}

export async function grantDatasourceAccess(
  token: string,
  userID: string,
  datasourceID: string,
  level: 'read' | 'write' | 'admin',
): Promise<DatasourceAccess> {
  const res = await fetch(`${AUTH_API_BASE}/admin/datasource-access`, {
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
  const res = await fetch(`${AUTH_API_BASE}/admin/datasource-access/${id}`, {
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
  const res = await fetch(`${AUTH_API_BASE}/admin/datasource-access`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ user_id: userID, datasource_id: datasourceID }),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function getMyDatasources(token: string): Promise<string[]> {
  const res = await fetch(`${AUTH_API_BASE}/me/datasources`, { headers: authHeaders(token) })
  const data = await handle<{ datasource_ids: string[] }>(res)
  return data.datasource_ids || []
}

// === Workspaces ===

export async function listWorkspaces(token: string): Promise<Workspace[]> {
  const res = await fetch(`${AUTH_API_BASE}/workspaces`, { headers: authHeaders(token) })
  return handle<Workspace[]>(res)
}

export async function createWorkspace(
  token: string,
  name: string,
  description?: string,
): Promise<Workspace> {
  const res = await fetch(`${AUTH_API_BASE}/workspaces`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders(token) },
    body: JSON.stringify({ name, description }),
  })
  return handle<Workspace>(res)
}

export async function deleteWorkspace(token: string, id: string): Promise<void> {
  const res = await fetch(`${AUTH_API_BASE}/workspaces/${id}`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
}

export async function listWorkspaceMembers(
  token: string,
  workspaceID: string,
): Promise<WorkspaceMember[]> {
  const res = await fetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members`, { headers: authHeaders(token) })
  return handle<WorkspaceMember[]>(res)
}

export async function addWorkspaceMember(
  token: string,
  workspaceID: string,
  userID: string,
  roleID: string,
): Promise<void> {
  const res = await fetch(`${AUTH_API_BASE}/workspaces/${workspaceID}/members`, {
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
