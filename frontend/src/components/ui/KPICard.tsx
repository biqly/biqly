import type { ReactNode } from 'react'

import { cn } from '../../lib/cn'

interface KPICardProps {
  label: string
  value: ReactNode
  color?: string
  className?: string
}

export function KPICard({ label, value, color = 'var(--accent)', className }: KPICardProps) {
  return (
    <div
      className={cn('bg-card flex flex-col gap-1 rounded-lg border p-4', className)}
      style={{ borderColor: color }}
      role="group"
      aria-label={label}
    >
      <div className="text-foreground-muted text-sm">{label}</div>
      <div className="text-2xl font-semibold" style={{ color }}>
        {value}
      </div>
    </div>
  )
}
