import type { AIQueryResponse } from '../types/ai'

const MAX_SUMMARY_LENGTH = 300
const MAX_ROWS_SMALL_RESULT = 5
const MAX_ROWS_LARGE_RESULT = 3

export function buildResultSummary(response: AIQueryResponse | null | undefined): string {
  if (!response) {
    return 'no response'
  }
  if (response.needs_clarification) {
    return 'clarification needed'
  }
  if (response.result) {
    const rows = response.result.rows
    if (rows.length === 0) {
      return 'no results'
    }
    const columns = response.result.columns.map((column) => column.name)
    const maxRows =
      rows.length <= MAX_ROWS_SMALL_RESULT ? MAX_ROWS_SMALL_RESULT : MAX_ROWS_LARGE_RESULT
    const formattedRows = rows.slice(0, maxRows).map((row) => formatRow(columns, row))
    const remaining = rows.length - formattedRows.length
    const numericExtremes = formatNumericExtremes(columns, rows)
    const summary =
      remaining > 0
        ? `${formattedRows.join('; ')}; and ${remaining} more`
        : formattedRows.join('; ')
    const summaryWithExtremes = numericExtremes ? `${summary}; ${numericExtremes}` : summary
    return truncateSummary(summaryWithExtremes)
  }
  if (response.sql) {
    return truncateSummary(`SQL generated: ${response.sql.slice(0, 80)}`)
  }
  if (response.warnings?.[0]) {
    return truncateSummary(`warning: ${response.warnings[0]}`)
  }
  return 'no results'
}

function formatRow(columns: string[], row: unknown[]): string {
  return row
    .map((value, index) => `${columns[index] ?? `col_${index + 1}`}=${formatValue(value)}`)
    .join(', ')
}

function formatValue(value: unknown): string {
  if (value == null) {
    return ''
  }
  if (typeof value === 'string') {
    return value
  }
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
    return String(value)
  }
  return JSON.stringify(value)
}

function formatNumericExtremes(columns: string[], rows: unknown[][]): string {
  for (const [columnIndex, columnName] of columns.entries()) {
    const numericValues = rows
      .map((row) => row[columnIndex])
      .filter((value): value is number => typeof value === 'number' && Number.isFinite(value))
    if (numericValues.length === 0) {
      continue
    }
    return `min ${columnName}=${Math.min(...numericValues)}, max ${columnName}=${Math.max(...numericValues)}`
  }
  return ''
}

function truncateSummary(summary: string): string {
  if (summary.length <= MAX_SUMMARY_LENGTH) {
    return summary
  }
  return `${summary.slice(0, MAX_SUMMARY_LENGTH - 3)}...`
}
