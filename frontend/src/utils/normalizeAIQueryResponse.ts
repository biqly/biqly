import type { AIQueryResponse } from '../types/ai'

function isRecord(v: unknown): v is Record<string, unknown> {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

function isNestedAIResult(inner: Record<string, unknown>): boolean {
  return (
    'sql' in inner ||
    'logical_query' in inner ||
    'confidence' in inner ||
    'warnings' in inner ||
    (isRecord(inner.result) && ('rows' in inner.result || 'columns' in inner.result))
  )
}

function pickClarification(
  raw: Record<string, unknown>,
  nested?: Record<string, unknown>,
): Pick<
  AIQueryResponse,
  'needs_clarification' | 'clarification_question' | 'clarification_options' | 'clarification'
> {
  const clar = nested ?? raw
  return {
    needs_clarification: Boolean(clar.needs_clarification ?? raw.needs_clarification),
    clarification_question:
      typeof clar.clarification_question === 'string'
        ? clar.clarification_question
        : typeof raw.clarification_question === 'string'
          ? raw.clarification_question
          : undefined,
    clarification_options: Array.isArray(clar.clarification_options)
      ? (clar.clarification_options as string[])
      : Array.isArray(raw.clarification_options)
        ? (raw.clarification_options as string[])
        : undefined,
    clarification:
      clar.clarification != null
        ? (clar.clarification as AIQueryResponse['clarification'])
        : raw.clarification != null
          ? (raw.clarification as AIQueryResponse['clarification'])
          : undefined,
  }
}

/** Unwraps backend `ai.Response` ({ result, metadata, clarification }) into flat `AIQueryResponse`. */
export function normalizeAIQueryResponse(raw: unknown): AIQueryResponse | null {
  if (!isRecord(raw)) {
    return null
  }

  const inner = raw.result
  const metadata = isRecord(raw.metadata) ? raw.metadata : null
  const nestedClar = isRecord(raw.clarification) ? raw.clarification : undefined

  const shouldUnwrap =
    isRecord(inner) && isNestedAIResult(inner) && !('sql' in raw) && !('logical_query' in raw)
  if (!shouldUnwrap) {
    return { ...(raw as AIQueryResponse), ...pickClarification(raw, nestedClar) }
  }

  const flat: AIQueryResponse = {
    logical_query: inner.logical_query as AIQueryResponse['logical_query'],
    sql: typeof inner.sql === 'string' ? inner.sql : undefined,
    warnings: Array.isArray(inner.warnings) ? (inner.warnings as string[]) : undefined,
    result: inner.result as AIQueryResponse['result'],
    confidence: typeof inner.confidence === 'number' ? inner.confidence : undefined,
    visualization_hint: inner.visualization_hint as AIQueryResponse['visualization_hint'],
    ...pickClarification(raw, nestedClar),
  }

  if (metadata) {
    flat.model_used = typeof metadata.model_used === 'string' ? metadata.model_used : undefined
    flat.prompt_stats = metadata.prompt_stats as AIQueryResponse['prompt_stats']
    flat.token_usage = metadata.token_usage as AIQueryResponse['token_usage']
    flat.cost_usd = typeof metadata.cost_usd === 'number' ? metadata.cost_usd : undefined
    flat.latency_ms = typeof metadata.latency_ms === 'number' ? metadata.latency_ms : undefined
    flat.retry_count = typeof metadata.retry_count === 'number' ? metadata.retry_count : undefined
    flat.table_routing = metadata.table_routing as AIQueryResponse['table_routing']
    flat.validation_result = metadata.validation_result as AIQueryResponse['validation_result']
    flat.candidates = metadata.candidates as AIQueryResponse['candidates']
    flat.candidates_count =
      typeof metadata.candidates_count === 'number' ? metadata.candidates_count : undefined
  }

  return flat
}
