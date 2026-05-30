import type { AIJob } from '../types/ai'
import { apiFetch } from './apiClient'

export type DescribeBatchConflictResult = {
  conflict: boolean
  existing_job_id?: string
  existing_job?: AIJob
  scope_schemas?: string[]
}

export async function fetchDescribeBatchConflict(
  datasourceId: string,
  schemas: string[],
): Promise<DescribeBatchConflictResult | null> {
  if (!datasourceId || schemas.length === 0) {
    return { conflict: false }
  }
  const params = new URLSearchParams({ datasource_id: datasourceId })
  for (const s of schemas) {
    params.append('schemas', s)
  }
  try {
    return await apiFetch<DescribeBatchConflictResult>('GET', `/api/ai/jobs/describe-batch/conflict?${params}`)
  } catch {
    return null
  }
}

export type DescribeBatchConflictBody = {
  error: string
  existing_job_id?: string
  existing_job?: AIJob
  scope_schemas?: string[]
}
