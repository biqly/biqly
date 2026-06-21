import type { DescribeJobRequest } from '../hooks/useAIJobsUtils'
import type { DescribeResult } from '../types/metadata'
import { apiFetch } from './apiClient'

export type { DescribeBatchResult, DescribeResult } from '../types/metadata'

const AI_METADATA_DESCRIBE_TIMEOUT_MS = 600_000

export async function runMetadataDescribeDirect(
  request: DescribeJobRequest,
): Promise<DescribeResult> {
  return apiFetch<DescribeResult>('POST', '/api/ai/metadata/describe', request, {
    timeout: AI_METADATA_DESCRIBE_TIMEOUT_MS,
  })
}
