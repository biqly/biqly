import type { TranslationKey } from '../../i18n'
import type {
  SemanticDimension,
  SemanticJoin,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import type { SuggestedJoin, Tab } from './types'
import { tableKey } from './utils'

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string
interface EntityImpact {
  joins: number
  dims: number
  metrics: number
}

interface ModelingPaletteProps {
  open: boolean
  onToggle: () => void
  model: SemanticModelDetail | null
  usedTableCount: number
  joins: SemanticJoin[]
  inactiveJoins: SemanticJoin[]
  dims: SemanticDimension[]
  inactiveDims: SemanticDimension[]
  metrics: SemanticMetric[]
  inactiveMetrics: SemanticMetric[]
  activeTab: Tab
  onTabChange: (tab: Tab) => void
  tables: TableRow[]
  includedTables: TableRow[]
  excludedSchemas: Set<string>
  tableCards: TableRow[]
  tableImpact: (schema: string, table: string) => EntityImpact
  suggestedJoins: SuggestedJoin[]
  highlightJoinId: string | null
  onHighlightJoin: (joinId: string | null) => void
  onSchemaToggle: (schemaName: string, isExcluded: boolean) => void
  onRenameTable: (table: TableRow) => void
  onMakeBase: (schema: string, table: string) => void
  onRemoveTable: (schema: string, table: string) => void
  onToggleTableVisibility: (schema: string, table: string, forceShow: boolean) => void
  onOpenBaseSwap: () => void
  onDeleteJoin: (joinId: string) => void
  onAddSuggestedJoin: (join: SuggestedJoin) => void
  onReactivateJoin: (join: SemanticJoin) => void
  onRenameDimension: (dimension: SemanticDimension) => void
  onEditDimensionValues: (dimension: SemanticDimension) => void
  onDeleteDimension: (dimensionId: string) => void
  onReactivateDimension: (dimension: SemanticDimension) => void
  onOpenAddMetric: () => void
  onRenameMetric: (metric: SemanticMetric) => void
  onDeleteMetric: (metricId: string) => void
  onReactivateMetric: (metric: SemanticMetric) => void
  t: Translate
}

export function ModelingPalette({
  open,
  onToggle,
  model,
  usedTableCount,
  joins,
  inactiveJoins,
  dims,
  inactiveDims,
  metrics,
  inactiveMetrics,
  activeTab,
  onTabChange,
  tables,
  includedTables,
  excludedSchemas,
  tableCards,
  tableImpact,
  suggestedJoins,
  highlightJoinId,
  onHighlightJoin,
  onSchemaToggle,
  onRenameTable,
  onMakeBase,
  onRemoveTable,
  onToggleTableVisibility,
  onOpenBaseSwap,
  onDeleteJoin,
  onAddSuggestedJoin,
  onReactivateJoin,
  onRenameDimension,
  onEditDimensionValues,
  onDeleteDimension,
  onReactivateDimension,
  onOpenAddMetric,
  onRenameMetric,
  onDeleteMetric,
  onReactivateMetric,
  t,
}: ModelingPaletteProps) {
  return (
    <aside
      className={`modeling-palette ${open ? '' : 'modeling-side--collapsed'}`}
      aria-label={t('modeling.model_summary_aria')}
    >
      <button
        type="button"
        className="modeling-side-toggle modeling-side-toggle--left"
        onClick={onToggle}
        title={open ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
      >
        {open ? '‹' : '›'}
      </button>
      <div className="modeling-side-body">
        <div>
          <span className="modeling-kicker">{t('modeling.semantic_layer')}</span>
          <h2>{model?.label || model?.name || t('modeling.no_model_selected')}</h2>
          <p>{t('modeling.semantic_description')}</p>
        </div>
        <div className="modeling-stat-grid">
          <div>
            <strong>{usedTableCount}</strong>
            <span>{t('modeling.tab_short_tables')}</span>
          </div>
          <div>
            <strong>{joins.length}</strong>
            <span>{t('modeling.tab_short_rel')}</span>
          </div>
          <div>
            <strong>{dims.length}</strong>
            <span>{t('modeling.tab_short_dim')}</span>
          </div>
          <div>
            <strong>{metrics.length}</strong>
            <span>{t('modeling.tab_short_metric')}</span>
          </div>
        </div>

        <div className="modeling-tabs">
          {(['tables', 'joins', 'dimensions', 'metrics'] as const).map((tab) => (
            <button
              className={`modeling-tab ${activeTab === tab ? 'modeling-tab--active' : ''}`}
              key={tab}
              onClick={() => onTabChange(tab)}
              title={t(`modeling.${tab}_tab`)}
            >
              {t(
                tab === 'joins'
                  ? 'modeling.tab_short_rel'
                  : `modeling.tab_short_${tab === 'dimensions' ? 'dim' : tab === 'metrics' ? 'metric' : 'tables'}`,
              )}
            </button>
          ))}
        </div>

        <div className="modeling-tab-content">
          {activeTab === 'tables' && (
            <div className="modeling-join-list">
              <h3>{t('modeling.schemas_heading')}</h3>
              {Array.from(new Set(tables.map((table) => table.schema_name)))
                .sort()
                .map((schemaName) => {
                  const isExcluded = excludedSchemas.has(schemaName)
                  return (
                    <div
                      className={`modeling-join-pill ${isExcluded ? '' : 'modeling-join-pill--active'}`}
                      key={`schema-${schemaName}`}
                    >
                      <div className="modeling-join-pill-header">
                        <strong>{schemaName}</strong>
                        <button
                          className={isExcluded ? 'modeling-add-btn' : 'modeling-delete-btn'}
                          onClick={() => onSchemaToggle(schemaName, isExcluded)}
                          title={
                            isExcluded
                              ? t('modeling.include_schema_again_title')
                              : t('modeling.exclude_schema_title_short')
                          }
                        >
                          {isExcluded ? '+' : '×'}
                        </button>
                      </div>
                      <span className="modeling-join-meta">
                        {isExcluded
                          ? t('modeling.schema_excluded_status')
                          : t('modeling.schema_included_status')}
                      </span>
                    </div>
                  )
                })}

              <h3>{t('modeling.datasource_tables_heading')}</h3>
              {tables.length === 0 ? (
                <p className="modeling-empty">{t('modeling.no_tables_sync')}</p>
              ) : (
                includedTables.map((table) => {
                  const key = tableKey(table.schema_name, table.table_name)
                  const isOnCanvas = tableCards.some(
                    (card) => tableKey(card.schema_name, card.table_name) === key,
                  )
                  const isBase = model
                    ? key === tableKey(model.base_schema, model.base_table)
                    : false
                  const impact = model
                    ? tableImpact(table.schema_name, table.table_name)
                    : { joins: 0, dims: 0, metrics: 0 }
                  const inModel =
                    isBase || impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
                  return (
                    <div
                      className={`modeling-join-pill ${isOnCanvas ? 'modeling-join-pill--active' : ''}`}
                      key={table.id}
                    >
                      <div className="modeling-join-pill-header">
                        <strong>
                          {table.label || table.table_name}
                          {isBase && (
                            <span
                              className="modeling-base-badge"
                              title={t('modeling.base_table_label')}
                            >
                              {' '}
                              ★
                            </span>
                          )}
                        </strong>
                        <span className="modeling-pill-actions">
                          <button
                            className="modeling-rename-btn"
                            onClick={() => onRenameTable(table)}
                            title={t('modeling.edit_display_name_title')}
                          >
                            ✎
                          </button>
                          {!isBase && inModel && (
                            <button
                              className="modeling-rename-btn"
                              onClick={() => onMakeBase(table.schema_name, table.table_name)}
                              title={t('modeling.make_base_title')}
                            >
                              ★
                            </button>
                          )}
                          {isOnCanvas && !isBase && (
                            <button
                              className="modeling-delete-btn"
                              onClick={() => onRemoveTable(table.schema_name, table.table_name)}
                              title={
                                inModel
                                  ? t('modeling.remove_from_model_title')
                                  : t('modeling.hide_from_canvas_title')
                              }
                            >
                              ×
                            </button>
                          )}
                          {!isOnCanvas && (
                            <button
                              className="modeling-add-btn"
                              onClick={() =>
                                onToggleTableVisibility(table.schema_name, table.table_name, true)
                              }
                              title={t('modeling.show_on_canvas_title')}
                            >
                              +
                            </button>
                          )}
                          {isBase && (
                            <button
                              className="modeling-delete-btn"
                              onClick={onOpenBaseSwap}
                              title={t('modeling.change_base_title')}
                            >
                              ×
                            </button>
                          )}
                        </span>
                      </div>
                      <span className="modeling-join-meta">
                        {table.schema_name}.{table.table_name}
                      </span>
                      <span className="modeling-join-meta">
                        {isOnCanvas ? t('modeling.on_canvas') : t('modeling.not_visible')}
                      </span>
                    </div>
                  )
                })
              )}
            </div>
          )}

          {activeTab === 'joins' && (
            <div className="modeling-join-list">
              <h3>{t('modeling.active_relationships')}</h3>
              {joins.length === 0 ? (
                <p className="modeling-empty">{t('modeling.no_relationships')}</p>
              ) : (
                joins.map((join) => (
                  <div
                    className={`modeling-join-pill ${highlightJoinId === join.id ? 'modeling-join-pill--active' : ''}`}
                    key={join.id}
                    onMouseEnter={() => onHighlightJoin(join.id)}
                    onMouseLeave={() => onHighlightJoin(null)}
                  >
                    <div className="modeling-join-pill-header">
                      <strong>{join.name}</strong>
                      <button
                        className="modeling-delete-btn"
                        onClick={() => onDeleteJoin(join.id)}
                        title={t('modeling.delete_relationship_title')}
                      >
                        ×
                      </button>
                    </div>
                    <span>
                      {join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}
                    </span>
                    <span className="modeling-join-meta">
                      {join.join_type} · {join.relationship}
                    </span>
                  </div>
                ))
              )}
              {suggestedJoins.length > 0 && (
                <>
                  <h3>{t('modeling.suggested_fk_relationships')}</h3>
                  {suggestedJoins.map((join, index) => (
                    <div className="modeling-join-pill modeling-join-pill--suggested" key={index}>
                      <div className="modeling-join-pill-header">
                        <strong>
                          {join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}
                        </strong>
                        <button
                          className="modeling-add-btn"
                          onClick={() => onAddSuggestedJoin(join)}
                          title={t('common.add')}
                        >
                          +
                        </button>
                      </div>
                    </div>
                  ))}
                </>
              )}
              {inactiveJoins.length > 0 && (
                <>
                  <h3>{t('modeling.inactive_joins_heading')}</h3>
                  {inactiveJoins.map((join) => (
                    <div className="modeling-join-pill modeling-join-pill--suggested" key={join.id}>
                      <div className="modeling-join-pill-header">
                        <strong>{join.name}</strong>
                        <button
                          className="modeling-add-btn"
                          onClick={() => onReactivateJoin(join)}
                          title={t('modeling.reactivate_title')}
                        >
                          +
                        </button>
                      </div>
                      <span>
                        {join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}
                      </span>
                    </div>
                  ))}
                </>
              )}
            </div>
          )}

          {activeTab === 'dimensions' && (
            <div className="modeling-join-list">
              <h3>{t('modeling.dimensions_tab')}</h3>
              {dims.length === 0 ? (
                <p className="modeling-empty">{t('modeling.no_dimensions')}</p>
              ) : (
                dims.map((dimension) => (
                  <div className="modeling-join-pill" key={dimension.id}>
                    <div className="modeling-join-pill-header">
                      <strong>{dimension.label || dimension.name}</strong>
                      <span className="modeling-pill-actions">
                        <button
                          className="modeling-rename-btn"
                          onClick={() => onRenameDimension(dimension)}
                          title={t('modeling.edit_display_name_title')}
                        >
                          ✎
                        </button>
                        <button
                          className="modeling-rename-btn"
                          onClick={() => onEditDimensionValues(dimension)}
                          title={t('modeling.enum_values_edit_title')}
                        >
                          ≣
                        </button>
                        <button
                          className="modeling-delete-btn"
                          onClick={() => onDeleteDimension(dimension.id)}
                          title={t('modeling.delete_dimension_title')}
                        >
                          ×
                        </button>
                      </span>
                    </div>
                    <span>{dimension.column_ref}</span>
                    <span className="modeling-join-meta">{dimension.type}</span>
                  </div>
                ))
              )}
              {inactiveDims.length > 0 && (
                <>
                  <h3>{t('modeling.inactive_dimensions_heading')}</h3>
                  {inactiveDims.map((dimension) => (
                    <div
                      className="modeling-join-pill modeling-join-pill--suggested"
                      key={dimension.id}
                    >
                      <div className="modeling-join-pill-header">
                        <strong>{dimension.label || dimension.name}</strong>
                        <button
                          className="modeling-add-btn"
                          onClick={() => onReactivateDimension(dimension)}
                          title={t('modeling.reactivate_title')}
                        >
                          +
                        </button>
                      </div>
                      <span>{dimension.column_ref}</span>
                    </div>
                  ))}
                </>
              )}
            </div>
          )}

          {activeTab === 'metrics' && (
            <div className="modeling-join-list">
              <div className="modeling-section-header" style={{ justifyContent: 'center' }}>
                <button
                  className="btn btn-sm btn-primary"
                  type="button"
                  onClick={onOpenAddMetric}
                  disabled={!model}
                  style={{ width: '100%' }}
                >
                  {t('modeling.add_metric_btn')}
                </button>
              </div>
              {metrics.length === 0 ? (
                <p className="modeling-empty">{t('modeling.no_metrics')}</p>
              ) : (
                metrics.map((metric) => (
                  <div className="modeling-join-pill" key={metric.id}>
                    <div className="modeling-join-pill-header">
                      <strong>{metric.label || metric.name}</strong>
                      <span className="modeling-pill-actions">
                        <button
                          className="modeling-rename-btn"
                          onClick={() => onRenameMetric(metric)}
                          title={t('modeling.edit_display_name_title')}
                        >
                          ✎
                        </button>
                        <button
                          className="modeling-delete-btn"
                          onClick={() => onDeleteMetric(metric.id)}
                          title={t('modeling.delete_metric_title')}
                        >
                          ×
                        </button>
                      </span>
                    </div>
                    <span>{metric.expression}</span>
                    <span className="modeling-join-meta">{metric.aggregation}</span>
                  </div>
                ))
              )}
              {inactiveMetrics.length > 0 && (
                <>
                  <h3>{t('modeling.inactive_metrics_heading')}</h3>
                  {inactiveMetrics.map((metric) => (
                    <div
                      className="modeling-join-pill modeling-join-pill--suggested"
                      key={metric.id}
                    >
                      <div className="modeling-join-pill-header">
                        <strong>{metric.label || metric.name}</strong>
                        <button
                          className="modeling-add-btn"
                          onClick={() => onReactivateMetric(metric)}
                          title={t('modeling.reactivate_title')}
                        >
                          +
                        </button>
                      </div>
                      <span>{metric.expression}</span>
                    </div>
                  ))}
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </aside>
  )
}
