import type { ReactNode } from 'react'
import { useT } from '../../i18n'
import { LoadingIndicator } from './LoadingIndicator'

interface LoadingOverlayProps {
  loading: boolean
  label?: string
  children?: ReactNode
  className?: string
}

export function LoadingOverlay({ loading, label, children, className }: LoadingOverlayProps) {
  const t = useT()
  const displayLabel = label ?? t('common.loading')
  if (!loading) return children ? <>{children}</> : null
  if (children) {
    return (
      <div className={`loading-overlay-wrap${className ? ` ${className}` : ''}`} aria-busy="true">
        {children}
        <div className="loading-overlay loading-overlay--subtle" role="status" aria-live="polite">
          <LoadingIndicator label={displayLabel} size="sm" className="loading-indicator--anchored" />
        </div>
      </div>
    )
  }
  return (
    <p className={`loading-text${className ? ` ${className}` : ''}`} role="status" aria-live="polite">
      {displayLabel}
    </p>
  )
}
