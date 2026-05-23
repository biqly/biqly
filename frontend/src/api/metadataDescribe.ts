import { getLocale } from '../i18n'
import type { DescribeBatchResult, DescribeResult } from '../types/metadata'
import { plainTextFromHTML } from '../utils/plainText'

export type { DescribeBatchResult, DescribeResult } from '../types/metadata'

const AI_METADATA_DESCRIBE_TIMEOUT_MS = 600_000

async function readDescribeResponse(res: Response): Promise<{ data: DescribeResult | null; error: string | null }> {
  const text = await res.text()
  if (!text) {
    return res.ok ? { data: null, error: null } : { data: null, error: `HTTP ${res.status}` }
  }

  const contentType = res.headers.get('content-type') ?? ''
  const looksJSON = contentType.includes('application/json') || /^[\s[{]/.test(text)
  if (!looksJSON) {
    const message = plainTextFromHTML(text)
    return { data: null, error: message ? `HTTP ${res.status}: ${message}` : `HTTP ${res.status}` }
  }

  try {
    const data = JSON.parse(text) as DescribeResult & { error?: string }
    if (!res.ok) return { data: null, error: data.error || `HTTP ${res.status}` }
    return { data, error: null }
  } catch {
    const message = plainTextFromHTML(text)
    return { data: null, error: message ? `HTTP ${res.status}: ${message}` : `HTTP ${res.status}: invalid JSON response` }
  }
}

export async function runMetadataDescribeDirect(request: Record<string, unknown>): Promise<DescribeResult> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), AI_METADATA_DESCRIBE_TIMEOUT_MS)
  try {
    const res = await fetch('/api/ai/metadata/describe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Locale': getLocale() },
      body: JSON.stringify(request),
      signal: controller.signal,
    })
    const { data, error } = await readDescribeResponse(res)
    if (error) throw new Error(error)
    if (!data) throw new Error('Empty describe response')
    return data
  } finally {
    window.clearTimeout(timeout)
  }
}
