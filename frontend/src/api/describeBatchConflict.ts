import type { AIJob } from '../types/ai'
import { getLocale } from '../i18n'

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
  const res = await fetch(`/api/ai/jobs/describe-batch/conflict?${params}`, {
    headers: { 'X-Locale': getLocale() },
  })
  if (!res.ok) return null
  return (await res.json()) as DescribeBatchConflictResult
}

export type DescribeBatchConflictBody = {
  error: string
  existing_job_id?: string
  existing_job?: AIJob
  scope_schemas?: string[]
}
