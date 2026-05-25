import { useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { RolesPanel } from './RolesPanel'
import { DatasourceAccessPanel } from './DatasourceAccessPanel'
import { WorkspacesPanel } from './WorkspacesPanel'
import { AuditLogPanel } from './AuditLogPanel'
import { UserListPage } from './UserListPage'
import { UserDetailPage } from './UserDetailPage'
import { useT } from '../../i18n'

type AdminTab = 'users' | 'roles' | 'datasource_access' | 'workspaces' | 'audit_log'

export default function Admin() {
  const t = useT()
  const { accessToken } = useAuth()
  const [tab, setTab] = useState<AdminTab>('users')
  const [selectedUserID, setSelectedUserID] = useState<string | null>(null)

  if (!accessToken) {
    return <div style={{ padding: 24 }}>{t('admin.auth_pending')}</div>
  }

  const handleTabChange = (newTab: AdminTab) => {
    setTab(newTab)
    setSelectedUserID(null) // reset selection when switching tabs
  }

  return (
    <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h1 style={{ margin: 0 }}>{t('admin.title')}</h1>
      <div style={{ display: 'flex', gap: 8, borderBottom: '1px solid var(--border-color, #e5e7eb)' }}>
        <TabButton active={tab === 'users'} onClick={() => handleTabChange('users')}>{t('admin.tabs.users')}</TabButton>
        <TabButton active={tab === 'roles'} onClick={() => handleTabChange('roles')}>{t('admin.tabs.roles')}</TabButton>
        <TabButton active={tab === 'datasource_access'} onClick={() => handleTabChange('datasource_access')}>{t('admin.tabs.datasource_access')}</TabButton>
        <TabButton active={tab === 'workspaces'} onClick={() => handleTabChange('workspaces')}>{t('admin.tabs.workspaces')}</TabButton>
        <TabButton active={tab === 'audit_log'} onClick={() => handleTabChange('audit_log')}>{t('admin.tabs.audit_log')}</TabButton>
      </div>

      <div>
        {tab === 'users' && (
          selectedUserID ? (
            <UserDetailPage token={accessToken} userID={selectedUserID} onBack={() => setSelectedUserID(null)} />
          ) : (
            <UserListPage token={accessToken} onSelectUser={(id) => setSelectedUserID(id)} />
          )
        )}
        {tab === 'roles' && <RolesPanel token={accessToken} />}
        {tab === 'datasource_access' && <DatasourceAccessPanel token={accessToken} />}
        {tab === 'workspaces' && <WorkspacesPanel token={accessToken} />}
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
        padding: '8px 14px',
        background: 'transparent',
        border: 0,
        borderBottom: active ? '2px solid var(--accent, #4f46e5)' : '2px solid transparent',
        color: active ? 'var(--accent, #4f46e5)' : 'var(--text-secondary, #4b5563)',
        fontWeight: active ? 600 : 400,
        cursor: 'pointer',
      }}
    >
      {children}
    </button>
  )
}
