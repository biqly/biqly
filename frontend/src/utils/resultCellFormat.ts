/**
 * Formats query result cells for display: by default integers with grouping (thousand separators),
 * rounded. Fractional digits only when the user explicitly asks for them (natural-language hint).
 */

export interface FormatResultCellOptions {
  /** Natural-language question; used to detect requests for decimal / fractional display. */
  question?: string
}

const MAX_NUMBER_FORMAT_CACHE_SIZE = 24
const FRACTION_HINT_PATTERNS: RegExp[] = [
  /\b\d+\s*(?:ondalık|decimal|hane|place|places)\b/,
  /\b(?:show|with|include|use)\s+.*\bdecimal\b/,
  /\bondalık\b/,
  /\bküsürat/,
  /\bfractional\b/,
  /\bvirgülden sonra\b/,
  /\bnoktadan sonra\b/,
  /\bkuruş/,
  /\bcents?\b/,
  /\bprecise\b.*\b(amount|total|value|sum|avg|average)\b/,
  /\bexact\b.*\b(amount|total|value)\b/,
  /\bto\s+\d+\s+(?:decimal|place)/,
  /\b\d+\s+digits?\s+after\b/,
]
const numberFormatCache = new Map<string, Intl.NumberFormat>()

function getNumberFormat(options: Intl.NumberFormatOptions): Intl.NumberFormat {
  const key = JSON.stringify(options)
  const cached = numberFormatCache.get(key)
  if (cached) {
    return cached
  }
  const formatter = new Intl.NumberFormat(undefined, options)
  if (numberFormatCache.size >= MAX_NUMBER_FORMAT_CACHE_SIZE) {
    const oldest = numberFormatCache.keys().next().value
    if (oldest) {
      numberFormatCache.delete(oldest)
    }
  }
  numberFormatCache.set(key, formatter)
  return formatter
}

function parseNumeric(value: unknown): number | null {
  if (value === null || value === undefined) {
    return null
  }
  if (typeof value === 'boolean') {
    return null
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'bigint') {
    return Number(value)
  }
  if (typeof value === 'string') {
    const t = value.trim()
    if (t === '') {
      return null
    }
    const n = Number(t)
    if (!Number.isNaN(n) && Number.isFinite(n)) {
      return n
    }
  }
  return null
}

/** Surrogate keys and numeric IDs: no thousand separators (e.g. 11091 not 11,091). */
function isIdentifierLikeColumn(name: string): boolean {
  const u = name.toUpperCase()
  if (
    u === 'ID' ||
    u === 'ROW' ||
    u === 'YEAR' ||
    u === 'MONTH' ||
    u === 'QUARTER' ||
    u === 'WEEK'
  ) {
    return false
  }
  if (u.endsWith('_ID') || u.endsWith('_KEY')) {
    return true
  }
  if (u.includes('ENTITYID')) {
    return true
  }
  // Camel/snake keys: customerid, orderid, territoryid (length avoids GRID, VALID, …)
  if (u.length >= 7 && u.endsWith('ID') && /[A-Z]ID$/.test(u)) {
    return true
  }
  return false
}

/** Calendar / period bucket columns: never use thousand separators (e.g. 2022 not 2,022). */
function isCalendarIntColumn(name: string): boolean {
  const u = name.toUpperCase()
  return (
    u.endsWith('_YEAR') ||
    u.endsWith('_MONTH') ||
    u.endsWith('_QUARTER') ||
    u.endsWith('_WEEK') ||
    u.endsWith('_ISO_WEEK') ||
    u.endsWith('_WEEK_OF_YEAR') ||
    u === 'YEAR' ||
    u === 'MONTH' ||
    u === 'QUARTER' ||
    u === 'WEEK' ||
    u === 'ISO_WEEK'
  )
}

function maxFractionDigitsFromQuestion(q: string): number | null {
  const lower = q.toLowerCase()
  const m = /(\d+)\s*(?:ondalık|decimal|hane|place|places|digits?)\b/.exec(lower)
  if (m?.[1]) {
    return Math.min(10, Math.max(1, parseInt(m[1], 10)))
  }
  return null
}

function looksLikeIsoDateTime(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}T/.test(value)) {
    return false
  }
  const d = new Date(value)
  return !Number.isNaN(d.getTime())
}

function formatNonNumericCell(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value)
  }
  if (typeof value === 'object' && value !== null) {
    return JSON.stringify(value)
  }
  return ''
}

function wantsFractionalDisplay(question: string | undefined): boolean {
  if (!question?.trim()) {
    return false
  }
  const q = question.toLowerCase()
  return FRACTION_HINT_PATTERNS.some((p) => p.test(q))
}

/**
 * Renders a single result cell for HTML tables. Non-numeric values pass through as strings.
 */
export function formatResultCell(
  value: unknown,
  columnName: string,
  options: FormatResultCellOptions = {},
): string {
  if (value === null || value === undefined) {
    return ''
  }

  if (typeof value === 'string' && looksLikeIsoDateTime(value)) {
    const d = new Date(value)
    if (!isNaN(d.getTime())) {
      const hasTime =
        !value.endsWith('T00:00:00Z') &&
        !value.endsWith('T00:00:00') &&
        !value.includes('T00:00:00.000')
      try {
        return new Intl.DateTimeFormat(undefined, {
          year: 'numeric',
          month: 'short',
          day: 'numeric',
          ...(hasTime ? { hour: '2-digit', minute: '2-digit' } : {}),
        }).format(d)
      } catch {
        // ignore and fallback
      }
    }
  }

  const n = parseNumeric(value)
  if (n === null) {
    return formatNonNumericCell(value)
  }

  const calendarInt = isCalendarIntColumn(columnName)
  const idLike = isIdentifierLikeColumn(columnName)
  const fractional = !calendarInt && !idLike && wantsFractionalDisplay(options.question)

  if (calendarInt || idLike) {
    const rounded = Math.round(n)
    return getNumberFormat({
      maximumFractionDigits: 0,
      minimumFractionDigits: 0,
      useGrouping: false,
    }).format(rounded)
  }

  if (!fractional) {
    const rounded = Math.round(n)
    return getNumberFormat({
      maximumFractionDigits: 0,
      minimumFractionDigits: 0,
      useGrouping: true,
    }).format(rounded)
  }

  const maxFrac = maxFractionDigitsFromQuestion(options.question ?? '') ?? 4
  return getNumberFormat({
    maximumFractionDigits: maxFrac,
    minimumFractionDigits: 0,
    useGrouping: true,
  }).format(n)
}
