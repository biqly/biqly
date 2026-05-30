import type { DescribeResult } from '../types/metadata'
import { apiFetch } from './apiClient'

export type { DescribeBatchResult, DescribeResult } from '../types/metadata'

const AI_METADATA_DESCRIBE_TIMEOUT_MS = 600_000

export async function runMetadataDescribeDirect(request: Record<string, unknown>): Promise<DescribeResult> {
  const data = await apiFetch<DescribeResult>('POST', '/api/ai/metadata/describe', request, {
    timeout: AI_METADATA_DESCRIBE_TIMEOUT_MS,
  })
  if (!data) throw new Error('Empty describe response')
  return data
}
