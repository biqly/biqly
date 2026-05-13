import type {
  CTE,
  CompiledQuery,
  FilterClause,
  GroupByField,
  LogicalQuery,
  OrderByField,
  QueryColumn,
  QueryResultPayload,
  SelectField,
  WindowFunction,
  WindowSpec,
} from './ai'

export type LogicalQueryPayload = LogicalQuery
export type {
  CTE,
  CompiledQuery,
  FilterClause,
  GroupByField,
  OrderByField,
  QueryColumn,
  SelectField,
  WindowFunction,
  WindowSpec,
}
export type QueryResult = QueryResultPayload

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
