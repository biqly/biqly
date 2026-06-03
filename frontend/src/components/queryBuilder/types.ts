export interface FilterRow {
  id: string
  field: string
  operator: string
  value: string
  caseSensitive?: boolean
}

export interface SelectItem {
  id: string
  type: 'dimension' | 'metric'
  name: string
}

export interface HavingRow {
  field: string
  operator: string
  value: string
}

export interface WindowFuncRow {
  func: string
  field: string
  partition_by: string
  order_by: string
}

export interface CTERow {
  name: string
  query: string
}

export type QueryBuilderMode = 'simple' | 'advanced'

export interface QueryBuilderFormState {
  datasourceId: string
  modelId: string
  mode: QueryBuilderMode
  selectItems: SelectItem[]
  filters: FilterRow[]
  groupBy: string[]
  having: HavingRow[]
  orderBy: string
  orderDir: string
  limit: number
  offset: number
  windowFunctions: WindowFuncRow[]
  ctes: CTERow[]
}

export const WINDOW_FUNC_OPTIONS = [
  'ROW_NUMBER',
  'RANK',
  'DENSE_RANK',
  'LAG',
  'LEAD',
  'SUM',
  'AVG',
  'COUNT',
] as const

export function newRowId(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `row-${Date.now()}-${Math.random().toString(36).slice(2)}`
}
