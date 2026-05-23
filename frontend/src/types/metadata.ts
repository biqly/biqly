// ─── Metadata Types ────────────────────────────────────────────────

export interface Datasource {
  id: string
  name: string
  type: string
  is_active?: boolean
  last_sync_at?: string | null
  created_at?: string
  updated_at?: string
  host?: string | null
  port?: number | null
  username?: string | null
  database_name?: string | null
  ssl_mode?: string | null
  dsn_mode?: 'raw' | 'structured'
}

export interface Table {
  schema_name: string
  table_name: string
  table_type?: string
  description?: string | null
  label?: string
  columns?: Column[]
}

export interface Column {
  column_name: string
  data_type: string
  is_nullable?: boolean
  description?: string | null
  ai_description?: string | null
  sample_values?: unknown[]
}

export interface Relation {
  from_table: string
  from_column: string
  to_table: string
  to_column: string
  relation_type: 'one-to-many' | 'many-to-one' | 'one-to-one' | 'many-to-many'
}

export interface DescribeResult {
  schema_name: string
  table_name: string
  description: string
  columns: {
    column_name: string
    description: string
  }[]
  sample_size: number
  translation_applied?: boolean
  translation_model?: string
  translation_error?: string
}

export interface BulkDescribeEntry {
  schema_name: string
  table_name: string
  status: 'pending' | 'running' | 'ok' | 'error' | 'skipped'
  error?: string
}
