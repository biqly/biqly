import type { AIQueryResponse } from '../types/ai'

function isRecord(v: unknown): v is Record<string, unknown> {
  return v != null && typeof v === 'object' && !Array.isArray(v)
}

function toStringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined
  }
  const strings = value.filter((x): x is string => typeof x === 'string')
  return strings.length > 0 ? strings : undefined
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
  | 'needs_clarification'
  | 'clarification_question'
  | 'clarification_options'
  | 'clarification_round'
  | 'resolved_question'
  | 'clarification'
> {
  const clar = nested ?? raw
  const clarificationRound =
    typeof clar.clarification_round === 'number'
      ? clar.clarification_round
      : typeof raw.clarification_round === 'number'
        ? raw.clarification_round
        : undefined
  const clarificationValue = clar.clarification ?? raw.clarification
  return {
    needs_clarification: Boolean(clar.needs_clarification ?? raw.needs_clarification),
    clarification_question:
      typeof clar.clarification_question === 'string'
        ? clar.clarification_question
        : typeof raw.clarification_question === 'string'
          ? raw.clarification_question
          : undefined,
    clarification_options:
      toStringArray(clar.clarification_options) ?? toStringArray(raw.clarification_options),
    clarification_round: clarificationRound,
    resolved_question:
      typeof clar.resolved_question === 'string'
        ? clar.resolved_question
        : typeof raw.resolved_question === 'string'
          ? raw.resolved_question
          : undefined,
    clarification: isRecord(clarificationValue)
      ? (clarificationValue as AIQueryResponse['clarification'])
      : undefined,
  }
}

function assignQueryFields(flat: AIQueryResponse, source: Record<string, unknown>): void {
  if (isRecord(source.logical_query)) {
    flat.logical_query = source.logical_query as AIQueryResponse['logical_query']
  }
  if (typeof source.sql === 'string') {
    flat.sql = source.sql
  }
  flat.warnings = toStringArray(source.warnings)
  if (isRecord(source.result)) {
    flat.result = source.result as AIQueryResponse['result']
  }
  if (typeof source.confidence === 'number') {
    flat.confidence = source.confidence
  }
  if (isRecord(source.visualization_hint)) {
    flat.visualization_hint = source.visualization_hint as AIQueryResponse['visualization_hint']
  }
}

function assignMetadataFields(flat: AIQueryResponse, metadata: Record<string, unknown>): void {
  if (typeof metadata.model_used === 'string') {
    flat.model_used = metadata.model_used
  }
  if (isRecord(metadata.prompt_stats)) {
    flat.prompt_stats = metadata.prompt_stats
  }
  if (isRecord(metadata.token_usage)) {
    flat.token_usage = metadata.token_usage as AIQueryResponse['token_usage']
  }
  if (typeof metadata.cost_usd === 'number') {
    flat.cost_usd = metadata.cost_usd
  }
  if (typeof metadata.latency_ms === 'number') {
    flat.latency_ms = metadata.latency_ms
  }
  if (typeof metadata.retry_count === 'number') {
    flat.retry_count = metadata.retry_count
  }
  if (isRecord(metadata.table_routing)) {
    flat.table_routing = metadata.table_routing as AIQueryResponse['table_routing']
  }
  if (isRecord(metadata.validation_result)) {
    flat.validation_result = metadata.validation_result as AIQueryResponse['validation_result']
  }
  if (Array.isArray(metadata.candidates)) {
    flat.candidates = metadata.candidates as AIQueryResponse['candidates']
  }
  if (typeof metadata.candidates_count === 'number') {
    flat.candidates_count = metadata.candidates_count
  }
  if (isRecord(metadata.generation_trace)) {
    flat.generation_trace = metadata.generation_trace
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
    const flat: AIQueryResponse = { ...pickClarification(raw, nestedClar) }
    assignQueryFields(flat, raw)
    return flat
  }

  const flat: AIQueryResponse = { ...pickClarification(raw, nestedClar) }
  assignQueryFields(flat, inner)
  if (metadata) {
    assignMetadataFields(flat, metadata)
  }

  return flat
}
