import { useState } from 'react'

import DashboardBuilder from './DashboardBuilder'
import DashboardList from './DashboardList'

export default function Dashboards() {
  const [selectedDashboardId, setSelectedDashboardId] = useState<string | null>(null)

  return selectedDashboardId ? (
    <DashboardBuilder
      dashboardId={selectedDashboardId}
      onBack={() => setSelectedDashboardId(null)}
    />
  ) : (
    <DashboardList onSelect={(id) => setSelectedDashboardId(id)} />
  )
}
