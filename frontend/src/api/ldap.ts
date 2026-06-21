import { apiFetch } from './apiClient'
import { AUTH_API_BASE } from './constants'

export type LDAPSecurity = 'none' | 'starttls' | 'ldaps'

export interface LDAPConfig {
  enabled: boolean
  auto_create_users: boolean
  host: string
  port: number
  security: LDAPSecurity
  skip_tls_verify: boolean
  bind_dn: string
  has_bind_password: boolean
  base_dn: string
  user_filter: string
  email_attr: string
  display_name_attr: string
  updated_at?: string
}

// LDAPConfigInput is the write shape: bind_password is set only when non-empty
// (blank keeps the stored secret).
export interface LDAPConfigInput {
  enabled: boolean
  auto_create_users: boolean
  host: string
  port: number
  security: LDAPSecurity
  skip_tls_verify: boolean
  bind_dn: string
  bind_password: string
  base_dn: string
  user_filter: string
  email_attr: string
  display_name_attr: string
}

export async function getLDAPConfig(token: string): Promise<LDAPConfig> {
  return apiFetch<LDAPConfig>('GET', `${AUTH_API_BASE}/admin/ldap-config`, undefined, { token })
}

export async function updateLDAPConfig(token: string, cfg: LDAPConfigInput): Promise<LDAPConfig> {
  return apiFetch<LDAPConfig>('PUT', `${AUTH_API_BASE}/admin/ldap-config`, cfg, { token })
}

export async function testLDAPConnection(
  token: string,
  cfg: LDAPConfigInput,
): Promise<{ status: string; message?: string }> {
  return apiFetch<{ status: string; message?: string }>(
    'POST',
    `${AUTH_API_BASE}/admin/ldap-config/test`,
    cfg,
    { token },
  )
}
