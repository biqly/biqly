import { getLocale } from '../i18n'

export interface DescribeResult {
  schema: string
  table: string
  description: string
  columns: { name: string; description: string }[]
  applied: boolean
  sample_rows: number
  model?: string
  translation_applied?: boolean
  translation_model?: string
  translation_error?: string
}

export interface DescribeBatchEntryResult {
  schema: string
  table: string
  status: 'ok' | 'error' | 'skipped'
  message?: string
  result?: DescribeResult
}

export interface DescribeBatchResult {
  entries: DescribeBatchEntryResult[]
  ok: number
  error: number
  skipped: number
}

const AI_METADATA_DESCRIBE_TIMEOUT_MS = 600_000

function plainTextFromResponse(text: string): string {
  return text.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
}

async function readDescribeResponse(res: Response): Promise<{ data: DescribeResult | null; error: string | null }> {
  const text = await res.text()
  if (!text) {
    return res.ok ? { data: null, error: null } : { data: null, error: `HTTP ${res.status}` }
  }

  const contentType = res.headers.get('content-type') ?? ''
  const looksJSON = contentType.includes('application/json') || /^[\s[{]/.test(text)
  if (!looksJSON) {
    const message = plainTextFromResponse(text)
    return { data: null, error: message ? `HTTP ${res.status}: ${message}` : `HTTP ${res.status}` }
  }

  try {
    const data = JSON.parse(text) as DescribeResult & { error?: string }
    if (!res.ok) return { data: null, error: data.error || `HTTP ${res.status}` }
    return { data, error: null }
  } catch {
    const message = plainTextFromResponse(text)
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
