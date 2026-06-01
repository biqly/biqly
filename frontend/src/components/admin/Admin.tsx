import { lazy, Suspense } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'
import { useQueryParam } from '../../hooks/useQueryParam'
import { LoadingScreen } from '../ui/LoadingScreen'
import { AdminNav } from './AdminNav'
import { isAdminTab, type AdminTab } from './adminNavConfig'

const RolesPanel = lazy(() => import('./RolesPanel').then(m => ({ default: m.RolesPanel })))
const DatasourceAccessPanel = lazy(() => import('./DatasourceAccessPanel').then(m => ({ default: m.DatasourceAccessPanel })))
const WorkspacesPanel = lazy(() => import('./WorkspacesPanel').then(m => ({ default: m.WorkspacesPanel })))
const AuditLogPanel = lazy(() => import('./AuditLogPanel').then(m => ({ default: m.AuditLogPanel })))
const UserListPage = lazy(() => import('./UserListPage').then(m => ({ default: m.UserListPage })))
const UserDetailPage = lazy(() => import('./UserDetailPage').then(m => ({ default: m.UserDetailPage })))
const AIHistoryPanel = lazy(() => import('../ai/AIHistoryPanel').then(m => ({ default: m.AIHistoryPanel })))
const SharedResourcesList = lazy(() => import('../sharing/SharedResourcesList').then(m => ({ default: m.SharedResourcesList })))
const AIProvidersPanel = lazy(() => import('./AIProvidersPanel').then(m => ({ default: m.AIProvidersPanel })))
const RowLevelSecurityPanel = lazy(() => import('./RowLevelSecurityPanel').then(m => ({ default: m.RowLevelSecurityPanel })))
const FieldPermissionPanel = lazy(() => import('./FieldPermissionPanel').then(m => ({ default: m.FieldPermissionPanel })))

const pendingStyle: React.CSSProperties = { padding: 24 }

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

  if (!accessToken) {
    return <div style={pendingStyle}>{t('admin.auth_pending')}</div>
  }

  const handleTabChange = (newTab: AdminTab) => {
    setTabParam(newTab)
    setUserIdParam('')
    setUserLabelParam('')
    setWorkspaceIdParam('')
    setWorkspaceLabelParam('')
    setSubTabParam('')
  }

  return (
    <div className="admin-layout">
      <AdminNav activeTab={tab} onTabChange={handleTabChange} />

      <div className="admin-content">
        <Suspense fallback={<LoadingScreen minHeight="200px" />}>
          {tab === 'users' && (
            selectedUserID ? (
              <UserDetailPage
                token={accessToken}
                userID={selectedUserID}
                onBack={() => {
                  setUserIdParam('')
                  setUserLabelParam('')
                }}
              />
            ) : (
              <UserListPage
                token={accessToken}
                onSelectUser={(id, label) => {
                  setUserIdParam(id)
                  setUserLabelParam(label)
                }}
              />
            )
          )}
          {tab === 'roles' && <RolesPanel token={accessToken} />}
          {tab === 'datasource_access' && <DatasourceAccessPanel token={accessToken} />}
          {tab === 'workspaces' && <WorkspacesPanel token={accessToken} />}
          {tab === 'ai_history' && <AIHistoryPanel />}
          {tab === 'sharing' && <SharedResourcesList />}
          {tab === 'audit_log' && <AuditLogPanel token={accessToken} />}
          {tab === 'ai_providers' && <AIProvidersPanel />}
          {tab === 'row_level_security' && <RowLevelSecurityPanel token={accessToken} />}
          {tab === 'field_permissions' && <FieldPermissionPanel token={accessToken} />}
        </Suspense>
      </div>
    </div>
  )
}
