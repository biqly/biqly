import { evalDisplayExpression } from './displayExpression'

const PREFERRED_TITLE_COLUMN_PATTERNS = [
  /^(title|name|label|subject|headline|heading|display_name)$/,
  /^(text|body|content|message|description|caption|summary)$/,
  /(_|^)(title|name|label|subject)$/,
  /(_|^)(text|body|content|message|description)$/,
]

const ID_COLUMN_PATTERNS = [/^(id|uuid|pk)$/, /(_|^)id$/]

function singularize(name: string): string {
  const n = name.toLowerCase()
  if (n.endsWith('ies')) {
    return `${n.slice(0, -3)}y`
  }
  if (n.endsWith('s') && !n.endsWith('ss')) {
    return n.slice(0, -1)
  }
  return n
}

function truncateTitle(s: string): string {
  return s.length > 80 ? `${s.slice(0, 77).trimEnd()}…` : s
}

/** Heuristic row title: preferred text columns, then table-specific id, then any id. */
export function heuristicRowTitle(
  row: unknown[],
  columns: string[],
  fallback: string,
  tableKeyValue?: string | null,
): string {
  const stringValues: { name: string; value: string }[] = []
  for (let i = 0; i < columns.length; i++) {
    const v = row[i]
    if (v == null) {
      continue
    }
    const s = typeof v === 'string' ? v : typeof v === 'number' ? String(v) : ''
    const trimmed = s.trim()
    if (!trimmed) {
      continue
    }
    const colName = columns[i]
    if (!colName) {
      continue
    }
    stringValues.push({ name: colName.toLowerCase(), value: trimmed })
  }
  if (stringValues.length === 0) {
    return fallback
  }

  for (const pattern of PREFERRED_TITLE_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncateTitle(hit.value)
    }
  }

  if (tableKeyValue) {
    const lastSegment = tableKeyValue.split('.').pop() ?? tableKeyValue
    const singular = singularize(lastSegment)
    const pkHit = stringValues.find(
      (c) =>
        c.name === `${singular}_id` ||
        c.name === `${lastSegment.toLowerCase()}_id` ||
        c.name === 'id',
    )
    if (pkHit) {
      return truncateTitle(`${pkHit.name} ${pkHit.value}`)
    }
  }

  for (const pattern of ID_COLUMN_PATTERNS) {
    const hit = stringValues.find((c) => pattern.test(c.name))
    if (hit) {
      return truncateTitle(`${hit.name} ${hit.value}`)
    }
  }
  return fallback
}

/** Display-expression title with heuristic fallback. */
export function rowTitleFor(
  row: unknown[],
  columns: string[],
  displayExpression: string | undefined,
  fallback: string,
  tableKeyValue?: string | null,
): string {
  if (displayExpression) {
    const idx = new Map(columns.map((c, i) => [c, i]))
    const fromExpr = evalDisplayExpression(displayExpression, (col) => {
      const i = idx.get(col)
      return i != null ? row[i] : null
    })
    if (fromExpr) {
      return fromExpr.length > 80 ? `${fromExpr.slice(0, 77).trimEnd()}…` : fromExpr
    }
  }
  return heuristicRowTitle(row, columns, fallback, tableKeyValue)
}
