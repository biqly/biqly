import type { ReactNode } from 'react'
import clsx from 'clsx'

interface TagBadgeProps {
  children: ReactNode
  tone?: 'default' | 'success' | 'warning' | 'error'
  className?: string
  ariaLabel?: string
}

export function TagBadge({ children, tone = 'default', className, ariaLabel }: TagBadgeProps) {
  return (
    <span className={clsx('status-badge', tone !== 'default' && tone, className)} aria-label={ariaLabel}>
      {children}
    </span>
  )
}
