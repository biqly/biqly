/**
 * Shared 1-based pagination math. Single policy for the whole app
 * (tasks/frontend-table-pagination-standardization.md, Faz 0.1 / OQ-1):
 * totalPages is never below 1, so an empty list still renders "page 1 of 1".
 * The Pagination component already defends with Math.max(1, totalPages), so
 * adopting this in screens does not change visible behavior.
 */
export function getTotalPages(totalItems: number, pageSize: number): number {
  if (pageSize <= 0 || !Number.isFinite(pageSize) || !Number.isFinite(totalItems)) {
    return 1
  }
  return Math.max(1, Math.ceil(totalItems / pageSize))
}

export function clampPage(page: number, totalPages: number): number {
  return Math.min(Math.max(1, page), Math.max(1, totalPages))
}

/**
 * "start–end of total" range for the current page, matching the inline math
 * in components/ui/Pagination.tsx. An empty list yields {start: 0, end: 0}
 * (same convention as tableBrowser derivePaging).
 */
export function pageRange(
  page: number,
  pageSize: number,
  totalItems: number,
): { start: number; end: number } {
  if (totalItems <= 0) {
    return { start: 0, end: 0 }
  }
  return {
    start: (page - 1) * pageSize + 1,
    end: Math.min(page * pageSize, totalItems),
  }
}

export const DEFAULT_TABLE_PAGE_SIZE_OPTIONS = [5, 10, 25, 50] as const

export function pageSizeSelectOptions(
  values: readonly number[],
): { value: string; label: string }[] {
  return values.map((n) => ({ value: String(n), label: String(n) }))
}
