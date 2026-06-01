import type { useT } from '../i18n'
import type { Crumb } from '../components/ui/Breadcrumbs'

type TFunction = ReturnType<typeof useT>

const ADMIN_TAB_LABEL_KEYS = {
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
} as const satisfies Record<string, string>

type AdminTabKey = keyof typeof ADMIN_TAB_LABEL_KEYS

function adminTabLabel(tabParam: string, t: TFunction): string {
  const key = ADMIN_TAB_LABEL_KEYS[tabParam as AdminTabKey]
  return key ? t(key) : ''
}

export function appendAdminBreadcrumbs(
  crumbs: Crumb[],
  search: string,
  t: TFunction,
  navigate: (path: string) => void,
): void {
  const params = new URLSearchParams(search)
  const tabParam = params.get('tab') || 'users'
  const tabLabel = adminTabLabel(tabParam, t)
  if (!tabLabel) return

  const userId = params.get('userId')
  const workspaceId = params.get('workspaceId')
  const subTab = params.get('subTab')

  const hasSubDetail =
    (tabParam === 'users' && !!userId) ||
    (tabParam === 'workspaces' && !!workspaceId) ||
    (tabParam === 'users' && subTab === 'invitations')

  const goTab = () => {
    const next = new URLSearchParams()
    next.set('tab', tabParam)
    navigate(`/admin?${next.toString()}`)
  }

  crumbs.push({
    label: tabLabel,
    onClick: hasSubDetail ? goTab : undefined,
  })

  if (tabParam === 'users' && subTab === 'invitations') {
    crumbs.push({ label: t('auth.invitations_tab') })
    return
  }

  if (tabParam === 'users' && userId) {
    const label = params.get('userLabel')?.trim() || t('admin.user_detail.title')
    crumbs.push({ label })
    return
  }

  if (tabParam === 'workspaces' && workspaceId) {
    const label = params.get('workspaceLabel')?.trim() || t('admin.workspaces.settings')
    crumbs.push({ label })
  }
}
