import type { AIRuntimeSettings } from '../../types/ai'
import type { RoutingPanelProps } from './types'

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

function activeModelsByPurpose(aiRuntime: AIRuntimeSettings | null | undefined) {
  const models = aiRuntime?.active_models ?? []
  return new Map(models.map((m) => [m.purpose, m] as const))
}

function resolveQueryBadge(
  activeModels: Map<string, NonNullable<AIRuntimeSettings['active_models']>[number]>,
  t: RoutingPanelProps['t'],
  dbManaged: boolean,
  aiRuntime: AIRuntimeSettings | null | undefined,
) {
  const activeQuery = activeModels.get('query')
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
  activeModels: Map<string, NonNullable<AIRuntimeSettings['active_models']>[number]>,
  dbManaged: boolean,
  aiRuntime: AIRuntimeSettings | null | undefined,
) {
  const activeEmbedding = activeModels.get('embedding')
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
  activeModels: Map<string, NonNullable<AIRuntimeSettings['active_models']>[number]>,
  dbManaged: boolean,
  aiRuntime: AIRuntimeSettings | null | undefined,
) {
  const activeTranslation = activeModels.get('translation')
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
  const activeModels = activeModelsByPurpose(aiRuntime)
  return {
    ...resolveQueryBadge(activeModels, t, dbManaged, aiRuntime),
    ...resolveEmbeddingBadge(activeModels, dbManaged, aiRuntime),
    ...resolveTranslationBadge(activeModels, dbManaged, aiRuntime),
  }
}
