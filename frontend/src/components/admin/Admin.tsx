import {
  type ComponentType,
  lazy,
  type LazyExoticComponent,
  startTransition,
  Suspense,
  useEffect,
} from 'react'

import { useQueryParam } from '../../hooks/useQueryParam'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import { adminContentClass, adminLayoutClass } from './adminClasses'
import { AdminNav } from './AdminNav'
import { type AdminTab, isAdminTab } from './adminNavConfig'

type PreloadableComponent<T extends ComponentType> = LazyExoticComponent<T> & {
  preload: () => Promise<{ default: T }>
}

interface AdminLazyPanel {
  preload: () => Promise<unknown>
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- React.lazy + named exports; props differ per panel
const lazyWithPreload = <T extends ComponentType<any>>(
  factory: () => Promise<{ default: T }>,
): PreloadableComponent<T> => {
  const Component = lazy(factory) as PreloadableComponent<T>
  Component.preload = factory
  return Component
}

const RolesPanel = lazyWithPreload(() =>
  import('./RolesPanel').then((m) => ({ default: m.RolesPanel })),
)
const DatasourceAccessPanel = lazyWithPreload(() =>
  import('./DatasourceAccessPanel').then((m) => ({ default: m.DatasourceAccessPanel })),
)
const WorkspacesPanel = lazyWithPreload(() =>
  import('./WorkspacesPanel').then((m) => ({ default: m.WorkspacesPanel })),
)
const AuditLogPanel = lazyWithPreload(() =>
  import('./AuditLogPanel').then((m) => ({ default: m.AuditLogPanel })),
)
const UserListPage = lazyWithPreload(() =>
  import('./UserListPage').then((m) => ({ default: m.UserListPage })),
)
const UserDetailPage = lazyWithPreload(() =>
  import('./UserDetailPage').then((m) => ({ default: m.UserDetailPage })),
)
const AIHistoryPanel = lazyWithPreload(() =>
  import('../ai/AIHistoryPanel').then((m) => ({ default: m.AIHistoryPanel })),
)
const AIUsageAdminPanel = lazyWithPreload(() =>
  import('./AIUsageAdminPanel').then((m) => ({ default: m.AIUsageAdminPanel })),
)
const AIJobsAdminPanel = lazyWithPreload(() =>
  import('./AIJobsAdminPanel').then((m) => ({ default: m.AIJobsAdminPanel })),
)
const SharedResourcesList = lazyWithPreload(() =>
  import('../sharing/SharedResourcesList').then((m) => ({ default: m.SharedResourcesList })),
)
const AIProvidersPanel = lazyWithPreload(() =>
  import('./AIProvidersPanel').then((m) => ({ default: m.AIProvidersPanel })),
)
const RowLevelSecurityPanel = lazyWithPreload(() =>
  import('./RowLevelSecurityPanel').then((m) => ({ default: m.RowLevelSecurityPanel })),
)
const FieldPermissionPanel = lazyWithPreload(() =>
  import('./FieldPermissionPanel').then((m) => ({ default: m.FieldPermissionPanel })),
)
const PIIDetectionPanel = lazyWithPreload(() =>
  import('./PIIDetectionPanel').then((m) => ({ default: m.PIIDetectionPanel })),
)
const PlatformSettingsPanel = lazyWithPreload(() =>
  import('./PlatformSettingsPanel').then((m) => ({ default: m.PlatformSettingsPanel })),
)
const LDAPSettingsPanel = lazyWithPreload(() =>
  import('./LDAPSettingsPanel').then((m) => ({ default: m.LDAPSettingsPanel })),
)
const ABExperimentPanel = lazyWithPreload(() =>
  import('./ABExperimentPanel').then((m) => ({ default: m.ABExperimentPanel })),
)
const ConfirmedQueriesPanel = lazyWithPreload(() =>
  import('./ConfirmedQueriesPanel').then((m) => ({ default: m.ConfirmedQueriesPanel })),
)
const QueryAuditPanel = lazyWithPreload(() =>
  import('./QueryAuditPanel').then((m) => ({ default: m.QueryAuditPanel })),
)

const pendingStyle: React.CSSProperties = { padding: 24 }

// Panels that take no props render through this map; panels needing the auth
// token (or extra wiring) keep explicit branches in Admin below.
const PROPLESS_TAB_PANELS: Partial<Record<AdminTab, ComponentType>> = {
  ai_usage: AIUsageAdminPanel,
  ai_jobs: AIJobsAdminPanel,
  ai_history: AIHistoryPanel,
  sharing: SharedResourcesList,
  ai_providers: AIProvidersPanel,
  ai_ab_experiments: ABExperimentPanel,
  ai_confirmed: ConfirmedQueriesPanel,
}

const TAB_COMPONENTS: Record<AdminTab, AdminLazyPanel> = {
  users: UserListPage,
  roles: RolesPanel,
  datasource_access: DatasourceAccessPanel,
  workspaces: WorkspacesPanel,
  ai_usage: AIUsageAdminPanel,
  ai_jobs: AIJobsAdminPanel,
  ai_history: AIHistoryPanel,
  sharing: SharedResourcesList,
  audit_log: AuditLogPanel,
  ai_providers: AIProvidersPanel,
  row_level_security: RowLevelSecurityPanel,
  field_permissions: FieldPermissionPanel,
  pii_detection: PIIDetectionPanel,
  ldap: LDAPSettingsPanel,
  platform_settings: PlatformSettingsPanel,
  ai_ab_experiments: ABExperimentPanel,
  ai_confirmed: ConfirmedQueriesPanel,
  query_audit: QueryAuditPanel,
}

export default function Admin() {
  const t = useT()
  const { accessToken } = useAuth()
  const [tabParam, setTabParam] = useQueryParam('tab')
  const [userIdParam, setUserIdParam] = useQueryParam('userId')
  const [, setUserLabelParam] = useQueryParam('userLabel')
  const [, setWorkspaceIdParam] = useQueryParam('workspaceId')
  const [, setWorkspaceLabelParam] = useQueryParam('workspaceLabel')
  const [, setSubTabParam] = useQueryParam('subTab')

  const tab: AdminTab = isAdminTab(tabParam) ? tabParam : 'users'
  const selectedUserID = userIdParam || null
  const ProplessPanel = PROPLESS_TAB_PANELS[tab]

  const handleTabHover = (hoveredTab: AdminTab) => {
    void TAB_COMPONENTS[hoveredTab].preload()
    if (hoveredTab === 'users') {
      void UserDetailPage.preload()
    }
  }

  useEffect(() => {
    const timer = setTimeout(() => {
      Object.values(TAB_COMPONENTS).forEach((comp) => {
        void comp.preload()
      })
      void UserDetailPage.preload()
    }, 1500)
    return () => clearTimeout(timer)
  }, [])

  if (!accessToken) {
    return <div style={pendingStyle}>{t('admin.auth_pending')}</div>
  }

  const handleTabChange = (newTab: AdminTab) => {
    startTransition(() => {
      setTabParam(newTab)
      setUserIdParam('')
      setUserLabelParam('')
      setWorkspaceIdParam('')
      setWorkspaceLabelParam('')
      setSubTabParam('')
    })
  }

  return (
    <div className={adminLayoutClass}>
      <AdminNav activeTab={tab} onTabChange={handleTabChange} onTabHover={handleTabHover} />

      <div className={adminContentClass}>
        <Suspense fallback={<LoadingScreen minHeight="200px" />}>
          {tab === 'users' &&
            (selectedUserID ? (
              <UserDetailPage token={accessToken} userID={selectedUserID} />
            ) : (
              <UserListPage
                token={accessToken}
                onSelectUser={(id, label) => {
                  setUserIdParam(id)
                  setUserLabelParam(label)
                }}
              />
            ))}
          {tab === 'roles' && <RolesPanel token={accessToken} />}
          {tab === 'datasource_access' && <DatasourceAccessPanel token={accessToken} />}
          {tab === 'workspaces' && <WorkspacesPanel token={accessToken} />}
          {tab === 'audit_log' && <AuditLogPanel token={accessToken} />}
          {tab === 'query_audit' && <QueryAuditPanel token={accessToken} />}
          {tab === 'row_level_security' && <RowLevelSecurityPanel token={accessToken} />}
          {tab === 'field_permissions' && <FieldPermissionPanel token={accessToken} />}
          {tab === 'pii_detection' && <PIIDetectionPanel token={accessToken} />}
          {tab === 'ldap' && <LDAPSettingsPanel token={accessToken} />}
          {tab === 'platform_settings' && <PlatformSettingsPanel token={accessToken} />}
          {ProplessPanel && <ProplessPanel />}
        </Suspense>
      </div>
    </div>
  )
}
