import clsx from 'clsx'

import { useT } from '../../i18n'
import { buildStablePageTokens } from './paginationTokens'

/**
 * Site-wide page navigation: « ‹ [numbered window] › ».
 * Pages are 1-based. Width follows visible page tokens only.
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

  const getBtnCls = (isActive: boolean) =>
    clsx(
      'inline-flex items-center justify-center border rounded-[0.4rem] text-foreground [font-variant-numeric:tabular-nums] transition-colors duration-120 ease-out focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-1',
      size === 'sm'
        ? 'min-h-[1.6rem] min-w-[1.6rem] text-[0.74rem] py-[0.15rem] px-[0.35rem]'
        : 'min-h-[1.9rem] min-w-[1.9rem] text-[0.8rem] py-1 px-[0.45rem]',
      isActive
        ? 'bg-accent border-accent text-white font-semibold cursor-default opacity-100 disabled:text-white disabled:opacity-100'
        : 'border-border bg-transparent enabled:hover:border-accent disabled:text-foreground-faint disabled:cursor-not-allowed disabled:opacity-[0.55]',
    )

  return (
    <nav className="inline-flex items-center gap-1" aria-label={t('table_browser.pagination_nav')}>
      <button
        type="button"
        className={getBtnCls(false)}
        disabled={disabled || current === 1}
        onClick={() => onPageChange(1)}
        aria-label={t('table_browser.first_page')}
        title={t('table_browser.first_page')}
      >
        «
      </button>
      <button
        type="button"
        className={getBtnCls(false)}
        disabled={disabled || current === 1}
        onClick={() => onPageChange(current - 1)}
        aria-label={t('table_browser.prev_page')}
        title={t('table_browser.prev_page')}
      >
        ‹
      </button>
      {tokens.map((token, idx) => {
        if (token === 'gap') {
          return (
            <span
              key={`gap-${idx}`}
              className="inline-flex items-center justify-center px-0.5 text-foreground-faint text-[0.8rem] select-none"
              aria-hidden="true"
            >
              …
            </span>
          )
        }
        return (
          <button
            key={token}
            type="button"
            className={getBtnCls(token === current)}
            disabled={disabled || token === current}
            onClick={() => onPageChange(token)}
            aria-label={t('table_browser.go_to_page', { page: token })}
            aria-current={token === current ? 'page' : undefined}
          >
            {fmt(token)}
          </button>
        )
      })}
      <button
        type="button"
        className={getBtnCls(false)}
        disabled={disabled || current === total}
        onClick={() => onPageChange(current + 1)}
        aria-label={t('table_browser.next_page')}
        title={t('table_browser.next_page')}
      >
        ›
      </button>
      <button
        type="button"
        className={getBtnCls(false)}
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
