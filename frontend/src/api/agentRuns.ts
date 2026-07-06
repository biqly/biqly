import type { RunStep } from '../types/ai'
import { apiFetch } from './apiClient'

// AgentRun is a persisted AI query run (Agentic Runtime A1). It mirrors the
// backend agent_runs row; the frontend uses it to re-hydrate a thread's
// reasoning timeline after reload.
export interface AgentRun {
  id: string
  conversation_id?: string
  datasource_id: string
  model_id?: string
  user_id?: string
  question: string
  mode: string
  status: 'running' | 'waiting_clarification' | 'completed' | 'failed'
  confidence: number
  answer?: string
  created_at: string
  updated_at: string
}

export interface AgentRunDetail {
  run: AgentRun
  steps: RunStep[]
}

// getAgentRun fetches a persisted run and its ordered step trace.
export async function getAgentRun(id: string): Promise<AgentRunDetail> {
  return apiFetch<AgentRunDetail>('GET', `/api/ai/runs/${encodeURIComponent(id)}`)
}
