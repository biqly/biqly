import { useId, useMemo } from 'react'

import { useT } from '../../i18n'
import { pageRange, pageSizeSelectOptions } from '../../utils/paging'
import { PaginationControls } from './PaginationControls'
import { Select } from './Select'

interface PaginationProps {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
  totalItems?: number
  itemsPerPage?: number
  alwaysShow?: boolean
  pageSizeOptions?: readonly number[]
  onPageSizeChange?: (size: number) => void
  pageSizeLabel?: string
}

export function Pagination({
  currentPage,
  totalPages,
  onPageChange,
  totalItems,
  itemsPerPage,
  alwaysShow = false,
  pageSizeOptions,
  onPageSizeChange,
  pageSizeLabel,
}: PaginationProps) {
  const t = useT()
  const pageSizeSelectId = useId()

  const safeTotalPages = Math.max(1, totalPages)
  const hasItems = totalItems === undefined || totalItems > 0
  const showPageSizeSelector =
    pageSizeOptions !== undefined &&
    pageSizeOptions.length > 0 &&
    onPageSizeChange !== undefined &&
    itemsPerPage !== undefined
  const sizeSelectOptions = useMemo(
    () => (pageSizeOptions ? pageSizeSelectOptions(pageSizeOptions) : []),
    [pageSizeOptions],
  )

  if (!hasItems) {
    return null
  }
  if (!alwaysShow && safeTotalPages <= 1 && !showPageSizeSelector) {
    return null
  }

  const { start, end } =
    totalItems !== undefined && itemsPerPage !== undefined
      ? pageRange(currentPage, itemsPerPage, totalItems)
      : { start: 0, end: 0 }
  const singlePage = safeTotalPages <= 1
  const resolvedPageSizeLabel = pageSizeLabel ?? t('common.pagination.page_size')

  return (
    <div
      className={`border-border flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3`}
    >
      <div className="text-foreground-muted text-caption flex flex-wrap items-center gap-3">
        <span>
          {totalItems !== undefined && itemsPerPage !== undefined
            ? t('common.pagination.range_of_total', { start, end, total: totalItems })
            : t('common.pagination.page_number', { page: currentPage })}
        </span>
        {showPageSizeSelector && (
          <label htmlFor={pageSizeSelectId} className="flex items-center gap-2">
            <span>{resolvedPageSizeLabel}</span>
            <Select
              id={pageSizeSelectId}
              value={String(itemsPerPage)}
              options={sizeSelectOptions}
              onChange={(v) => onPageSizeChange(Number(v))}
              size="sm"
              className="w-18"
              ariaLabel={resolvedPageSizeLabel}
            />
          </label>
        )}
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
