import type { TranslationKey } from '../../i18n'

export type AdminTab =
  | 'users'
  | 'roles'
  | 'datasource_access'
  | 'workspaces'
  | 'ai_history'
  | 'sharing'
  | 'audit_log'
  | 'ai_providers'
  | 'row_level_security'
  | 'field_permissions'
  | 'platform_settings'

export const ADMIN_TAB_LABEL_KEYS = {
  users: 'admin.tabs.users',
  roles: 'admin.tabs.roles',
  datasource_access: 'admin.tabs.datasource_access',
  workspaces: 'admin.tabs.workspaces',
  ai_history: 'admin.ai_history.title',
  sharing: 'admin.sharing.title',
  audit_log: 'admin.tabs.audit_log',
  ai_providers: 'admin.tabs.ai_providers',
  row_level_security: 'admin.tabs.row_level_security',
  field_permissions: 'admin.tabs.field_permissions',
  platform_settings: 'admin.tabs.platform_settings',
} as const satisfies Record<AdminTab, TranslationKey>

export const ADMIN_NAV_GROUPS: { id: string; labelKey: TranslationKey; tabs: AdminTab[] }[] = [
  {
    id: 'access',
    labelKey: 'admin.nav.access',
    tabs: ['users', 'roles', 'workspaces', 'datasource_access'],
  },
  {
    id: 'security',
    labelKey: 'admin.nav.security',
    tabs: ['row_level_security', 'field_permissions'],
  },
  {
    id: 'ai',
    labelKey: 'admin.nav.ai',
    tabs: ['ai_providers', 'ai_history', 'sharing'],
  },
  {
    id: 'compliance',
    labelKey: 'admin.nav.compliance',
    tabs: ['audit_log', 'platform_settings'],
  },
]

export const ADMIN_TABS = ADMIN_NAV_GROUPS.flatMap((g) => g.tabs)

export function isAdminTab(value: string): value is AdminTab {
  return ADMIN_TABS.includes(value as AdminTab)
}
