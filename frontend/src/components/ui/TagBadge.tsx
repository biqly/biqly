import type { ReactNode } from 'react'

import { cn } from '../../lib/cn'

interface TagBadgeProps {
  children: ReactNode
  tone?: 'default' | 'success' | 'warning' | 'error'
  className?: string
  ariaLabel?: string
}

export function TagBadge({ children, tone = 'default', className, ariaLabel }: TagBadgeProps) {
  return (
    <span
      className={cn('status-badge', tone !== 'default' && tone, className)}
      aria-label={ariaLabel}
    >
      {children}
    </span>
  )
}
