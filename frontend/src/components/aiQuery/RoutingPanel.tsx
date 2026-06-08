import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { Select } from '../ui/Select'
import { RoutingPanelManualScope } from './RoutingPanelManualScope'
import {
  filterRoutingTables,
  resolveRoutingModelBadges,
  routingEmbedButtonLabel,
} from './routingPanelUtils'
import type { RoutingPanelProps } from './types'

export function RoutingPanel({
  t,
  aiRuntime,
  aiRuntimeErr,
  datasources,
  datasourceId,
  setDatasourceId,
  semanticModels,
  semanticModelId,
  setSemanticModelId,
  composites,
  tables,
  selectedTables,
  setSelectedTables,
  tableSearch,
  setTableSearch,
  includeBaseTables,
  setIncludeBaseTables,
  includeViews,
  setIncludeViews,
  autoTableRouting,
  setAutoTableRouting,
  embeddingStatus,
  embeddingError,
  embeddingLoading,
  embeddingRunning,
  selectedDatasourceName,
  semanticModelName,
  onRefreshEmbeddings,
}: RoutingPanelProps) {
  const filteredTables = filterRoutingTables(tables, {
    includeBaseTables,
    includeViews,
    tableSearch,
  })

  const embedButtonLabel = routingEmbedButtonLabel(
    t,
    embeddingLoading,
    embeddingRunning,
    semanticModelId,
  )

  const {
    queryModel,
    queryNote,
    embeddingsAvailable,
    embeddingBadge,
    embeddingNote,
    translationBadge,
    translationNote,
  } = resolveRoutingModelBadges(aiRuntime, t)

  return (
    <header className="query-config-header">
      <div className="query-config-top">
        <ModelBadgeRow
          primaryLabel={t('ai_query.model_badge_query')}
          primaryModel={queryModel}
          primaryNote={queryNote}
          embeddingModel={embeddingBadge}
          embeddingNote={embeddingNote}
          translationModel={translationBadge}
          translationNote={translationNote}
          className="query-config-badges"
        />
        {embeddingsAvailable && (
          <button
            type="button"
            className="btn btn-sm query-config-embed-btn"
            onClick={onRefreshEmbeddings}
            disabled={!datasourceId || embeddingLoading || embeddingRunning}
            title={
              datasourceId
                ? semanticModelId
                  ? t('ai_query.embed_title_model', { name: semanticModelName })
                  : t('ai_query.embed_title_ds', { name: selectedDatasourceName ?? '' })
                : t('ai_query.embed_title_none')
            }
          >
            {embedButtonLabel}
          </button>
        )}
      </div>
      {embeddingsAvailable && (embeddingStatus ?? embeddingError ?? aiRuntimeErr) && (
        <div className="query-config-embed-status">
          {embeddingStatus && <span className="ai-embedding-status">{embeddingStatus}</span>}
          {embeddingError && <span className="ai-embedding-error">{embeddingError}</span>}
          {aiRuntimeErr && <span className="error">{aiRuntimeErr}</span>}
        </div>
      )}

      <div className="query-controls">
        <div className="form-group">
          <label htmlFor="ai-datasource">{t('ai_query.datasource_label')}</label>
          <Select
            id="ai-datasource"
            value={datasourceId}
            onChange={setDatasourceId}
            placeholder={t('ai_query.select_placeholder')}
            header={t('ai_query.header_datasources')}
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <div className="form-group">
          <label htmlFor="ai-semantic-model">{t('ai_query.semantic_model_label')}</label>
          <Select
            id="ai-semantic-model"
            value={semanticModelId}
            onChange={setSemanticModelId}
            placeholder={t('ai_query.semantic_model_auto')}
            header={t('ai_query.semantic_model_header')}
            options={[
              { value: '', label: t('ai_query.semantic_model_auto') },
              ...semanticModels.map((m) => ({
                value: m.id,
                label: m.label ?? m.name,
                hint: m.status,
              })),
              ...(composites ?? []).map((c) => ({
                value: `composite:${c.id}`,
                label: `⧉ ${c.label ?? c.name}`,
                hint: c.status,
              })),
            ]}
          />
        </div>
        <div className="form-group routing-toggle">
          <span className="form-label">{t('ai_query.table_routing_label')}</span>
          <div className="routing-toggle-row">
            <div
              className="toggle-group"
              role="group"
              aria-label={t('ai_query.table_routing_label')}
            >
              <button
                type="button"
                className={`toggle-btn ${autoTableRouting ? 'active' : ''}`}
                onClick={() => setAutoTableRouting(true)}
              >
                {t('ai_query.table_routing_auto')}
              </button>
              <button
                type="button"
                className={`toggle-btn ${!autoTableRouting ? 'active' : ''}`}
                onClick={() => setAutoTableRouting(false)}
              >
                {t('ai_query.table_routing_manual')}
              </button>
            </div>
          </div>
        </div>
      </div>

      {!autoTableRouting && (
        <RoutingPanelManualScope
          t={t}
          datasourceId={datasourceId}
          tables={tables}
          includeBaseTables={includeBaseTables}
          setIncludeBaseTables={setIncludeBaseTables}
          includeViews={includeViews}
          setIncludeViews={setIncludeViews}
          tableSearch={tableSearch}
          setTableSearch={setTableSearch}
          selectedTables={selectedTables}
          setSelectedTables={setSelectedTables}
          filteredTables={filteredTables}
        />
      )}
    </header>
  )
}
