export type PageToken = number | 'gap'

const MAX_WINDOW = 7

/**
 * Compact page window: shows every page when the count is small, otherwise a
 * sliding window with ellipsis — width grows with visible pages, not total pages.
 */
export function buildStablePageTokens(current: number, total: number): PageToken[] {
  if (total <= MAX_WINDOW) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  if (current <= 4) {
    return [1, 2, 3, 4, 5, 'gap', total]
  }
  if (current >= total - 3) {
    return [1, 'gap', total - 4, total - 3, total - 2, total - 1, total]
  }
  return [1, 'gap', current - 1, current, current + 1, 'gap', total]
}
