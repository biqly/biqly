// ─── Semantic Layer Types ──────────────────────────────────────────

export interface SemanticModel {
  id: string
  name: string
  datasource_id: string
  description?: string
  dimensions: Dimension[]
  metrics: Metric[]
  joins: Join[]
  created_at?: string
  updated_at?: string
}

export interface Dimension {
  name: string
  table: string
  column: string
  type: 'string' | 'number' | 'date' | 'boolean'
  label?: string
  description?: string
  format?: string
  hidden?: boolean
}

export interface Metric {
  name: string
  table: string
  column: string
  type: 'count' | 'sum' | 'avg' | 'min' | 'max' | 'count_distinct'
  label?: string
  description?: string
  format?: string
  hidden?: boolean
}

export interface Join {
  name: string
  left_table: string
  left_column: string
  right_table: string
  right_column: string
  join_type: 'inner' | 'left' | 'right' | 'full'
  description?: string
}
