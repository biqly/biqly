import { fetchJSON, type FetchJSONResult } from '../api/apiClient'
import type { AIJob, AIJobKind } from '../types/ai'

export { fetchJSON }
export type { FetchJSONResult }

export type TrackedAIJob = AIJob & {
  questionPreview?: string
}

export type { JobCallbacks } from './jobWaiter'

export interface BulkDescribeSummary {
  ok: number
  error: number
  skipped: number
}

export interface BulkDescribeTarget {
  schema_name: string
  table_name: string
  description: string | null
}

export function jobIsActive(job: AIJob): boolean {
  return job.status === 'pending' || job.status === 'queued' || job.status === 'running'
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function jobQuestionPreview(kind: AIJobKind, req: unknown): string {
  if (!req || typeof req !== 'object') {
    return kind
  }
  const record = req as Record<string, unknown>
  if (kind === 'describe') {
    const schema = asString(record.schema)
    const table = asString(record.table)
    const target = [schema, table].filter(Boolean).join('.')
    return target || kind
  }
  if (kind === 'describe_batch') {
    const tables = record.tables
    if (Array.isArray(tables)) {
      return `${tables.length} tables`
    }
  }
  if (kind === 'embed_metadata') {
    return 'Embedding refresh'
  }
  const q = asString(record.question)
  if (q.length <= 80) {
    return q
  }
  return `${q.slice(0, 77)}…`
}

export function trackedJobFromAIJob(job: AIJob): TrackedAIJob {
  const questionPreview =
    job.request_json && typeof job.request_json === 'object'
      ? jobQuestionPreview(job.kind, job.request_json)
      : undefined
  return { ...job, questionPreview }
}
