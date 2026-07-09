const SQL_KEYWORD_PATTERN =
  /\b(SELECT|FROM|WHERE|GROUP\s+BY|ORDER\s+BY|HAVING|LIMIT|OFFSET|JOIN|LEFT|RIGHT|INNER|OUTER|ON|AS|AND|OR|NOT|IN|IS|NULL|CASE|WHEN|THEN|ELSE|END|ASC|DESC|DISTINCT|WITH|UNION|ALL)\b/gi

export type SQLTokenKind = 'keyword' | 'text'

export interface SQLToken {
  value: string
  kind: SQLTokenKind
}

/** Splits SQL into display-safe keyword and text spans for the preview. */
export function tokenizeSQL(sql: string): SQLToken[] {
  const tokens: SQLToken[] = []
  let start = 0
  for (const match of sql.matchAll(SQL_KEYWORD_PATTERN)) {
    const index = match.index
    if (index > start) {
      tokens.push({ value: sql.slice(start, index), kind: 'text' })
    }
    tokens.push({ value: match[0], kind: 'keyword' })
    start = index + match[0].length
  }
  if (start < sql.length) {
    tokens.push({ value: sql.slice(start), kind: 'text' })
  }
  return tokens
}
