import type { TFunction, TranslationKey } from '../../i18n/locale'
import type { AIQueryResponse, EmbedMetadataResponse } from '../../types/ai'

export function formatAiWaitElapsed(ms: number, t: TFunction): string {
  if (ms < 1000) {
    return t('ai_query.wait_ms', { ms })
  }
  const sec = ms / 1000
  if (sec < 60) {
    return t('ai_query.wait_sec', { sec: Number(sec.toFixed(1)) })
  }
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return t('ai_query.wait_min_sec', { m, s })
}

export function warningBodyKey(result: AIQueryResponse): TranslationKey {
  const hasQueryShapeWarning = result.warnings?.some((warning) =>
    /validation|semantic|unknown (dimension|field|metric)|ambiguous|dry-run|compilation|compile/i.test(
      warning,
    ),
  )
  if (result.sql && !hasQueryShapeWarning) {
    return 'ai_query.warnings_body_success'
  }
  return 'ai_query.warnings_body'
}

export function routingMethodLabel(method: string | undefined, t: TFunction): string {
  const m = (method ?? 'keyword').toLowerCase()
  if (m === 'keyword') {
    return t('ai_query.routing_method_keyword')
  }
  if (m === 'vector') {
    return t('ai_query.routing_method_vector')
  }
  if (m === 'hybrid') {
    return t('ai_query.routing_method_hybrid')
  }
  if (m === 'manual') {
    return t('ai_query.routing_method_manual')
  }
  if (m === 'semantic') {
    return t('ai_query.routing_method_semantic')
  }
  return method ?? t('ai_query.routing_method_keyword')
}

export function contextSourceLabel(source: string | undefined, t: TFunction): string {
  const s = (source ?? 'auto').toLowerCase()
  if (s === 'semantic_model') {
    return t('ai_query.context_source_semantic_model')
  }
  if (s === 'manual') {
    return t('ai_query.context_source_manual')
  }
  if (s === 'auto') {
    return t('ai_query.context_source_auto')
  }
  return source ?? t('ai_query.context_source_auto')
}

export function compactItems(items: string[] | undefined, limit = 8) {
  if (!items || items.length === 0) {
    return null
  }
  const visible = items.slice(0, limit)
  const rest = items.length - visible.length
  return { visible, rest }
}

export function compactList(items: string[] | undefined, limit = 8) {
  const compacted = compactItems(items, limit)
  if (!compacted) {
    return null
  }
  return `${compacted.visible.join(', ')}${compacted.rest > 0 ? ` +${compacted.rest}` : ''}`
}

export function embeddingSummary(response: EmbedMetadataResponse, t: TFunction): string {
  const tableKeys = new Set<string>()
  const columnKeys = new Set<string>()
  for (const item of response.results ?? []) {
    if (item.skipped) {
      continue
    }
    const kind = item.kind ?? 'table'
    if (kind === 'column') {
      columnKeys.add(`${item.schema}.${item.table}.${item.column ?? ''}`)
    } else {
      tableKeys.add(`${item.schema}.${item.table}`)
    }
  }
  const tables = tableKeys.size
  const columns = columnKeys.size
  const vectors = response.embedded
  const unique = tables + columns
  const locales = unique > 0 ? Math.max(1, Math.round(vectors / unique)) : 1
  return t('ai_query.embedding_summary', {
    tables,
    columns,
    vectors,
    locales,
    model: response.model,
  })
}
