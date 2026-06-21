import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { legacyFormClass } from '../../lib/formClasses'
import { toggleBtnClass, toggleGroupClass } from '../../lib/toggleClasses'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { Select } from '../ui/Select'
import {
  aiEmbeddingErrorClass,
  aiEmbeddingStatusClass,
  queryConfigEmbedBtnClass,
  queryConfigEmbedStatusClass,
  queryConfigHeaderClass,
  queryConfigTopClass,
  queryControlsClass,
  routingToggleRowBtnClass,
  routingToggleRowGroupClass,
} from './aiQueryClasses'
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
    <header className={queryConfigHeaderClass}>
      <div className={queryConfigTopClass}>
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
            className={cn(buttonClass('secondary', { size: 'sm' }), queryConfigEmbedBtnClass)}
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
        <div className={queryConfigEmbedStatusClass}>
          {embeddingStatus && <span className={aiEmbeddingStatusClass}>{embeddingStatus}</span>}
          {embeddingError && <span className={aiEmbeddingErrorClass}>{embeddingError}</span>}
          {aiRuntimeErr && <span className={legacyFeedbackClass('error')}>{aiRuntimeErr}</span>}
        </div>
      )}

      <div className={queryControlsClass}>
        <div className={legacyFormClass('form-group')}>
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
        <div className={legacyFormClass('form-group')}>
          <label htmlFor="ai-semantic-model">{t('ai_query.semantic_model_label')}</label>
          <div className="flex items-stretch gap-3">
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
            <div className="flex items-center self-stretch">
              <div
                className={toggleGroupClass(routingToggleRowGroupClass)}
                role="group"
                aria-label={t('ai_query.table_routing_label')}
              >
                <button
                  type="button"
                  className={toggleBtnClass(autoTableRouting, routingToggleRowBtnClass)}
                  onClick={() => setAutoTableRouting(true)}
                >
                  {t('ai_query.table_routing_auto')}
                </button>
                <button
                  type="button"
                  className={toggleBtnClass(!autoTableRouting, routingToggleRowBtnClass)}
                  onClick={() => setAutoTableRouting(false)}
                >
                  {t('ai_query.table_routing_manual')}
                </button>
              </div>
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
