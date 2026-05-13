import type { ReactNode } from 'react'

interface KPICardProps {
  label: string
  value: ReactNode
  color?: string
  className?: string
}

export function KPICard({ label, value, color = 'var(--accent)', className }: KPICardProps) {
  return (
    <div className={'kpi-card' + (className ? ` ${className}` : '')} style={{ borderColor: color }}>
      <div className="kpi-label">{label}</div>
      <div className="kpi-value" style={{ color }}>{value}</div>
    </div>
  )
}
