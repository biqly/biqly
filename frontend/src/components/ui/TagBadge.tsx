import type { ReactNode } from 'react'

interface TagBadgeProps {
  children: ReactNode
  tone?: 'default' | 'success' | 'warning' | 'error'
  className?: string
}

const toneClass = {
  default: '',
  success: ' success',
  warning: ' warning',
  error: ' error',
}

export function TagBadge({ children, tone = 'default', className }: TagBadgeProps) {
  return (
    <span className={`status-badge${toneClass[tone]}${className ? ` ${className}` : ''}`}>
      {children}
    </span>
  )
}
