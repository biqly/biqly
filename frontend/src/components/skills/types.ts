export interface SkillParameter {
  name: string
  label?: string
  type?: string
  required?: boolean
  default?: unknown
}

export interface Skill {
  id: string
  datasource_id: string
  model_id?: string
  name: string
  description: string
  question: string
  logical_query: Record<string, unknown> | null
  parameters: SkillParameter[]
  tags: string[]
  created_by: string
  version: number
  is_active: boolean
  last_verified_at?: string
  created_at: string
  updated_at: string
}

export interface SkillFormState {
  datasourceId: string
  modelId: string
  name: string
  description: string
  question: string
  logicalQuery: string
  parameters: string
  tags: string
  isActive: boolean
}

export interface SkillRunResult {
  skill_id: string
  name: string
  sql?: string
  result: {
    columns: { name: string; type?: string }[]
    rows: unknown[][]
    stats?: { row_count?: number; duration_ms?: number }
  } | null
}

export function paramDefaultText(value: unknown): string {
  if (value === undefined || value === null) {
    return ''
  }
  return typeof value === 'string' ? value : JSON.stringify(value)
}
