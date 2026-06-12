import '../../styles/pagination.css'

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
    <div className="table-pagination">
      <div className="table-pagination__label">
        {totalItems !== undefined && itemsPerPage !== undefined
          ? t('common.pagination.range_of_total', { start, end, total: totalItems })
          : t('common.pagination.page_number', { page: currentPage })}
      </div>
      <PaginationControls
        currentPage={currentPage}
        totalPages={safeTotalPages}
        onPageChange={onPageChange}
        disabled={singlePage}
        size="sm"
      />
    </div>
  )
}
