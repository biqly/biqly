import type { QueryColumnFormat } from '../../types/ai'

/**
 * Display-only relabeling for calendar time-grain dimensions.
 *
 * Month and quarter grains compile to EXTRACT ordinals on the backend
 * (month-of-year 1-12, quarter-of-year 1-4), so the raw cell/category value is
 * an integer. This turns that ordinal into a localized name for the chart axis
 * and table cell while the caller keeps the original integer for sorting and
 * drill-down.
 *
 * Returns `null` when the format is not a grain ordinal or the value is out of
 * range / non-numeric, so callers can fall back to the raw value.
 */

const monthFormatters = new Map<string, Intl.DateTimeFormat>()

function monthFormatter(localeTag: string): Intl.DateTimeFormat {
  const cached = monthFormatters.get(localeTag)
  if (cached) {
    return cached
  }
  // Empty tag → runtime default locale (undefined).
  const fmt = new Intl.DateTimeFormat(localeTag || undefined, { month: 'long' })
  monthFormatters.set(localeTag, fmt)
  return fmt
}

export function formatGrainValue(
  format: QueryColumnFormat | undefined,
  value: unknown,
  localeTag: string,
): string | null {
  if (format !== 'month_of_year' && format !== 'quarter') {
    return null
  }
  if (typeof value !== 'number' && typeof value !== 'string') {
    return null
  }
  const n = Number(value)
  if (!Number.isInteger(n)) {
    return null
  }
  if (format === 'month_of_year') {
    if (n < 1 || n > 12) {
      return null
    }
    // Day 1 avoids month-length edge cases; only the month field is rendered.
    return monthFormatter(localeTag).format(new Date(2000, n - 1, 1))
  }
  // quarter
  if (n < 1 || n > 4) {
    return null
  }
  return localeTag.toLowerCase().startsWith('tr') ? `${n}. Çeyrek` : `Q${n}`
}
