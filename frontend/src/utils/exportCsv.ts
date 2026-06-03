function escapeCsvValue(value: unknown): string {
  if (value === null || value === undefined) {
    return ''
  }
  const raw = typeof value === 'object' ? JSON.stringify(value) : String(value)
  if (/[",\n\r]/.test(raw)) {
    return `"${raw.replace(/"/g, '""')}"`
  }
  return raw
}

/** Build CSV text (RFC 4180) from column headers and row values. */
export function buildCsv(columns: { name: string }[], rows: unknown[][]): string {
  const header = columns.map((c) => escapeCsvValue(c.name)).join(',')
  const body = rows.map((row) => row.map(escapeCsvValue).join(',')).join('\r\n')
  return body ? `${header}\r\n${body}` : header
}

function sanitizeFilename(name: string): string {
  const trimmed = name
    .trim()
    .slice(0, 60)
    .replace(/[^\w\d-]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return trimmed || 'export'
}

/**
 * Download tabular data as a CSV file. Prepends a UTF-8 BOM so Excel renders
 * non-ASCII characters (e.g. Turkish) correctly.
 */
export function downloadCsv(
  columns: { name: string }[],
  rows: unknown[][],
  baseName = 'biqly-export',
): void {
  const csv = buildCsv(columns, rows)
  const blob = new Blob(['﻿', csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${sanitizeFilename(baseName)}.csv`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
