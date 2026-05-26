import { useT } from '../../i18n'

interface PaginationProps {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  totalItems?: number
  itemsPerPage?: number
}

export function Pagination({
  currentPage,
  totalPages,
  onPageChange,
  totalItems,
  itemsPerPage,
}: PaginationProps) {
  const t = useT()

  if (totalPages <= 1) return null

  const start = totalItems !== undefined && itemsPerPage !== undefined ? (currentPage - 1) * itemsPerPage + 1 : 0
  const end = totalItems !== undefined && itemsPerPage !== undefined ? Math.min(currentPage * itemsPerPage, totalItems) : 0

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '12px 16px',
        borderTop: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
        flexWrap: 'wrap',
        gap: 12,
      }}
    >
      <div style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
        {totalItems !== undefined && itemsPerPage !== undefined
          ? t('table_browser.range_of_total', { start, end, total: totalItems })
          : t('table_browser.page_number', { page: currentPage })}
      </div>

      <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
        <button
          type="button"
          onClick={() => onPageChange(1)}
          disabled={currentPage === 1}
          style={currentPage === 1 ? btnDisabled : btnActive}
          title={t('table_browser.first_page')}
        >
          «
        </button>
        <button
          type="button"
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage === 1}
          style={currentPage === 1 ? btnDisabled : btnActive}
        >
          {t('table_browser.prev_page')}
        </button>

        <span style={{ fontSize: 13, color: 'var(--text-primary, #f4f4f5)', margin: '0 8px' }}>
          {currentPage} / {totalPages}
        </span>

        <button
          type="button"
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage === totalPages}
          style={currentPage === totalPages ? btnDisabled : btnActive}
        >
          {t('table_browser.next_page')}
        </button>
        <button
          type="button"
          onClick={() => onPageChange(totalPages)}
          disabled={currentPage === totalPages}
          style={currentPage === totalPages ? btnDisabled : btnActive}
          title={t('table_browser.last_page')}
        >
          »
        </button>
      </div>
    </div>
  )
}

const btnActive: React.CSSProperties = {
  padding: '6px 12px',
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.08))',
  color: 'var(--text-primary, #f4f4f5)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
  transition: 'all 150ms',
}

const btnDisabled: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  color: 'var(--text-muted, #8a8a92)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  cursor: 'not-allowed',
  fontSize: 13,
  fontWeight: 500,
  opacity: 0.5,
}
