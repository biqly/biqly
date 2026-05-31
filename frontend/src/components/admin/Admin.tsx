import { lazy, Suspense } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'
import { useQueryParam } from '../../hooks/useQueryParam'

const RolesPanel = lazy(() => import('./RolesPanel').then(m => ({ default: m.RolesPanel })))
const DatasourceAccessPanel = lazy(() => import('./DatasourceAccessPanel').then(m => ({ default: m.DatasourceAccessPanel })))
const WorkspacesPanel = lazy(() => import('./WorkspacesPanel').then(m => ({ default: m.WorkspacesPanel })))
const AuditLogPanel = lazy(() => import('./AuditLogPanel').then(m => ({ default: m.AuditLogPanel })))
const UserListPage = lazy(() => import('./UserListPage').then(m => ({ default: m.UserListPage })))
const UserDetailPage = lazy(() => import('./UserDetailPage').then(m => ({ default: m.UserDetailPage })))
const AIHistoryPanel = lazy(() => import('../ai/AIHistoryPanel').then(m => ({ default: m.AIHistoryPanel })))
const SharedResourcesList = lazy(() => import('../sharing/SharedResourcesList').then(m => ({ default: m.SharedResourcesList })))

type AdminTab = 'users' | 'roles' | 'datasource_access' | 'workspaces' | 'ai_history' | 'sharing' | 'audit_log'

const pendingStyle: React.CSSProperties = { padding: 24 }
const layoutStyle: React.CSSProperties = { padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }
const tabBarStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.04))',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: '8px',
  padding: '4px',
  gap: '4px',
  width: 'fit-content',
  flexWrap: 'wrap',
}
const tabBtnBase: React.CSSProperties = {
  padding: '6px 12px',
  border: '1px solid',
  borderRadius: '6px',
  fontSize: '13px',
  cursor: 'pointer',
  transition: 'all 150ms cubic-bezier(0.4, 0, 0.2, 1)',
}
const tabBtnActive: React.CSSProperties = {
  ...tabBtnBase,
  background: 'var(--bg-card, #18181b)',
  borderColor: 'var(--border-strong, rgba(255, 255, 255, 0.12))',
  color: 'var(--text-primary, #f4f4f5)',
  fontWeight: 600,
  boxShadow: 'var(--shadow-sm, 0 4px 12px -2px rgba(0, 0, 0, 0.3))',
}
const tabBtnInactive: React.CSSProperties = {
  ...tabBtnBase,
  background: 'transparent',
  borderColor: 'transparent',
  color: 'var(--text-secondary, #a1a1aa)',
  fontWeight: 500,
  boxShadow: 'none',
}

export default function Admin() {
  const t = useT()
  const { accessToken } = useAuth()
  const [tabParam, setTabParam] = useQueryParam('tab')
  const [userIdParam, setUserIdParam] = useQueryParam('userId')
  const [, setSubTabParam] = useQueryParam('subTab')

  const tab = (tabParam as AdminTab) || 'users'
  const selectedUserID = userIdParam || null

  if (!accessToken) {
    return <div style={pendingStyle}>{t('admin.auth_pending')}</div>
  }

  const handleTabChange = (newTab: AdminTab) => {
    setTabParam(newTab)
    setUserIdParam('') // reset selection when switching tabs
    setSubTabParam('') // reset sub tab selection when switching main tabs
  }

  return (
    <div style={layoutStyle}>
      <div style={tabBarStyle}>
        <TabButton active={tab === 'users'} onClick={() => handleTabChange('users')}>{t('admin.tabs.users')}</TabButton>
        <TabButton active={tab === 'roles'} onClick={() => handleTabChange('roles')}>{t('admin.tabs.roles')}</TabButton>
        <TabButton active={tab === 'datasource_access'} onClick={() => handleTabChange('datasource_access')}>{t('admin.tabs.datasource_access')}</TabButton>
        <TabButton active={tab === 'workspaces'} onClick={() => handleTabChange('workspaces')}>{t('admin.tabs.workspaces')}</TabButton>
        <TabButton active={tab === 'ai_history'} onClick={() => handleTabChange('ai_history')}>{t('admin.ai_history.title')}</TabButton>
        <TabButton active={tab === 'sharing'} onClick={() => handleTabChange('sharing')}>{t('admin.sharing.title')}</TabButton>
        <TabButton active={tab === 'audit_log'} onClick={() => handleTabChange('audit_log')}>{t('admin.tabs.audit_log')}</TabButton>
      </div>

      <div style={{ position: 'relative', minHeight: '200px' }}>
        <Suspense
          fallback={
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '3rem', minHeight: '200px' }}>
              <div className="spinner" style={{ width: '32px', height: '32px', borderTopColor: 'var(--accent, #6366f1)' }} />
            </div>
          }
        >
          {tab === 'users' && (
            selectedUserID ? (
              <UserDetailPage token={accessToken} userID={selectedUserID} onBack={() => setUserIdParam('')} />
            ) : (
              <UserListPage token={accessToken} onSelectUser={(id) => setUserIdParam(id)} />
            )
          )}
          {tab === 'roles' && <RolesPanel token={accessToken} />}
          {tab === 'datasource_access' && <DatasourceAccessPanel token={accessToken} />}
          {tab === 'workspaces' && <WorkspacesPanel token={accessToken} />}
          {tab === 'ai_history' && <AIHistoryPanel />}
          {tab === 'sharing' && <SharedResourcesList />}
          {tab === 'audit_log' && <AuditLogPanel token={accessToken} />}
        </Suspense>
      </div>
    </div>
  )
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      style={active ? tabBtnActive : tabBtnInactive}
    >
      {children}
    </button>
  )
}
