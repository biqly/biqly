import { type ReactNode, useMemo, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import type {
  SemanticDimension,
  SemanticJoin,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import type { SuggestedJoin, Tab } from './types'
import { columnRefMatchesTable, tableKey } from './utils'

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
  onEditDimension: (dimension: SemanticDimension) => void
  onEditDimensionValues: (dimension: SemanticDimension) => void
  onDeleteDimension: (dimensionId: string) => void
  onReactivateDimension: (dimension: SemanticDimension) => void
  onSyncDimensions: () => void
  onOpenAddMetric: () => void
  onEditMetric: (metric: SemanticMetric) => void
  onDeleteMetric: (metricId: string) => void
  onReactivateMetric: (metric: SemanticMetric) => void
  t: Translate
}

interface CollapsibleGroupProps {
  title: string
  meta?: string
  count: number
  defaultOpen?: boolean
  children: ReactNode
}

// CollapsibleGroup renders one table's entities (dimensions/metrics) behind a
// click-to-expand header with a count badge, so large schemas stay scannable
// instead of an endless flat list.
function CollapsibleGroup({
  title,
  meta,
  count,
  defaultOpen = false,
  children,
}: CollapsibleGroupProps) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className={`modeling-group ${open ? 'modeling-group--open' : ''}`}>
      <button
        type="button"
        className="modeling-group-header"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <span className="modeling-group-chevron" aria-hidden="true">
          {open ? '▾' : '▸'}
        </span>
        <span className="modeling-group-title">{title}</span>
        {meta && <span className="modeling-group-meta">{meta}</span>}
        <span className="modeling-group-count">{count}</span>
      </button>
      {open && <div className="modeling-group-body">{children}</div>}
    </div>
  )
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
  onEditDimension,
  onEditDimensionValues,
  onDeleteDimension,
  onReactivateDimension,
  onSyncDimensions,
  onOpenAddMetric,
  onEditMetric,
  onDeleteMetric,
  onReactivateMetric,
  t,
}: ModelingPaletteProps) {
  const tableByKey = useMemo(() => {
    const out = new Map<string, TableRow>()
    includedTables.forEach((table) => {
      out.set(tableKey(table.schema_name, table.table_name), table)
    })
    return out
  }, [includedTables])

  const scopedTableKeys = useMemo(() => {
    const visible = new Set<string>(
      tableCards.map((table) => tableKey(table.schema_name, table.table_name)),
    )
    if (visible.size > 0) {
      return visible
    }
    return new Set(includedTables.map((table) => tableKey(table.schema_name, table.table_name)))
  }, [includedTables, tableCards])

  const resolveTableKeyFromRef = (
    ref: string | undefined | null,
    fallbackBaseSchema?: string,
  ): string | null => {
    for (const table of includedTables) {
      if (
        columnRefMatchesTable(
          ref,
          table.schema_name,
          table.table_name,
          fallbackBaseSchema ?? model?.base_schema ?? '',
        )
      ) {
        return tableKey(table.schema_name, table.table_name)
      }
    }
    return null
  }

  const resolveMetricTableKey = (metric: SemanticMetric): string | null => {
    const direct = resolveTableKeyFromRef(metric.expression, model?.base_schema)
    if (direct) {
      return direct
    }
    const expr = metric.expression
    const byFullRef = includedTables.find((table) =>
      expr.includes(`${table.schema_name}.${table.table_name}.`),
    )
    if (byFullRef) {
      return tableKey(byFullRef.schema_name, byFullRef.table_name)
    }
    const byBaseRef = includedTables.find(
      (table) => table.schema_name === model?.base_schema && expr.includes(`${table.table_name}.`),
    )
    if (byBaseRef) {
      return tableKey(byBaseRef.schema_name, byBaseRef.table_name)
    }
    return null
  }

  const tableDisplayLabel = (table: TableRow | undefined, fallback: string) => {
    const label = table?.label?.trim()
    if (label && label.length > 0) {
      return label
    }
    return table?.table_name ?? fallback
  }

  const groupByTable = <T,>(items: T[], resolveKey: (item: T) => string | null) => {
    const grouped = new Map<string, T[]>()
    items.forEach((item) => {
      const key = resolveKey(item)
      if (!key || !scopedTableKeys.has(key)) {
        return
      }
      const bucket = grouped.get(key)
      if (bucket) {
        bucket.push(item)
      } else {
        grouped.set(key, [item])
      }
    })
    return Array.from(grouped.entries())
      .map(([key, values]) => ({ key, values }))
      .sort((a, b) => {
        const aTable = tableByKey.get(a.key)
        const bTable = tableByKey.get(b.key)
        const aLabel = tableDisplayLabel(aTable, a.key).toLowerCase()
        const bLabel = tableDisplayLabel(bTable, b.key).toLowerCase()
        return aLabel.localeCompare(bLabel)
      })
  }

  const dimGroups = groupByTable(dims, (dimension) =>
    resolveTableKeyFromRef(dimension.column_ref, model?.base_schema),
  )
  const inactiveDimGroups = groupByTable(inactiveDims, (dimension) =>
    resolveTableKeyFromRef(dimension.column_ref, model?.base_schema),
  )
  const metricGroups = groupByTable(metrics, resolveMetricTableKey)
  const inactiveMetricGroups = groupByTable(inactiveMetrics, resolveMetricTableKey)

  const visibleDimsCount = dimGroups.reduce((sum, group) => sum + group.values.length, 0)
  const visibleMetricsCount = metricGroups.reduce((sum, group) => sum + group.values.length, 0)

  // Only suggest FK joins between tables already in the model/canvas. The
  // backend includes any FK with one endpoint in the model (to invite adding
  // related tables), but that surfaces tables the user hasn't selected — which
  // is confusing here.
  const visibleSuggestedJoins = suggestedJoins.filter(
    (join) =>
      scopedTableKeys.has(tableKey(join.from_schema, join.from_table)) &&
      scopedTableKeys.has(tableKey(join.to_schema, join.to_table)),
  )

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
          <h2>{model?.label ?? model?.name ?? t('modeling.no_model_selected')}</h2>
          <p>{t('modeling.semantic_description')}</p>
        </div>
        <div className="modeling-tabs">
          {(['tables', 'joins', 'dimensions', 'metrics'] as const).map((tab) => {
            const count =
              tab === 'tables'
                ? usedTableCount
                : tab === 'joins'
                  ? joins.length
                  : tab === 'dimensions'
                    ? visibleDimsCount
                    : visibleMetricsCount
            return (
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
                <span
                  className={`modeling-tab__count${count === 0 ? ' modeling-tab__count--zero' : ''}`}
                >
                  {count}
                </span>
              </button>
            )
          })}
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
                          {table.label ?? table.table_name}
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
              {visibleSuggestedJoins.length > 0 && (
                <>
                  <h3>{t('modeling.suggested_fk_relationships')}</h3>
                  {visibleSuggestedJoins.map((join, index) => (
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
              <div className="modeling-section-header">
                <h3>{t('modeling.dimensions_tab')}</h3>
                <button
                  className="btn btn-sm btn-primary modeling-section-add-btn"
                  type="button"
                  onClick={onSyncDimensions}
                  disabled={!model}
                  title={t('modeling.sync_dimensions_title')}
                >
                  {t('modeling.sync_dimensions_btn')}
                </button>
              </div>
              {visibleDimsCount === 0 ? (
                <p className="modeling-empty">{t('modeling.no_dimensions')}</p>
              ) : (
                dimGroups.map((group) => {
                  const table = tableByKey.get(group.key)
                  const tableLabel = tableDisplayLabel(table, group.key)
                  return (
                    <CollapsibleGroup
                      key={group.key}
                      title={tableLabel}
                      meta={table ? `${table.schema_name}.${table.table_name}` : undefined}
                      count={group.values.length}
                      defaultOpen={dimGroups.length === 1}
                    >
                      {group.values.map((dimension) => (
                        <div className="modeling-join-pill" key={dimension.id}>
                          <div className="modeling-join-pill-header">
                            <strong>{dimension.label ?? dimension.name}</strong>
                            <span className="modeling-pill-actions">
                              <button
                                className="modeling-rename-btn"
                                onClick={() => onEditDimension(dimension)}
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
                      ))}
                    </CollapsibleGroup>
                  )
                })
              )}
              {inactiveDimGroups.length > 0 && (
                <>
                  <h3>{t('modeling.inactive_dimensions_heading')}</h3>
                  {inactiveDimGroups.map((group) => (
                    <div className="modeling-group-body" key={group.key}>
                      {group.values.map((dimension) => (
                        <div
                          className="modeling-join-pill modeling-join-pill--suggested"
                          key={dimension.id}
                        >
                          <div className="modeling-join-pill-header">
                            <strong>{dimension.label ?? dimension.name}</strong>
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
                    </div>
                  ))}
                </>
              )}
            </div>
          )}

          {activeTab === 'metrics' && (
            <div className="modeling-join-list">
              <div className="modeling-section-header">
                <h3>{t('modeling.metrics_tab')}</h3>
                <button
                  className="btn btn-sm btn-primary modeling-section-add-btn"
                  type="button"
                  onClick={onOpenAddMetric}
                  disabled={!model}
                >
                  {t('modeling.add_metric_btn')}
                </button>
              </div>
              {visibleMetricsCount === 0 ? (
                <p className="modeling-empty">{t('modeling.no_metrics')}</p>
              ) : (
                metricGroups.map((group) => {
                  const table = tableByKey.get(group.key)
                  const tableLabel = tableDisplayLabel(table, group.key)
                  return (
                    <CollapsibleGroup
                      key={group.key}
                      title={tableLabel}
                      meta={table ? `${table.schema_name}.${table.table_name}` : undefined}
                      count={group.values.length}
                      defaultOpen={metricGroups.length === 1}
                    >
                      {group.values.map((metric) => (
                        <div className="modeling-join-pill" key={metric.id}>
                          <div className="modeling-join-pill-header">
                            <strong>{metric.label ?? metric.name}</strong>
                            <span className="modeling-pill-actions">
                              <button
                                className="modeling-rename-btn"
                                onClick={() => onEditMetric(metric)}
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
                      ))}
                    </CollapsibleGroup>
                  )
                })
              )}
              {inactiveMetricGroups.length > 0 && (
                <>
                  <h3>{t('modeling.inactive_metrics_heading')}</h3>
                  {inactiveMetricGroups.map((group) => (
                    <div className="modeling-group-body" key={group.key}>
                      {group.values.map((metric) => (
                        <div
                          className="modeling-join-pill modeling-join-pill--suggested"
                          key={metric.id}
                        >
                          <div className="modeling-join-pill-header">
                            <strong>{metric.label ?? metric.name}</strong>
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
