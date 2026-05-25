export interface AuthUser {
  id: string
  email: string
  username?: string
  displayName?: string
  avatarUrl?: string
  isActive: boolean
  emailVerified: boolean
  createdAt: string
  updatedAt: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  user_id: string
  email: string
  roles: string[]
}

export interface PasskeyInfo {
  id: string
  name: string
  created_at: string
  last_used_at?: string
}
