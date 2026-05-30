export interface SavedQuestion {
  id: string
  name: string
  description: string
  datasource_id: string
  model_id?: string
  question: string
  logical_query: Record<string, unknown>
  tags: string[]
  dialect: string
  locale?: string
  is_few_shot: boolean
  created_at?: string
  updated_at?: string
}

export interface SavedQuestionSemanticModel {
  id: string
  name: string
  label?: string | null
  status: string
}

export interface SavedQuestionFormState {
  datasourceId: string
  modelId: string
  name: string
  description: string
  question: string
  logicalQuery: string
  tags: string
  dialect: string
  locale: string
  isFewShot: boolean
}
