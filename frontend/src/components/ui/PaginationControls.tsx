import '../../styles/pagination.css'

import { useT } from '../../i18n'
import { buildStablePageTokens } from './paginationTokens'

/**
 * Site-wide page navigation: « ‹ [stable numbered window] › ».
 * Pages are 1-based. Slot width follows the widest page number so the layout
 * never shifts while paging.
 */
export function PaginationControls({
  currentPage,
  totalPages,
  onPageChange,
  disabled = false,
  size = 'md',
  formatNumber,
}: {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  disabled?: boolean
  size?: 'sm' | 'md'
  formatNumber?: (n: number) => string
}) {
  const t = useT()
  const total = Math.max(1, totalPages)
  const current = Math.min(Math.max(1, currentPage), total)
  const fmt = formatNumber ?? ((n: number) => String(n))
  const tokens = buildStablePageTokens(current, total)
  const slotCh = `${Math.max(2, fmt(total).length) + 1.6}ch`

  return (
    <nav
      className={`page-nav${size === 'sm' ? ' page-nav--sm' : ''}`}
      style={{ '--page-nav-slot': slotCh } as React.CSSProperties}
      aria-label={t('table_browser.pagination_nav')}
    >
      <button
        type="button"
        className="page-nav__btn"
        disabled={disabled || current === 1}
        onClick={() => onPageChange(1)}
        aria-label={t('table_browser.first_page')}
        title={t('table_browser.first_page')}
      >
        «
      </button>
      <button
        type="button"
        className="page-nav__btn"
        disabled={disabled || current === 1}
        onClick={() => onPageChange(current - 1)}
        aria-label={t('table_browser.prev_page')}
        title={t('table_browser.prev_page')}
      >
        ‹
      </button>
      {tokens.map((token, idx) => {
        if (token === 'pad') {
          return <span key={`pad-${idx}`} className="page-nav__gap" aria-hidden="true" />
        }
        if (token === 'gap') {
          return (
            <span key={`gap-${idx}`} className="page-nav__gap" aria-hidden="true">
              …
            </span>
          )
        }
        return (
          <span key={token} className="page-nav__slot">
            <button
              type="button"
              className={`page-nav__btn${token === current ? ' page-nav__btn--active' : ''}`}
              style={{ width: '100%' }}
              disabled={disabled || token === current}
              onClick={() => onPageChange(token)}
              aria-label={t('table_browser.go_to_page', { page: token })}
              aria-current={token === current ? 'page' : undefined}
            >
              {fmt(token)}
            </button>
          </span>
        )
      })}
      <button
        type="button"
        className="page-nav__btn"
        disabled={disabled || current === total}
        onClick={() => onPageChange(current + 1)}
        aria-label={t('table_browser.next_page')}
        title={t('table_browser.next_page')}
      >
        ›
      </button>
      <button
        type="button"
        className="page-nav__btn"
        disabled={disabled || current === total}
        onClick={() => onPageChange(total)}
        aria-label={t('table_browser.last_page')}
        title={t('table_browser.last_page')}
      >
        »
      </button>
    </nav>
  )
}
