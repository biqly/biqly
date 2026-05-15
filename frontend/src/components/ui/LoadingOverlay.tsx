import type { ReactNode } from 'react'

interface LoadingOverlayProps {
  loading: boolean
  label?: string
  children?: ReactNode
  className?: string
}

export function LoadingOverlay({ loading, label = 'Yükleniyor…', children, className }: LoadingOverlayProps) {
  if (!loading) return children ? <>{children}</> : null
  if (children) {
    return (
      <div className={`loading-overlay-wrap${className ? ` ${className}` : ''}`} aria-busy="true">
        {children}
        <div className="loading-overlay" role="status" aria-live="polite">
          <span className="loading-overlay-spinner" aria-hidden="true" />
          <span className="loading-overlay-label">{label}</span>
        </div>
      </div>
    )
  }
  return (
    <p className={`loading-text${className ? ` ${className}` : ''}`} role="status" aria-live="polite">
      {label}
    </p>
  )
}
