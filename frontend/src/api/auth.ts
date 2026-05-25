import type { AuthUser, PasskeyInfo, TokenResponse } from '../types/auth'

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

export async function apiRegister(email: string, password: string, displayName: string): Promise<TokenResponse> {
  const res = await fetch('/auth/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, display_name: displayName }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiLogin(email: string, password: string): Promise<TokenResponse> {
  const res = await fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiRefresh(refreshToken: string): Promise<TokenResponse> {
  const res = await fetch('/auth/refresh', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken }),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiLogout(refreshToken: string): Promise<void> {
  const res = await fetch('/auth/logout', {
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
  const res = await fetch('/auth/me', {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<AuthUser>(res)
}

export async function apiPasskeyRegisterBegin(accessToken: string): Promise<any> {
  const res = await fetch('/auth/passkey/register-begin', {
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
  let url = '/auth/passkey/register-finish'
  if (name) {
    url += `?name=${encodeURIComponent(name)}`
  }
  const res = await fetch(url, {
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
  const res = await fetch('/auth/passkey/login-begin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  return handleResponse<any>(res)
}

export async function apiPasskeyLoginFinish(credential: any): Promise<TokenResponse> {
  const res = await fetch('/auth/passkey/login-finish', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(credential),
  })
  return handleResponse<TokenResponse>(res)
}

export async function apiGetPasskeys(accessToken: string): Promise<PasskeyInfo[]> {
  const res = await fetch('/auth/me/passkeys', {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${accessToken}`,
    },
  })
  return handleResponse<PasskeyInfo[]>(res)
}

export async function apiDeletePasskey(accessToken: string, id: string): Promise<void> {
  const res = await fetch(`/auth/me/passkeys/${id}`, {
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
  const res = await fetch('/auth/forgot-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email }),
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiResetPassword(token: string, password: string): Promise<{ message: string }> {
  const res = await fetch('/auth/reset-password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, password }),
  })
  return handleResponse<{ message: string }>(res)
}

export async function apiVerifyEmail(token: string): Promise<{ message: string }> {
  const res = await fetch(`/auth/verify-email?token=${encodeURIComponent(token)}`, {
    method: 'GET',
  })
  return handleResponse<{ message: string }>(res)
}

