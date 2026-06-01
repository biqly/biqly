import type { AuthUser, PasskeyInfo, PasswordPolicy, SetActiveWorkspaceResponse, TokenResponse, Invitation } from '../types/auth'
import { apiFetch } from './apiClient'

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
      const data = await apiFetch<PasswordPolicy>('GET', `${AUTH_API_BASE}/password-policy`)
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

export function normalizeAuthUser(raw: any): AuthUser {
  return {
    id: raw.id,
    email: raw.email,
    username: raw.username,
    displayName: raw.displayName ?? raw.display_name,
    avatarUrl: raw.avatarUrl ?? raw.avatar_url,
    isActive: raw.isActive ?? raw.is_active,
    emailVerified: raw.emailVerified ?? raw.email_verified,
    hasPassword: raw.hasPassword ?? raw.has_password,
    active_workspace_id: raw.active_workspace_id,
    createdAt: raw.createdAt ?? raw.created_at,
    updatedAt: raw.updatedAt ?? raw.updated_at,
  }
}

export async function apiRegister(email: string, password: string, displayName: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/register`, { email, password, display_name: displayName })
}

export async function apiLogin(email: string, password: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/login`, { email, password })
}

export async function apiOAuthExchange(code: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/oauth/exchange`, { code })
}

export async function apiRefresh(refreshToken: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/refresh`, { refresh_token: refreshToken })
}

export async function apiLogout(refreshToken: string): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/logout`, { refresh_token: refreshToken })
}

export async function apiGetMe(accessToken: string): Promise<AuthUser> {
  const data = await apiFetch<any>('GET', `${AUTH_API_BASE}/me`, undefined, { token: accessToken })
  return normalizeAuthUser(data)
}

export async function apiUpdateProfile(accessToken: string, displayName: string): Promise<AuthUser> {
  const data = await apiFetch<any>('PATCH', `${AUTH_API_BASE}/me/profile`, { display_name: displayName }, { token: accessToken })
  return normalizeAuthUser(data)
}

export async function apiChangePassword(accessToken: string, currentPassword: string, newPassword: string): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/me/password`, {
    current_password: currentPassword,
    new_password: newPassword,
  }, { token: accessToken })
}

export async function apiRequestEmailChange(accessToken: string, newEmail: string): Promise<{ message?: string }> {
  return apiFetch<{ message?: string }>('POST', `${AUTH_API_BASE}/me/email-change/request`, { new_email: newEmail }, { token: accessToken })
}

export async function apiGenerateMFABypassSelf(accessToken: string): Promise<{ bypass_code: string }> {
  return apiFetch<{ bypass_code: string }>('POST', `${AUTH_API_BASE}/me/mfa/bypass`, undefined, { token: accessToken })
}

export async function apiSetActiveWorkspace(accessToken: string, workspaceID: string): Promise<SetActiveWorkspaceResponse> {
  return apiFetch<SetActiveWorkspaceResponse>('POST', `${AUTH_API_BASE}/me/active-workspace`, { workspace_id: workspaceID }, { token: accessToken })
}

export async function apiPasskeyRegisterBegin(accessToken: string): Promise<any> {
  return apiFetch<any>('POST', `${AUTH_API_BASE}/passkey/register-begin`, undefined, { token: accessToken })
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
  return apiFetch<{ status: string }>('POST', url, credential, { token: accessToken })
}

export async function apiPasskeyLoginBegin(email?: string): Promise<any> {
  return apiFetch<any>('POST', `${AUTH_API_BASE}/passkey/login-begin`, email ? { email } : {})
}

export async function apiPasskeyLoginFinish(credential: any): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/passkey/login-finish`, credential)
}

export async function apiGetPasskeys(accessToken: string): Promise<PasskeyInfo[]> {
  return apiFetch<PasskeyInfo[]>('GET', `${AUTH_API_BASE}/me/passkeys`, undefined, { token: accessToken })
}

export async function apiDeletePasskey(accessToken: string, id: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/me/passkeys/${id}`, undefined, { token: accessToken })
}

export async function apiPasskeyRename(
  accessToken: string,
  id: string,
  name: string
): Promise<void> {
  await apiFetch<void>('PATCH', `${AUTH_API_BASE}/me/passkeys/${id}`, { name }, { token: accessToken })
}

export async function apiForgotPassword(email: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('POST', `${AUTH_API_BASE}/forgot-password`, { email })
}

export async function apiResetPassword(token: string, password: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('POST', `${AUTH_API_BASE}/reset-password`, { token, password })
}

export async function apiVerifyEmail(token: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('GET', `${AUTH_API_BASE}/verify-email?token=${encodeURIComponent(token)}`)
}

export async function apiInviteUser(accessToken: string, email: string, roleName: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('POST', `${AUTH_API_BASE}/admin/invitations`, { email, role_name: roleName }, { token: accessToken })
}

export async function apiGetInvitation(token: string): Promise<{
  id: string
  email: string
  role_id: string
  role_name: string
  invited_by: string
  expires_at: string
}> {
  return apiFetch<any>('GET', `${AUTH_API_BASE}/invitations/${encodeURIComponent(token)}`)
}

export async function apiClaimInvitation(token: string, password: string, displayName: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/invitations/${encodeURIComponent(token)}/claim`, { password, display_name: displayName })
}

export async function apiListInvitations(
  accessToken: string,
  params: { page: number; pageSize: number; search?: string; status?: string }
): Promise<{ invitations: Invitation[]; total: number }> {
  const query = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
    search: params.search || '',
    status: params.status || 'all',
  })
  return apiFetch<{ invitations: Invitation[]; total: number }>('GET', `${AUTH_API_BASE}/admin/invitations?${query.toString()}`, undefined, { token: accessToken })
}

export async function apiRevokeInvitation(accessToken: string, id: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('DELETE', `${AUTH_API_BASE}/admin/invitations/${encodeURIComponent(id)}`, undefined, { token: accessToken })
}

export async function apiResendInvitation(accessToken: string, id: string): Promise<{ message: string }> {
  return apiFetch<{ message: string }>('POST', `${AUTH_API_BASE}/admin/invitations/${encodeURIComponent(id)}/resend`, undefined, { token: accessToken })
}

export async function apiMFAStatus(accessToken: string): Promise<{
  enabled: boolean
  method?: string
  verified_at?: string
}> {
  return apiFetch<{ enabled: boolean; method?: string; verified_at?: string }>('GET', `${AUTH_API_BASE}/mfa/status`, undefined, { token: accessToken })
}

export async function apiMFAEnroll(accessToken: string): Promise<{
  secret: string
  otpauth_url: string
  recovery_codes: string[]
}> {
  return apiFetch<{ secret: string; otpauth_url: string; recovery_codes: string[] }>('POST', `${AUTH_API_BASE}/mfa/enroll`, undefined, { token: accessToken })
}

export async function apiMFAVerify(accessToken: string, code: string): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/mfa/verify`, { code }, { token: accessToken })
}

export async function apiMFADisable(accessToken: string, code: string): Promise<void> {
  await apiFetch<void>('POST', `${AUTH_API_BASE}/mfa/disable`, { code }, { token: accessToken })
}

export async function apiMFALogin(mfaToken: string, code: string): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('POST', `${AUTH_API_BASE}/mfa/login`, { mfa_token: mfaToken, code })
}

export async function apiMFARegenerateRecovery(accessToken: string, code: string): Promise<{ recovery_codes: string[] }> {
  return apiFetch<{ recovery_codes: string[] }>('POST', `${AUTH_API_BASE}/mfa/recovery/regenerate`, { code }, { token: accessToken })
}
