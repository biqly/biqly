import type { PivotHint, QueryColumn } from '../types/ai'

export interface PivotTableData {
  columns: QueryColumn[]
  rows: unknown[][]
}

function pivotCellKey(value: unknown): string {
  if (value === null || value === undefined) {
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

export function buildPivotTable(
  columns: QueryColumn[],
  rows: unknown[][],
  hint: PivotHint,
): PivotTableData | null {
  const rowIdx = columns.findIndex((c) => c.name === hint.row_field)
  const colIdx = columns.findIndex((c) => c.name === hint.column_field)
  const valueField = hint.value_fields[0]
  if (!valueField) {
    return null
  }
  const valIdx = columns.findIndex((c) => c.name === valueField)
  if (rowIdx < 0 || colIdx < 0 || valIdx < 0) {
    return null
  }

  const grid = new Map<string, Map<string, number>>()
  const colKeys = new Set<string>()

  for (const row of rows) {
    const rk = pivotCellKey(row[rowIdx])
    const ck = pivotCellKey(row[colIdx])
    const raw = row[valIdx]
    const n = typeof raw === 'number' ? raw : Number(raw)
    if (Number.isNaN(n)) {
      continue
    }
    colKeys.add(ck)
    if (!grid.has(rk)) {
      grid.set(rk, new Map())
    }
    const bucket = grid.get(rk)!
    bucket.set(ck, (bucket.get(ck) ?? 0) + n)
  }

  const sortedColKeys = [...colKeys].sort((a, b) => a.localeCompare(b))
  const sortedRowKeys = [...grid.keys()].sort((a, b) => a.localeCompare(b))

  const pivotColumns: QueryColumn[] = [
    { name: hint.row_field, type: 'text' },
    ...sortedColKeys.map((k) => ({ name: k, type: 'number' as const })),
  ]

  const pivotRows = sortedRowKeys.map((rk) => {
    const byCol = grid.get(rk)!
    return [rk, ...sortedColKeys.map((ck) => byCol.get(ck) ?? null)]
  })

  return { columns: pivotColumns, rows: pivotRows }
}
