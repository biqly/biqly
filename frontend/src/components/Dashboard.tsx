import { useState } from 'react'

import { useT } from '../i18n'
import AIUsageDashboard from './AIUsageDashboard'
import DashboardBuilder from './DashboardBuilder'
import DashboardList from './DashboardList'

export default function Dashboard() {
  const t = useT()
  const [activeTab, setActiveTab] = useState<'custom' | 'ai_usage'>('custom')
  const [selectedDashboardId, setSelectedDashboardId] = useState<string | null>(null)

  // Sub-header selection layout
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      {/* Tab Selectors: Hide when a custom dashboard is actively open for editing */}
      {!selectedDashboardId && (
        <div
          style={{
            display: 'flex',
            borderBottom: '1px solid var(--border)',
            gap: '1rem',
            paddingBottom: '0.2rem',
            marginBottom: '0.5rem',
          }}
        >
          <button
            type="button"
            style={{
              background: 'none',
              border: 'none',
              borderBottom:
                activeTab === 'custom' ? '2px solid var(--accent)' : '2px solid transparent',
              color: activeTab === 'custom' ? 'var(--text)' : 'var(--text-muted)',
              cursor: 'pointer',
              fontSize: '1rem',
              fontWeight: activeTab === 'custom' ? 600 : 500,
              padding: '0.5rem 1rem',
              transition: 'all 0.2s',
            }}
            onClick={() => setActiveTab('custom')}
          >
            📊 Custom Dashboards
          </button>
          <button
            type="button"
            style={{
              background: 'none',
              border: 'none',
              borderBottom:
                activeTab === 'ai_usage' ? '2px solid var(--accent)' : '2px solid transparent',
              color: activeTab === 'ai_usage' ? 'var(--text)' : 'var(--text-muted)',
              cursor: 'pointer',
              fontSize: '1rem',
              fontWeight: activeTab === 'ai_usage' ? 600 : 500,
              padding: '0.5rem 1rem',
              transition: 'all 0.2s',
            }}
            onClick={() => setActiveTab('ai_usage')}
          >
            🤖 AI Analytics
          </button>
        </div>
      )}

      {/* Conditional rendering based on tab and selection */}
      {activeTab === 'custom' ? (
        selectedDashboardId ? (
          <DashboardBuilder
            dashboardId={selectedDashboardId}
            onBack={() => setSelectedDashboardId(null)}
          />
        ) : (
          <DashboardList onSelect={(id) => setSelectedDashboardId(id)} />
        )
      ) : (
        <AIUsageDashboard />
      )}
    </div>
  )
}
