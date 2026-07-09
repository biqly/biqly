import type { LogicalQuery } from '../types/ai'
import { apiFetch } from './apiClient'

export interface CompileQueryResponse {
  sql: string
  args: unknown[]
}

export interface DryRunQueryResponse extends CompileQueryResponse {
  fingerprint: string
}

export function compileQuery(logicalQuery: LogicalQuery): Promise<CompileQueryResponse> {
  return apiFetch<CompileQueryResponse>('POST', '/api/query/compile', logicalQuery)
}

export function dryRunQuery(logicalQuery: LogicalQuery): Promise<DryRunQueryResponse> {
  return apiFetch<DryRunQueryResponse>('POST', '/api/query/dry-run', logicalQuery)
}
