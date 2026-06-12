/** Shared 1-based pagination contracts (tasks/frontend-table-pagination-standardization.md, Faz 1). */

export interface PageQuery {
  page: number
  pageSize: number
}

/**
 * Normalized list-page result. Backend endpoints return `{ <resourceKey>: T[], total }`
 * with varying keys (users, entries, access, ...); each screen's fetcher maps its
 * endpoint shape into this one. The API contract itself stays untouched.
 */
export interface PagedResult<T> {
  items: T[]
  total: number
}
