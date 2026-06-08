import { useT } from '../../i18n'
import { PaginationControls } from './PaginationControls'

interface PaginationProps {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  totalItems?: number
  itemsPerPage?: number
  alwaysShow?: boolean
}

export function Pagination({
  currentPage,
  totalPages,
  onPageChange,
  totalItems,
  itemsPerPage,
  alwaysShow = false,
}: PaginationProps) {
  const t = useT()

  const safeTotalPages = Math.max(1, totalPages)
  const hasItems = totalItems === undefined || totalItems > 0

  if (!hasItems) {
    return null
  }
  if (!alwaysShow && safeTotalPages <= 1) {
    return null
  }

  const start =
    totalItems !== undefined && itemsPerPage !== undefined
      ? (currentPage - 1) * itemsPerPage + 1
      : 0
  const end =
    totalItems !== undefined && itemsPerPage !== undefined
      ? Math.min(currentPage * itemsPerPage, totalItems)
      : 0
  const singlePage = safeTotalPages <= 1

  return (
    <div
      className="table-pagination"
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
      <PaginationControls
        currentPage={currentPage}
        safeTotalPages={safeTotalPages}
        singlePage={singlePage}
        onPageChange={onPageChange}
        prevLabel={t('table_browser.prev_page')}
        nextLabel={t('table_browser.next_page')}
        firstTitle={t('table_browser.first_page')}
        lastTitle={t('table_browser.last_page')}
      />
    </div>
  )
}
