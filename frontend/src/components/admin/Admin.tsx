import { useState } from 'react'
import { useAuth } from '../auth/AuthProvider'
import { RolesPanel } from './RolesPanel'
import { DatasourceAccessPanel } from './DatasourceAccessPanel'
import { WorkspacesPanel } from './WorkspacesPanel'

type AdminTab = 'roles' | 'datasource_access' | 'workspaces'

export default function Admin() {
  const { accessToken } = useAuth()
  const [tab, setTab] = useState<AdminTab>('roles')

  if (!accessToken) {
    return <div style={{ padding: 24 }}>Yetkilendirme bekleniyor…</div>
  }

  return (
    <div style={{ padding: 24, display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h1 style={{ margin: 0 }}>Yönetim</h1>
      <div style={{ display: 'flex', gap: 8, borderBottom: '1px solid var(--border-color, #e5e7eb)' }}>
        <TabButton active={tab === 'roles'} onClick={() => setTab('roles')}>Roller & İzinler</TabButton>
        <TabButton active={tab === 'datasource_access'} onClick={() => setTab('datasource_access')}>Datasource Erişimi</TabButton>
        <TabButton active={tab === 'workspaces'} onClick={() => setTab('workspaces')}>Workspace'ler</TabButton>
      </div>

      <div>
        {tab === 'roles' && <RolesPanel token={accessToken} />}
        {tab === 'datasource_access' && <DatasourceAccessPanel token={accessToken} />}
        {tab === 'workspaces' && <WorkspacesPanel token={accessToken} />}
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
