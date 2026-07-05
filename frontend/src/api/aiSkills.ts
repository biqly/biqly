import type { Skill, SkillParameter } from '../components/skills/types'
import { apiFetch } from './apiClient'

// SavedQueryOption is the lightweight shape the composer "/"-picker needs: a
// selectable saved query identified by id, shown by name + originating question.
export interface SavedQueryOption {
  id: string
  name: string
  question: string
}

// listSavedQueries fetches the datasource's runnable saved queries for the
// composer "/"-picker. Returns [] when the datasource is unset.
export async function listSavedQueries(datasourceId: string): Promise<SavedQueryOption[]> {
  if (!datasourceId) {
    return []
  }
  const data = await apiFetch<{ skills: Skill[] }>(
    'GET',
    `/api/ai/skills?datasource_id=${encodeURIComponent(datasourceId)}`,
  )
  return data.skills.map((s) => ({ id: s.id, name: s.name, question: s.question }))
}

// AI generation can take a while; allow more than the default 30s.
const AI_DRAFT_TIMEOUT_MS = 120_000

export interface DraftSavedQueryRequest {
  datasource_id: string
  model_id?: string
  question: string
}

export interface DraftSavedQueryResponse {
  name?: string
  description?: string
  question: string
  logical_query?: Record<string, unknown> | null
  parameters?: SkillParameter[]
  needs_clarification?: boolean
  message?: string
  error?: string
}

// draftSavedQuery asks the backend to draft a Saved Query (LogicalQuery +
// suggested name/description) from a natural-language description, reusing the
// text-to-SQL generation. Nothing is persisted — the caller prefills the form
// and the user reviews before saving. On a 4xx/5xx the promise rejects with an
// ApiError whose message is the server-provided reason.
export async function draftSavedQuery(
  req: DraftSavedQueryRequest,
): Promise<DraftSavedQueryResponse> {
  return apiFetch<DraftSavedQueryResponse>('POST', '/api/ai/skills/draft', req, {
    timeout: AI_DRAFT_TIMEOUT_MS,
  })
}
