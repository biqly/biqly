import { useAuth } from '../auth/AuthProvider'
import { RolesPanel } from './RolesPanel'
import { DatasourceAccessPanel } from './DatasourceAccessPanel'
import { WorkspacesPanel } from './WorkspacesPanel'
import { AuditLogPanel } from './AuditLogPanel'
import { UserListPage } from './UserListPage'
import { UserDetailPage } from './UserDetailPage'
import { AIHistoryPanel } from '../ai/AIHistoryPanel'
import { SharedResourcesList } from '../sharing/SharedResourcesList'
import { useT } from '../../i18n'
import { useQueryParam } from '../../hooks/useQueryParam'

type AdminTab = 'users' | 'roles' | 'datasource_access' | 'workspaces' | 'ai_history' | 'sharing' | 'audit_log'

export default function Admin() {
  const t = useT()
  const { accessToken } = useAuth()
  const [tabParam, setTabParam] = useQueryParam('tab')
  const [userIdParam, setUserIdParam] = useQueryParam('userId')

  const tab = (tabParam as AdminTab) || 'users'
  const selectedUserID = userIdParam || null

  if (!accessToken) {
    return <div style={{ padding: 24 }}>{t('admin.auth_pending')}</div>
  }

  const handleTabChange = (newTab: AdminTab) => {
    setTabParam(newTab)
    setUserIdParam('') // reset selection when switching tabs
  }

  return (
    <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <div
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.04))',
          border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
          borderRadius: '8px',
          padding: '4px',
          gap: '4px',
          width: 'fit-content',
          flexWrap: 'wrap',
        }}
      >
        <TabButton active={tab === 'users'} onClick={() => handleTabChange('users')}>{t('admin.tabs.users')}</TabButton>
        <TabButton active={tab === 'roles'} onClick={() => handleTabChange('roles')}>{t('admin.tabs.roles')}</TabButton>
        <TabButton active={tab === 'datasource_access'} onClick={() => handleTabChange('datasource_access')}>{t('admin.tabs.datasource_access')}</TabButton>
        <TabButton active={tab === 'workspaces'} onClick={() => handleTabChange('workspaces')}>{t('admin.tabs.workspaces')}</TabButton>
        <TabButton active={tab === 'ai_history'} onClick={() => handleTabChange('ai_history')}>{t('admin.ai_history.title')}</TabButton>
        <TabButton active={tab === 'sharing'} onClick={() => handleTabChange('sharing')}>{t('admin.sharing.title')}</TabButton>
        <TabButton active={tab === 'audit_log'} onClick={() => handleTabChange('audit_log')}>{t('admin.tabs.audit_log')}</TabButton>
      </div>

      <div>
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
      </div>
    </div>
  )
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      style={{
        padding: '6px 12px',
        background: active ? 'var(--bg-card, #18181b)' : 'transparent',
        border: '1px solid',
        borderColor: active ? 'var(--border-strong, rgba(255, 255, 255, 0.12))' : 'transparent',
        color: active ? 'var(--text-primary, #f4f4f5)' : 'var(--text-secondary, #a1a1aa)',
        fontWeight: active ? 600 : 500,
        borderRadius: '6px',
        fontSize: '13px',
        cursor: 'pointer',
        boxShadow: active ? 'var(--shadow-sm, 0 4px 12px -2px rgba(0, 0, 0, 0.3))' : 'none',
        transition: 'all 150ms cubic-bezier(0.4, 0, 0.2, 1)',
      }}
    >
      {children}
    </button>
  )
}
