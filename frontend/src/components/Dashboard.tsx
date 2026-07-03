import { useState } from 'react'

import { useT } from '../i18n'
import { cn } from '../lib/cn'
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
        <div className="border-border mb-2 flex gap-4 border-b pb-[0.2rem] max-sm:flex-col max-sm:gap-1 max-sm:border-b-0">
          <button
            type="button"
            className={cn(
              'cursor-pointer border-0 border-b-2 bg-transparent px-4 py-2 text-[1rem] transition-all duration-200',
              activeTab === 'custom'
                ? 'border-accent text-foreground font-semibold'
                : 'text-foreground-muted hover:text-foreground border-transparent font-medium',
            )}
            onClick={() => setActiveTab('custom')}
          >
            {t('dashboard.tab_custom')}
          </button>
          <button
            type="button"
            className={cn(
              'cursor-pointer border-0 border-b-2 bg-transparent px-4 py-2 text-[1rem] transition-all duration-200',
              activeTab === 'ai_usage'
                ? 'border-accent text-foreground font-semibold'
                : 'text-foreground-muted hover:text-foreground border-transparent font-medium',
            )}
            onClick={() => setActiveTab('ai_usage')}
          >
            {t('dashboard.tab_ai_usage')}
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
