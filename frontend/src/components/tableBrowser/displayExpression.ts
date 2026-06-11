/**
 * Display expressions label a single row: column tokens and quoted string
 * literals joined with '+', e.g. `author_name + " " + screen_name`.
 * Evaluated entirely client-side; never sent to SQL.
 */

/** Splits on top-level '+' while respecting single/double quoted literals. */
function splitTopLevel(expr: string): string[] {
  const parts: string[] = []
  let current = ''
  let quote: string | null = null
  for (const ch of expr) {
    if (quote) {
      current += ch
      if (ch === quote) {
        quote = null
      }
      continue
    }
    if (ch === '"' || ch === "'") {
      quote = ch
      current += ch
      continue
    }
    if (ch === '+') {
      parts.push(current.trim())
      current = ''
      continue
    }
    current += ch
  }
  parts.push(current.trim())
  return parts.filter((p) => p.length > 0)
}

/**
 * Evaluates a display expression against a row. Returns null when the
 * expression references no resolvable column values (so callers can fall back
 * to a heuristic title).
 */
export function evalDisplayExpression(
  expr: string,
  getValue: (column: string) => unknown,
): string | null {
  const parts = splitTopLevel(expr)
  if (parts.length === 0) {
    return null
  }
  let out = ''
  let anyColumnValue = false
  for (const part of parts) {
    const isLiteral =
      (part.startsWith('"') && part.endsWith('"') && part.length >= 2) ||
      (part.startsWith("'") && part.endsWith("'") && part.length >= 2)
    if (isLiteral) {
      out += part.slice(1, -1)
      continue
    }
    const v = getValue(part)
    if (v == null) {
      continue
    }
    const s = typeof v === 'string' ? v : typeof v === 'number' ? String(v) : ''
    if (s) {
      anyColumnValue = true
      out += s
    }
  }
  const trimmed = out.trim()
  return anyColumnValue && trimmed ? trimmed : null
}

/** Lists the column tokens referenced by a display expression. */
export function displayExpressionColumns(expr: string): string[] {
  return splitTopLevel(expr).filter((p) => !p.startsWith('"') && !p.startsWith("'"))
}
