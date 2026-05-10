/**
 * Formats query result cells for display: by default integers with grouping (thousand separators),
 * rounded. Fractional digits only when the user explicitly asks for them (natural-language hint).
 */

export interface FormatResultCellOptions {
  /** Natural-language question; used to detect requests for decimal / fractional display. */
  question?: string
}

function parseNumeric(value: unknown): number | null {
  if (value === null || value === undefined) return null
  if (typeof value === 'boolean') return null
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'bigint') return Number(value)
  if (typeof value === 'string') {
    const t = value.trim()
    if (t === '') return null
    const n = Number(t)
    if (!Number.isNaN(n) && Number.isFinite(n)) return n
  }
  return null
}

/** Surrogate keys and numeric IDs: no thousand separators (e.g. 11091 not 11,091). */
function isIdentifierLikeColumn(name: string): boolean {
  const u = name.toUpperCase()
  if (u === 'ID' || u === 'ROW' || u === 'YEAR' || u === 'MONTH' || u === 'QUARTER' || u === 'WEEK') {
    return false
  }
  if (u.endsWith('_ID') || u.endsWith('_KEY')) return true
  if (u.includes('ENTITYID')) return true
  // Camel/snake keys: customerid, orderid, territoryid (length avoids GRID, VALID, …)
  if (u.length >= 7 && u.endsWith('ID') && /[A-Z]ID$/.test(u)) return true
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
  const m = lower.match(/(\d+)\s*(?:ondalık|decimal|hane|place|places|digits?)\b/)
  if (m?.[1]) return Math.min(10, Math.max(1, parseInt(m[1], 10)))
  return null
}

function wantsFractionalDisplay(question: string | undefined): boolean {
  if (!question || !question.trim()) return false
  const q = question.toLowerCase()
  const patterns: RegExp[] = [
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
  return patterns.some((p) => p.test(q))
}

/**
 * Renders a single result cell for HTML tables. Non-numeric values pass through as strings.
 */
export function formatResultCell(
  value: unknown,
  columnName: string,
  options: FormatResultCellOptions = {}
): string {
  if (value === null || value === undefined) return ''

  const n = parseNumeric(value)
  if (n === null) return String(value)

  const calendarInt = isCalendarIntColumn(columnName)
  const idLike = isIdentifierLikeColumn(columnName)
  const fractional =
    !calendarInt && !idLike && wantsFractionalDisplay(options.question)

  if (calendarInt || idLike) {
    const rounded = Math.round(n)
    return new Intl.NumberFormat(undefined, {
      maximumFractionDigits: 0,
      minimumFractionDigits: 0,
      useGrouping: false,
    }).format(rounded)
  }

  if (!fractional) {
    const rounded = Math.round(n)
    return new Intl.NumberFormat(undefined, {
      maximumFractionDigits: 0,
      minimumFractionDigits: 0,
      useGrouping: true,
    }).format(rounded)
  }

  const maxFrac = maxFractionDigitsFromQuestion(options.question || '') ?? 4
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: maxFrac,
    minimumFractionDigits: 0,
    useGrouping: true,
  }).format(n)
}
