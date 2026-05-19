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
  type: 'string' | 'text' | 'number' | 'date' | 'boolean' | 'geo'
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
  id?: string
  name: string
  from_schema?: string
  from_table: string
  from_column: string
  to_schema?: string
  to_table: string
  to_column: string
  join_type?: string
  relationship?: string
  /** @deprecated use from_table / to_table */
  left_table?: string
  /** @deprecated use from_column / to_column */
  left_column?: string
  /** @deprecated */
  right_table?: string
  /** @deprecated */
  right_column?: string
  description?: string
}
