import type { TranslationKey } from '../../i18n'

export type AdminTab =
  | 'users'
  | 'roles'
  | 'datasource_access'
  | 'workspaces'
  | 'ai_history'
  | 'ai_jobs'
  | 'ai_usage'
  | 'sharing'
  | 'audit_log'
  | 'ai_providers'
  | 'row_level_security'
  | 'field_permissions'
  | 'pii_detection'
  | 'ldap'
  | 'platform_settings'
  | 'ai_ab_experiments'
  | 'ai_confirmed'
  | 'ai_lexicon'
  | 'query_audit'
  | 'mcp'
  | 'reports'

export const ADMIN_TAB_LABEL_KEYS = {
  users: 'admin.tabs.users',
  roles: 'admin.tabs.roles',
  datasource_access: 'admin.tabs.datasource_access',
  workspaces: 'admin.tabs.workspaces',
  ai_history: 'admin.ai_history.title',
  ai_jobs: 'admin.ai_jobs.title',
  ai_usage: 'admin.ai_usage.title',
  sharing: 'admin.sharing.title',
  audit_log: 'admin.tabs.audit_log',
  ai_providers: 'admin.tabs.ai_providers',
  row_level_security: 'admin.tabs.row_level_security',
  field_permissions: 'admin.tabs.field_permissions',
  pii_detection: 'admin.tabs.pii_detection',
  ldap: 'admin.tabs.ldap',
  platform_settings: 'admin.tabs.platform_settings',
  ai_ab_experiments: 'admin.tabs.ai_ab_experiments',
  ai_confirmed: 'admin.tabs.ai_confirmed',
  ai_lexicon: 'admin.tabs.ai_lexicon',
  query_audit: 'admin.tabs.query_audit',
  mcp: 'admin.tabs.mcp',
  reports: 'admin.tabs.reports',
} as const satisfies Record<AdminTab, TranslationKey>

export interface AdminNavGroup {
  id: string
  labelKey: TranslationKey
  tabs: AdminTab[]
}

export const ADMIN_NAV_GROUPS: AdminNavGroup[] = [
  {
    id: 'access',
    labelKey: 'admin.nav.access',
    tabs: ['users', 'roles', 'workspaces', 'datasource_access'],
  },
  {
    id: 'security',
    labelKey: 'admin.nav.security',
    tabs: ['row_level_security', 'field_permissions', 'pii_detection', 'ldap'],
  },
  {
    id: 'ai',
    labelKey: 'admin.nav.ai',
    tabs: [
      'ai_providers',
      'ai_jobs',
      'ai_usage',
      'ai_history',
      'ai_confirmed',
      'ai_lexicon',
      'ai_ab_experiments',
    ],
  },
  {
    id: 'sharing',
    labelKey: 'admin.nav.sharing_reports',
    tabs: ['sharing', 'reports'],
  },
  {
    id: 'compliance',
    labelKey: 'admin.nav.compliance',
    tabs: ['audit_log', 'query_audit', 'mcp', 'platform_settings'],
  },
]

export const ADMIN_TABS = ADMIN_NAV_GROUPS.flatMap((g) => g.tabs)

export function isAdminTab(value: string): value is AdminTab {
  return ADMIN_TABS.includes(value as AdminTab)
}

/** Category that owns a tab — drives the primary tab bar for deep links like ?tab=ldap. */
export function adminGroupForTab(tab: AdminTab): AdminNavGroup {
  return ADMIN_NAV_GROUPS.find((g) => g.tabs.includes(tab)) ?? ADMIN_NAV_GROUPS[0]!
}
