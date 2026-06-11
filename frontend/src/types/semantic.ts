import type { Table } from './metadata'

export interface TableRow extends Omit<Table, 'columns' | 'table_type' | 'description' | 'label'> {
  id: string
  table_type: string
  description: string | null
  label?: string | null
  /** Row display label template, e.g. `author_name + " " + screen_name`. */
  display_expression?: string | null
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
  description?: string | null
  base_schema: string
  base_table: string
  status: string
  excluded_schemas?: string[]
}

export interface EnumMapping {
  id?: string
  dimension_id?: string
  raw_value: string
  label: string
  description?: string | null
  sort_order?: number
}

export type SemanticExprNode =
  | SemanticLiteralExpr
  | SemanticColumnRefExpr
  | SemanticMetricRefExpr
  | SemanticDimensionRefExpr
  | SemanticBinaryExpr
  | SemanticUnaryExpr
  | SemanticFunctionCallExpr
  | SemanticCaseExpr

export interface SemanticLiteralExpr {
  type: 'literal'
  value: string | number | boolean | null
}

export interface SemanticColumnRefExpr {
  type: 'column_ref'
  table?: string
  column: string
}

export interface SemanticMetricRefExpr {
  type: 'metric_ref'
  name: string
}

export interface SemanticDimensionRefExpr {
  type: 'dimension_ref'
  name: string
}

export interface SemanticBinaryExpr {
  type: 'binary'
  op: string
  left: SemanticExprNode
  right: SemanticExprNode
}

export interface SemanticUnaryExpr {
  type: 'unary'
  op: string
  expr: SemanticExprNode
}

export interface SemanticFunctionCallExpr {
  type: 'function_call'
  name: string
  args?: SemanticExprNode[]
}

export interface SemanticCaseExpr {
  type: 'case'
  conditions?: {
    when: SemanticExprNode
    then: SemanticExprNode
  }[]
  else?: SemanticExprNode
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
  calculated_expression?: string
  calculated_expr?: SemanticExprNode
  enum_values?: EnumMapping[]
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
  expr?: SemanticExprNode
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
  join_type: 'LEFT' | 'INNER' | 'RIGHT'
  relationship: 'many_to_one' | 'one_to_many' | 'one_to_one' | 'many_to_many'
  is_active?: boolean
}

export interface SemanticModelDetail extends SemanticModelSummary {
  dimensions?: SemanticDimension[]
  metrics?: SemanticMetric[]
  joins?: SemanticJoin[]
}

export interface SemanticModelFieldRow {
  kind: 'dimension' | 'metric'
  id: string
  name: string
  label?: string | null
  ref: string
  subtype: string
}

export interface SemanticModelFieldsPage {
  model_name: string
  items: SemanticModelFieldRow[]
  total: number
  page: number
  page_size: number
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
  if (m.label?.trim()) {
    return `${m.name} (${m.label})`
  }
  return m.name
}

export function modelListHint(m: SemanticModelSummary): string {
  return [m.status, `${m.base_schema}.${m.base_table}`].join(' · ')
}
