import '../../styles/data-state.css'

import type { ReactNode } from 'react'

import { ErrorAlert } from './ErrorAlert'
import { LoadingOverlay } from './LoadingOverlay'

interface DataStateProps {
  loading: boolean
  /** Rendered as an ErrorAlert banner above the content; content (stale rows) stays visible. */
  error?: string | null
  /** Prepended to the error message, e.g. t('common.error'). */
  errorPrefix?: string
  /** Whether the list is empty. Also reserves overlay space while loading an empty list. */
  empty?: boolean
  /**
   * Shown instead of children when empty and not loading (use an <EmptyState/>).
   * Omit it to keep rendering children when empty — e.g. a table that shows
   * its own placeholder row inside tbody (those move into DataTable in Faz 3).
   */
  emptyState?: ReactNode
  /** Extra class on the body wrapper (e.g. 'data-state__body--scroll-x'). */
  className?: string
  children: ReactNode
}

/**
 * Standard error → loading → empty → content composition for list screens
 * (Faz 2, tasks/frontend-table-pagination-standardization.md). Replaces the
 * per-screen error banner + LoadingOverlay + minHeight + empty ternary trees.
 */
export function DataState({
  loading,
  error,
  errorPrefix,
  empty = false,
  emptyState,
  className,
  children,
}: DataStateProps) {
  const bodyCls = [
    'data-state__body',
    empty && loading ? 'data-state__body--min' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <>
      {error ? <ErrorAlert error={errorPrefix ? `${errorPrefix}: ${error}` : error} /> : null}
      <LoadingOverlay loading={loading}>
        <div className={bodyCls}>
          {empty && emptyState !== undefined ? (loading ? null : emptyState) : children}
        </div>
      </LoadingOverlay>
    </>
  )
}
