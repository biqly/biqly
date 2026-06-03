export interface AuthUser {
  id: string
  email: string
  username?: string
  displayName?: string
  avatarUrl?: string
  isActive: boolean
  emailVerified: boolean
  hasPassword?: boolean
  mfaEnabled?: boolean
  mfaPending?: boolean
  passkeyCount?: number
  active_workspace_id?: string
  createdAt: string
  updatedAt: string
}

export interface SetActiveWorkspaceResponse {
  access_token: string
  active_workspace_id: string
}

export interface PasswordPolicy {
  min_length: number
  max_length: number
  require_upper: boolean
  require_lower: boolean
  require_digit: boolean
  require_special: boolean
  min_score: number
  self_signup_enabled?: boolean
  ldap_enabled?: boolean
}

export interface PlatformSettings {
  self_signup_enabled: boolean
  updated_at?: string
  updated_by?: string
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  user_id: string
  email: string
  roles: string[]
  mfa_required?: boolean
  mfa_token?: string
  password_expired?: boolean
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
  mfa_required: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface WorkspaceMember {
  workspace_id: string
  user_id: string
  email?: string
  display_name?: string | null
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

export interface WorkspaceDatasource {
  workspace_id: string
  datasource_id: string
  datasource_name?: string
  access_level: string
  attached_by?: string
  attached_at: string
}

export interface AIHistoryEntry {
  id: string
  datasource_id: string
  model_id?: string
  user_id?: string
  question: string
  prompt_context?: unknown
  ai_response?: unknown
  logical_query?: unknown
  confidence_score?: number
  warnings?: string[]
  outcome_status: string
  retry_count: number
  needs_clarification: boolean
  model_used?: string
  prompt_tokens?: number
  completion_tokens?: number
  token_count?: number
  cost_usd?: number
  latency_ms?: number
  created_at: string
}

export interface ResourceShare {
  id: string
  resource_type: string
  resource_id: string
  owner_id: string
  shared_with?: string
  workspace_id?: string
  permission: 'view' | 'execute' | 'edit'
  created_at: string
}

export interface Invitation {
  id: string
  email: string
  role_id: string
  role_name: string
  invited_by: string
  created_at: string
  expires_at: string
  claimed_at?: string
}
