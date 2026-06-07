import { ADMIN_TAB_LABEL_KEYS, type AdminTab, isAdminTab } from '../components/admin/adminNavConfig'
import type { Crumb } from '../components/ui/Breadcrumbs'
import type { TFunction } from '../i18n'

function adminTabLabel(tabParam: string, t: TFunction): string {
  if (!isAdminTab(tabParam)) {
    return ''
  }
  return t(ADMIN_TAB_LABEL_KEYS[tabParam])
}

export function appendAdminBreadcrumbs(
  crumbs: Crumb[],
  search: string,
  t: TFunction,
  navigate: (path: string) => void,
): void {
  const params = new URLSearchParams(search)
  const tabParam = params.get('tab') ?? 'users'
  const tabLabel = adminTabLabel(tabParam, t)
  if (!tabLabel) {
    return
  }

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
    const label = params.get('userLabel')?.trim() ?? t('admin.user_detail.title')
    crumbs.push({ label })
    return
  }

  if (tabParam === 'workspaces' && workspaceId) {
    const label = params.get('workspaceLabel')?.trim() ?? t('admin.workspaces.settings')
    crumbs.push({ label })
  }
}
