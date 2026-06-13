import type { ReactNode } from 'react'

import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import {
  loadingOverlayClass,
  loadingOverlaySpinnerClass,
  loadingOverlayWrapClass,
  loadingTextClass,
} from '../../lib/feedbackClasses'

interface LoadingOverlayProps {
  loading: boolean
  label?: string
  children?: ReactNode
  className?: string
}

export function LoadingOverlay({ loading, label, children, className }: LoadingOverlayProps) {
  const t = useT()
  const displayLabel = label ?? t('common.loading')
  if (!loading) {
    return children ? <>{children}</> : null
  }
  if (children) {
    return (
      <div className={cn(loadingOverlayWrapClass, className)} aria-busy="true">
        {children}
        <div className={loadingOverlayClass} role="status" aria-live="polite">
          <span className={loadingOverlaySpinnerClass} aria-hidden="true" />
          <span>{displayLabel}</span>
        </div>
      </div>
    )
  }
  return (
    <p className={cn(loadingTextClass, className)} role="status" aria-live="polite">
      {displayLabel}
    </p>
  )
}
