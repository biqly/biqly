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

export interface Role {
  id: string
  name: string
  description?: string
  created_at: string
}

export interface Permission {
  id: string
  name: string
  description?: string
  resource: string
  action: string
  created_at: string
}

export interface Workspace {
  id: string
  name: string
  slug: string
  description?: string
  is_personal: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface WorkspaceMember {
  workspace_id: string
  user_id: string
  role_id: string
  role_name?: string
  joined_at: string
  invited_by?: string
}

export interface DatasourceAccess {
  id: string
  user_id: string
  datasource_id: string
  access_level: 'read' | 'write' | 'admin'
  granted_by?: string
  granted_at: string
}

export interface AuditLogEntry {
  id: string
  user_id?: string
  action: string
  resource?: string
  resource_id?: string
  metadata?: unknown
  ip_address?: string
  created_at: string
}

export interface AIQueueStatus {
  total_pending: number
  my_position?: number
  my_job_id?: string
  my_job_status: string
}

export interface UserRoleInfo {
  role_id: string
  role_name: string
  scope_type: string
  scope_id: string
}
