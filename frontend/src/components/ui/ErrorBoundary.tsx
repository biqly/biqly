import { Component, type ErrorInfo, type ReactNode } from 'react'

import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { emptyStateClass } from '../../lib/feedbackClasses'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

function ErrorFallback({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const t = useT()
  return (
    <section className={cn(legacyCardClass('card'), emptyStateClass)} role="alert">
      <h2>{t('common.error_boundary_title')}</h2>
      <p>{error.message || t('common.error_boundary_fallback_message')}</p>
      <button type="button" className={legacyButtonClass('btn btn-sm')} onClick={onRetry}>
        {t('common.error_boundary_retry')}
      </button>
    </section>
  )
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  override state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Unhandled UI error', error, info.componentStack)
  }

  override render() {
    if (!this.state.error) {
      return this.props.children
    }

    return <ErrorFallback error={this.state.error} onRetry={() => this.setState({ error: null })} />
  }
}
