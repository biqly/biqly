export interface TableRow {
  id: string
  schema_name: string
  table_name: string
  table_type: string
  description: string | null
  label?: string | null
}

export interface ColumnRow {
  id: string
  schema_name: string
  table_name: string
  column_name: string
  data_type: string
  nullable: boolean
  description: string | null
  is_primary_key: boolean
  is_foreign_key: boolean
  referenced_schema?: string | null
  referenced_table: string | null
  referenced_column: string | null
}

export interface SemanticModelSummary {
  id: string
  datasource_id: string
  name: string
  label?: string | null
  base_schema: string
  base_table: string
  status: string
  excluded_schemas?: string[]
}

export interface SemanticDimension {
  id: string
  name: string
  label?: string | null
  column_ref: string
  type: string
  synonyms?: string[]
  description?: string | null
  is_active?: boolean
}

export interface SemanticMetric {
  id: string
  name: string
  label?: string | null
  expression: string
  aggregation: string
  format?: string | null
  synonyms?: string[]
  description?: string | null
  is_active?: boolean
}

export interface SemanticJoin {
  id: string
  name: string
  from_schema?: string
  from_table: string
  from_column: string
  to_schema?: string
  to_table: string
  to_column: string
  join_type: string
  relationship: string
  is_active?: boolean
}

export interface SemanticModelDetail extends SemanticModelSummary {
  dimensions?: SemanticDimension[]
  metrics?: SemanticMetric[]
  joins?: SemanticJoin[]
}

export interface GenerateSemanticModelResponse {
  model: SemanticModelDetail
  warnings?: string[]
  validation?: {
    valid: boolean
    errors?: string[]
    warnings?: string[]
    estimated_prompt_size?: number
  }
  published: boolean
}

export function modelListLabel(m: SemanticModelSummary): string {
  if (m.label && m.label.trim()) return `${m.name} (${m.label})`
  return m.name
}

export function modelListHint(m: SemanticModelSummary): string {
  return [m.status, `${m.base_schema}.${m.base_table}`].join(' · ')
}
