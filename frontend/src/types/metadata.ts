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
  relation_type: 'one_to_many' | 'many_to_one' | 'one_to_one' | 'many_to_many'
}

// Matches the GET /api/datasources/{id}/relations response: an introspected
// FK relationship enriched with a semantic-join description when one exists.
export interface RelationDetail {
  id: string
  constraint_name: string
  from_schema: string
  from_table: string
  from_column: string
  to_schema: string
  to_table: string
  to_column: string
  relationship_type: string
  description: string
}

export interface DescribeResult {
  schema: string
  table: string
  description: string
  columns: { name: string; description: string }[]
  applied: boolean
  sample_rows: number
  model?: string
  translation_applied?: boolean
  translation_model?: string
  translation_error?: string
}

export interface DescribeBatchEntryResult {
  schema: string
  table: string
  status: 'ok' | 'error' | 'skipped'
  message?: string
  result?: DescribeResult
}

export interface DescribeBatchResult {
  entries: DescribeBatchEntryResult[]
  ok: number
  error: number
  skipped: number
}

export interface BulkDescribeEntry {
  schema_name: string
  table_name: string
  status: 'pending' | 'running' | 'ok' | 'error' | 'skipped'
  error?: string
}
