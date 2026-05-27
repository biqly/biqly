import type { AuthUser, PasskeyInfo, PasswordPolicy, SetActiveWorkspaceResponse, TokenResponse, Invitation } from '../types/auth'
import { csrfFetch } from './csrf'

const AUTH_API_BASE = '/api/auth'

const DEFAULT_PASSWORD_POLICY: PasswordPolicy = {
  min_length: 8,
  max_length: 128,
  require_upper: true,
  require_lower: true,
  require_digit: true,
  require_special: true,
  min_score: 2,
}

let cachedPolicy: PasswordPolicy | null = null
let inflightPolicy: Promise<PasswordPolicy> | null = null

export async function apiGetPasswordPolicy(): Promise<PasswordPolicy> {
  if (cachedPolicy) return cachedPolicy
  if (inflightPolicy) return inflightPolicy
  inflightPolicy = (async () => {
    try {
      const res = await csrfFetch(`${AUTH_API_BASE}/password-policy`, { method: 'GET' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = (await res.json()) as PasswordPolicy
      cachedPolicy = { ...DEFAULT_PASSWORD_POLICY, ...data }
      return cachedPolicy
    } catch {
      // Network or backend unavailable — fall back to defaults so the SPA
      // still enforces the historic rules client-side.
      cachedPolicy = DEFAULT_PASSWORD_POLICY
      return cachedPolicy
    } finally {
      inflightPolicy = null
    }
  })()
  return inflightPolicy
}

async function handleResponse<T>(res: Response): Promise<T> {
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
    const msg = data?.error || data?.message || `HTTP ${res.status}`
    throw new Error(msg)
  }

  return data as T
}

export function normalizeAuthUser(raw: any): AuthUser {
  return {
    id: raw.id,
    email: raw.email,
    username: raw.username,
    displayName: raw.displayName ?? raw.display_name,
    avatarUrl: raw.avatarUrl ?? raw.avatar_url,
    isActive: raw.isActive ?? raw.is_active,
    emailVerified: raw.emailVerified ?? raw.email_verified,
    active_workspace_id: raw.active_workspace_id,
    createdAt: raw.createdAt ?? raw.created_at,
    updatedAt: raw.updatedAt ?? raw.updated_at,
  }
}

export async function apiRegister(email: string, password: string, displayName: string): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiLogin(email: string, password: string): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiOAuthExchange(code: string): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/oauth/exchange`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiRefresh(refreshToken: string): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiLogout(refreshToken: string): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/logout`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }
}

export async function apiGetMe(accessToken: string): Promise<AuthUser> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return normalizeAuthUser(await handleResponse<any>(res))
}

export async function apiSetActiveWorkspace(accessToken: string, workspaceID: string): Promise<SetActiveWorkspaceResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me/active-workspace`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ workspace_id: workspaceID }),
  })
  return handleResponse<SetActiveWorkspaceResponse>(res)
}

export async function apiPasskeyRegisterBegin(accessToken: string): Promise<any> {
  const res = await csrfFetch(`${AUTH_API_BASE}/passkey/register-begin`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<any>(res)
}

export async function apiPasskeyRegisterFinish(
  accessToken: string,
  credential: any,
  name?: string
): Promise<{ status: string }> {
  let url = `${AUTH_API_BASE}/passkey/register-finish`
  if (name) {
    url += `?name=${encodeURIComponent(name)}`
  }
  const res = await csrfFetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${accessToken}`,
    },
    body: JSON.stringify(credential),
  })
  return handleResponse<{ status: string }>(res)
}

export async function apiPasskeyLoginBegin(email: string): Promise<any> {
  const res = await csrfFetch(`${AUTH_API_BASE}/passkey/login-begin`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  return handleResponse<any>(res)
}

export async function apiPasskeyLoginFinish(credential: any): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/passkey/login-finish`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(credential),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiGetPasskeys(accessToken: string): Promise<PasskeyInfo[]> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me/passkeys`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<PasskeyInfo[]>(res)
}

export async function apiDeletePasskey(accessToken: string, id: string): Promise<void> {
  const res = await csrfFetch(`${AUTH_API_BASE}/me/passkeys/${id}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `HTTP ${res.status}`)
  }
}

export async function apiForgotPassword(email: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/forgot-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiResetPassword(token: string, password: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/reset-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, password }),
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiVerifyEmail(token: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/verify-email?token=${encodeURIComponent(token)}`, {
    method: 'GET',
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiInviteUser(accessToken: string, email: string, roleName: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/invitations`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, role_name: roleName }),
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiGetInvitation(token: string): Promise<{
  id: string
  email: string
  role_id: string
  role_name: string
  invited_by: string
  expires_at: string
}> {
  const res = await csrfFetch(`${AUTH_API_BASE}/invitations/${encodeURIComponent(token)}`, {
    method: 'GET',
  })
  return handleResponse<any>(res)
}

export async function apiClaimInvitation(token: string, password: string, displayName: string): Promise<TokenResponse> {
  const res = await csrfFetch(`${AUTH_API_BASE}/invitations/${encodeURIComponent(token)}/claim`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ password, display_name: displayName }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiListInvitations(
  accessToken: string,
  params: { page: number; pageSize: number; search?: string; status?: string }
): Promise<{ invitations: Invitation[]; total: number }> {
  const query = new URLSearchParams({
    page: String(params.page),
    pageSize: String(params.pageSize),
    search: params.search || '',
    status: params.status || 'all',
  })
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/invitations?${query.toString()}`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<{ invitations: Invitation[]; total: number }>(res)
}

export async function apiRevokeInvitation(accessToken: string, id: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/invitations/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiResendInvitation(accessToken: string, id: string): Promise<{ message: string }> {
  const res = await csrfFetch(`${AUTH_API_BASE}/admin/invitations/${encodeURIComponent(id)}/resend`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<{ message: string }>(res)
}


