export interface GlossaryAIContext {
  synonyms?: string[]
  unit?: string
  null_meaning?: string
  business_rules?: string[]
}

export interface BusinessGlossaryTerm {
  id: string
  datasource_id: string
  model_id?: string
  term: string
  definition?: string
  maps_to_type: 'dimension' | 'metric' | 'model'
  maps_to_name: string
  aliases?: string[]
  ai_context?: GlossaryAIContext
  is_active: boolean
  created_at: string
  updated_at: string
}
