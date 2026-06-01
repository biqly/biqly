import { MultiSelect } from '../ui/MultiSelect'
import { Select } from '../ui/Select'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
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
  const tableLabel = (table: (typeof tables)[number]) => table.label || `${table.schema_name}.${table.table_name}`

  const tablesInTypeScope = tables.filter((table) => {
    const typ = (table.table_type || '').toUpperCase()
    if (typ === 'VIEW') return includeViews
    if (typ === 'BASE TABLE') return includeBaseTables
    return includeBaseTables
  })

  const filteredTables = (() => {
    const search = tableSearch.trim().toLowerCase()
    return tablesInTypeScope.filter((table) => {
      if (!search) return true
      return tableLabel(table).toLowerCase().includes(search) || (table.description || '').toLowerCase().includes(search)
    })
  })()

  const embedButtonLabel =
    embeddingLoading || embeddingRunning
      ? t('ai_query.embed_refreshing_short')
      : semanticModelId
        ? t('ai_query.embed_refresh_model')
        : t('ai_query.embed_refresh')

  // When provider/model selection is DB-managed, the active model per purpose
  // is authoritative; otherwise fall back to the env-backed fields.
  const dbManaged = aiRuntime?.db_managed === true
  const activeFor = (purpose: 'query' | 'embedding' | 'translation') =>
    aiRuntime?.active_models?.find((m) => m.purpose === purpose)
  const activeQuery = activeFor('query')
  const activeEmbedding = activeFor('embedding')
  const activeTranslation = activeFor('translation')

  const queryModel = dbManaged && activeQuery
    ? activeQuery.display_name
    : aiRuntime?.query_model_override ? aiRuntime?.query_model : aiRuntime?.llm_model
  const queryNote = dbManaged
    ? activeQuery?.provider_name
    : aiRuntime?.query_model_override
      ? undefined
      : aiRuntime
        ? t('ai_query.model_badge_legacy')
        : undefined
  const embeddingBadge = dbManaged
    ? activeEmbedding?.display_name
    : aiRuntime?.embeddings_enabled ? aiRuntime?.embedding_model : undefined
  const translationBadge = dbManaged
    ? activeTranslation?.display_name
    : aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined

  return (
    <header className="query-config-header">
      <div className="query-config-top">
        <ModelBadgeRow
          primaryLabel={t('ai_query.model_badge_query')}
          primaryModel={queryModel}
          primaryNote={queryNote}
          embeddingModel={embeddingBadge}
          translationModel={translationBadge}
          className="query-config-badges"
        />
        {aiRuntime?.embeddings_enabled === true && (
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
      {aiRuntime?.embeddings_enabled === true && (embeddingStatus || embeddingError || aiRuntimeErr) && (
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
                label: m.label || m.name,
                hint: m.status,
              })),
            ]}
          />
        </div>
        <div className="form-group routing-toggle">
          <span className="form-label">{t('ai_query.table_routing_label')}</span>
          <div className="routing-toggle-row">
            <div className="toggle-group" role="group" aria-label={t('ai_query.table_routing_label')}>
              <button type="button" className={`toggle-btn ${autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(true)}>{t('ai_query.table_routing_auto')}</button>
              <button type="button" className={`toggle-btn ${!autoTableRouting ? 'active' : ''}`} onClick={() => setAutoTableRouting(false)}>{t('ai_query.table_routing_manual')}</button>
            </div>
          </div>
        </div>
      </div>

      {!autoTableRouting && (
        <div className="form-group">
          <span className="ai-scope-label">{t('ai_query.scope_label')}</span>
          <div className="ai-scope-type-filters" role="group">
            <label className="ai-scope-type-option">
              <input type="checkbox" checked={includeBaseTables} onChange={(e) => setIncludeBaseTables(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
              <span>{t('ai_query.scope_base_tables')}</span>
            </label>
            <label className="ai-scope-type-option">
              <input type="checkbox" checked={includeViews} onChange={(e) => setIncludeViews(e.target.checked)} disabled={!datasourceId || tables.length === 0} />
              <span>{t('ai_query.scope_views')}</span>
            </label>
          </div>
          <input value={tableSearch} onChange={(e) => setTableSearch(e.target.value)} placeholder={t('ai_query.table_search_placeholder')} disabled={!datasourceId || tables.length === 0} autoComplete="off" />
          <MultiSelect
            display="inline"
            className="ai-scope-multiselect"
            ariaLabel={t('ai_query.selected_tables_aria')}
            value={selectedTables}
            onChange={setSelectedTables}
            disabled={!datasourceId || tables.length === 0 || (!includeBaseTables && !includeViews)}
            maxHeight={Math.min(288, Math.max(120, (filteredTables.length || 3) * 36))}
            options={filteredTables.map((table) => {
              const label = tableLabel(table)
              return { value: label, label }
            })}
          />
        </div>
      )}
    </header>
  )
}
