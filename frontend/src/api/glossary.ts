import { apiFetch } from './apiClient'
import type { BusinessGlossaryTerm } from '../types/glossary'

const GLOSSARY_API_BASE = '/api/ai/glossary'

export const listGlossary = (datasourceId: string, modelId?: string) => {
  const params = new URLSearchParams()
  params.set('datasource_id', datasourceId)
  if (modelId) {
    params.set('model_id', modelId)
  }
  return apiFetch<BusinessGlossaryTerm[]>('GET', `${GLOSSARY_API_BASE}?${params.toString()}`)
}

export interface CreateGlossaryPayload {
  datasource_id: string
  model_id?: string
  term: string
  definition?: string
  maps_to_type: 'dimension' | 'metric' | 'model'
  maps_to_name: string
  aliases?: string[]
}

export const createGlossary = (payload: CreateGlossaryPayload) =>
  apiFetch<BusinessGlossaryTerm>('POST', GLOSSARY_API_BASE, payload)

export interface UpdateGlossaryPayload {
  term: string
  definition?: string
  maps_to_type: 'dimension' | 'metric' | 'model'
  maps_to_name: string
  aliases?: string[]
  is_active?: boolean
}

export const updateGlossary = (id: string, payload: UpdateGlossaryPayload) =>
  apiFetch<{ status: string }>('PUT', `${GLOSSARY_API_BASE}/${id}`, payload)

export const deleteGlossary = (id: string) =>
  apiFetch<void>('DELETE', `${GLOSSARY_API_BASE}/${id}`)
