import type { SkillParameter } from '../components/skills/types'
import { apiFetch } from './apiClient'

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
