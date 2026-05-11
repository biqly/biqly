// ─── Query Execution Types ─────────────────────────────────────────

export interface LogicalQueryPayload {
  datasource_id: string
  model_id: string
  select?: SelectField[]
  filters?: FilterClause[]
  group_by?: GroupByField[]
  order_by?: OrderByField[]
  having?: FilterClause[]
  limit?: number
  offset?: number
  ctes?: CTE[]
}

export interface SelectField {
  type: 'dimension' | 'metric' | 'window'
  name: string
  alias?: string
  window?: WindowSpec
}

export interface WindowSpec {
  aggregation: string
  expression?: string
  metric?: string
  partition_by?: string[]
  order_by?: OrderByField[]
  frame?: string
}

export interface FilterClause {
  field: string
  operator: string
  value: unknown
}

export interface GroupByField {
  field: string
}

export interface OrderByField {
  field: string
  direction: 'asc' | 'desc'
}

export interface CTE {
  name: string
  query: Record<string, unknown>
}

export interface WindowFunction {
  function: string
  field: string
  partition_by?: string[]
  order_by?: OrderByField[]
  alias?: string
}

export interface QueryResult {
  columns: QueryColumn[]
  rows: unknown[][]
  stats: {
    row_count: number
    duration_ms: number
  }
}

export interface QueryColumn {
  name: string
  type?: string
}

export interface CompiledQuery {
  sql: string
  dialect: string
  parameters?: Record<string, unknown>
  execution_plan?: string
}

export interface QueryHistoryEntry {
  id: string
  question: string
  logical_query: LogicalQueryPayload
  compiled_sql?: string
  result?: QueryResult
  created_at: string
  datasource_id: string
  duration_ms?: number
  row_count?: number
}
