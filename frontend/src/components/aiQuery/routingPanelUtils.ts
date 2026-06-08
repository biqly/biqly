import type { AIRuntimeSettings } from '../../types/ai'
import type { RoutingPanelProps } from './types'

type RoutingTable = RoutingPanelProps['tables'][number]

export function routingTableLabel(table: RoutingTable): string {
  return table.label ?? `${table.schema_name}.${table.table_name}`
}

export function filterRoutingTables(
  tables: RoutingTable[],
  options: {
    includeBaseTables: boolean
    includeViews: boolean
    tableSearch: string
  },
): RoutingTable[] {
  const tablesInTypeScope = tables.filter((table) => {
    const typ = (table.table_type ?? '').toUpperCase()
    if (typ === 'VIEW') {
      return options.includeViews
    }
    if (typ === 'BASE TABLE') {
      return options.includeBaseTables
    }
    return options.includeBaseTables
  })

  const search = options.tableSearch.trim().toLowerCase()
  if (!search) {
    return tablesInTypeScope
  }
  return tablesInTypeScope.filter((table) => {
    const label = routingTableLabel(table)
    return (
      label.toLowerCase().includes(search) ||
      (table.description ?? '').toLowerCase().includes(search)
    )
  })
}

export function routingEmbedButtonLabel(
  t: RoutingPanelProps['t'],
  embeddingLoading: boolean,
  embeddingRunning: boolean,
  semanticModelId: string,
): string {
  if (embeddingLoading || embeddingRunning) {
    return t('ai_query.embed_refreshing_short')
  }
  if (semanticModelId) {
    return t('ai_query.embed_refresh_model')
  }
  return t('ai_query.embed_refresh')
}

function activeModelFor(
  aiRuntime: AIRuntimeSettings | null | undefined,
  purpose: 'query' | 'embedding' | 'translation',
) {
  return aiRuntime?.active_models?.find((m) => m.purpose === purpose)
}

function resolveQueryBadge(
  aiRuntime: AIRuntimeSettings | null | undefined,
  t: RoutingPanelProps['t'],
  dbManaged: boolean,
) {
  const activeQuery = activeModelFor(aiRuntime, 'query')
  const queryModel =
    dbManaged && activeQuery
      ? activeQuery.display_name
      : aiRuntime?.query_model_override
        ? aiRuntime.query_model
        : aiRuntime?.llm_model
  const queryNote = dbManaged
    ? activeQuery?.provider_name
    : aiRuntime?.query_model_override
      ? undefined
      : aiRuntime
        ? t('ai_query.model_badge_legacy')
        : undefined
  return { queryModel, queryNote }
}

function resolveEmbeddingBadge(
  aiRuntime: AIRuntimeSettings | null | undefined,
  dbManaged: boolean,
) {
  const activeEmbedding = activeModelFor(aiRuntime, 'embedding')
  const embeddingsAvailable = dbManaged
    ? Boolean(activeEmbedding?.model_id.trim())
    : aiRuntime?.embeddings_enabled === true
  const embeddingBadge = embeddingsAvailable
    ? dbManaged
      ? activeEmbedding?.display_name
      : aiRuntime?.embedding_model
    : undefined
  const embeddingNote = dbManaged ? activeEmbedding?.provider_name : undefined
  return { embeddingsAvailable, embeddingBadge, embeddingNote }
}

function resolveTranslationBadge(
  aiRuntime: AIRuntimeSettings | null | undefined,
  dbManaged: boolean,
) {
  const activeTranslation = activeModelFor(aiRuntime, 'translation')
  const translationBadge = dbManaged
    ? activeTranslation?.display_name
    : aiRuntime?.translation_enabled
      ? aiRuntime.translation_model
      : undefined
  const translationNote = dbManaged ? activeTranslation?.provider_name : undefined
  return { translationBadge, translationNote }
}

export function resolveRoutingModelBadges(
  aiRuntime: AIRuntimeSettings | null | undefined,
  t: RoutingPanelProps['t'],
) {
  const dbManaged = aiRuntime?.db_managed === true
  return {
    ...resolveQueryBadge(aiRuntime, t, dbManaged),
    ...resolveEmbeddingBadge(aiRuntime, dbManaged),
    ...resolveTranslationBadge(aiRuntime, dbManaged),
  }
}
