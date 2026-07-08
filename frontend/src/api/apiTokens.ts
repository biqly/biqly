import { apiFetch } from './apiClient'
import { AUTH_API_BASE } from './constants'

// ApiToken is a personal access token's metadata — the plaintext value is
// never included here; it is only ever present once, on CreatedApiToken.
export interface ApiToken {
  id: string
  name: string
  token_prefix: string
  created_at: string
  expires_at?: string | null
  last_used_at?: string | null
}

export interface CreatedApiToken extends ApiToken {
  token: string
}

export interface CreateApiTokenPayload {
  name: string
  expiresInDays?: number
}

export async function listApiTokens(): Promise<ApiToken[]> {
  const data = await apiFetch<{ tokens: ApiToken[] | null }>('GET', `${AUTH_API_BASE}/me/tokens`)
  return data.tokens ?? []
}

export async function createApiToken(payload: CreateApiTokenPayload): Promise<CreatedApiToken> {
  return apiFetch<CreatedApiToken>('POST', `${AUTH_API_BASE}/me/tokens`, {
    name: payload.name,
    expires_in_days: payload.expiresInDays,
  })
}

export async function revokeApiToken(id: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AUTH_API_BASE}/me/tokens/${encodeURIComponent(id)}`)
}
