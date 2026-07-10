import type { QueryResultPayload } from '../../types/ai'

export type ResultChartType = 'bar' | 'line' | 'pie' | 'table'

/** Column names that read as a time axis even without an explicit format. */
const TIME_NAME_RE = /date|time|hour|day|week|month|quarter|year|_ts$|_at$/i

function isTimeColumn(column: QueryResultPayload['columns'][number] | undefined): boolean {
  if (!column) {
    return false
  }
  if (column.format === 'date' || column.format === 'datetime') {
    return true
  }
  if (column.format === 'month_of_year' || column.format === 'quarter') {
    return true
  }
  return TIME_NAME_RE.test(column.name)
}

/** Categories beyond this stop being readable as a pie. */
const MAX_PIE_CATEGORIES = 12

/**
 * relevantChartTypes narrows the chart-type tabs to what the result shape can
 * meaningfully display: time series → line/bar, small categorical → bar/pie,
 * everything else → bar/table. The table view is always available.
 */
export function relevantChartTypes(payload: QueryResultPayload): ResultChartType[] {
  const { columns, rows } = payload
  const hasValueColumn =
    columns.some((column) => column.semantic_type === 'metric') || columns.length > 1
  if (rows.length <= 1 || columns.length < 2 || !hasValueColumn) {
    return ['table']
  }
  if (isTimeColumn(columns[0])) {
    return ['line', 'bar', 'table']
  }
  if (rows.length <= MAX_PIE_CATEGORIES) {
    return ['bar', 'pie', 'table']
  }
  return ['bar', 'table']
}

/** Keeps the currently selected type visible even when the shape heuristic
 * would drop it (e.g. a server visualization hint chose it). */
export function chartTypeOptions(
  payload: QueryResultPayload,
  selected: ResultChartType,
): ResultChartType[] {
  const types = relevantChartTypes(payload)
  if (types.includes(selected)) {
    return types
  }
  const order: ResultChartType[] = ['bar', 'line', 'pie', 'table']
  return order.filter((type) => type === selected || types.includes(type))
}
