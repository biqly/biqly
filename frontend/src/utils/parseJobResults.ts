import type { DescribeBatchResult } from '../types/metadata'

function isRecord(v: unknown): v is Record<string, unknown> {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

export function parseDescribeBatchResult(u: unknown): DescribeBatchResult {
  if (!isRecord(u) || !Array.isArray(u.entries)) {
    throw new Error('Invalid describe batch result')
  }
  return u as unknown as DescribeBatchResult
}
