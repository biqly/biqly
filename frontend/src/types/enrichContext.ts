export type EnrichGapKind =
  | 'column_missing_description'
  | 'dimension_missing_description'
  | 'metric_missing_description'
  | 'glossary_missing_definition'
  | 'enum_missing_label'
  | 'synonym_collision'

export interface EnrichGap {
  id: string
  kind: EnrichGapKind
  summary: string
  detail?: string
  entity?: Record<string, string>
  applyable: boolean
}

export interface EnrichSuggestion {
  gap_id: string
  text: string
}

export interface EnrichAnalyzeResult {
  datasource_id: string
  model_id: string
  model_name: string
  gaps: EnrichGap[]
  suggestions?: EnrichSuggestion[]
  sample_rows?: number
}

export interface EnrichApplyItem {
  gap_id: string
  value: string
}

export interface EnrichApplyResult {
  applied: number
  skipped: number
  errors?: string[]
}
