export type ComponentRole = 'primary' | 'secondary'

export type ConflictResolution = 'use_primary' | 'rename' | 'merge'

export interface ComponentModelRef {
  id?: string
  composite_id?: string
  model_id: string
  alias: string
  role: ComponentRole
  created_at?: string
}

export interface CrossModelJoin {
  id?: string
  composite_id?: string
  name?: string
  from_model: string
  from_dimension: string
  to_model: string
  to_dimension: string
  join_type: 'LEFT' | 'INNER' | 'RIGHT'
  relationship: 'many_to_one' | 'one_to_many' | 'one_to_one' | 'many_to_many'
  is_active?: boolean
}

export interface CanonicalDateRef {
  model_alias: string
  dimension_name: string
}

export interface DimensionConflictResolution {
  id?: string
  composite_id?: string
  dimension_name: string
  resolution: ConflictResolution
  source_alias?: string
  target_alias?: string
}

export interface CompositeModelSummary {
  id: string
  datasource_id: string
  name: string
  label?: string | null
  description?: string | null
  is_active: boolean
  status: string
  version: number
  published_at?: string | null
  published_by?: string | null
}

export interface CompositeModelDetail extends CompositeModelSummary {
  components?: ComponentModelRef[]
  cross_model_joins?: CrossModelJoin[]
  canonical_date?: CanonicalDateRef | null
  conflict_resolutions?: DimensionConflictResolution[]
}

export interface CompositeValidationIssue {
  field?: string
  message: string
  severity?: string
}

export interface CompositeValidationResult {
  valid: boolean
  errors?: CompositeValidationIssue[]
  warnings?: CompositeValidationIssue[]
}

export interface CompositePublishResult {
  composite?: CompositeModelDetail
  resolved?: unknown
  validation: CompositeValidationResult
  version?: number
}

export interface SuggestedCrossJoin {
  from_model: string
  from_dimension: string
  to_model: string
  to_dimension: string
  reason: string
}

export interface SuggestedJoinsResponse {
  suggestions: SuggestedCrossJoin[]
}
