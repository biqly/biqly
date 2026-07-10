import { type ReactNode, useMemo, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import {
  modelingAddBtnClass,
  modelingBaseBadgeClass,
  modelingDeleteBtnClass,
  modelingEmptyClass,
  modelingGroupBodyClass,
  modelingGroupChevronClass,
  modelingGroupClass,
  modelingGroupCountClass,
  modelingGroupHeaderClass,
  modelingGroupMetaClass,
  modelingGroupTitleClass,
  modelingJoinListClass,
  modelingJoinMetaClass,
  modelingJoinPillClass,
  modelingJoinPillHeaderClass,
  modelingKickerClass,
  modelingPaletteClass,
  modelingPaletteSideBodyClass,
  modelingPillActionsClass,
  modelingRailLabelClass,
  modelingRenameBtnClass,
  modelingSchemaTagClass,
  modelingSchemaTagListClass,
  modelingSchemaTagNameClass,
  modelingSchemaTagToggleClass,
  modelingSectionAddBtnClass,
  modelingSectionHeaderClass,
  modelingSideToggleClass,
  modelingTabClass,
  modelingTabContentClass,
  modelingTabCountClass,
  modelingTabsClass,
} from '../../lib/modelingClasses'
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

// dimTypeGlyph maps a dimension's semantic type to the tiny leading tag shown
// in the model tree (WrenAI-style field markers).
function dimTypeGlyph(type: string): string {
  switch (type) {
    case 'number':
      return '123'
    case 'date':
    case 'timestamp':
      return '🕒'
    case 'boolean':
      return '✓'
    default:
      return 'Az'
  }
}

// ModelTreeNode is one table in the merged model tree: an expandable header
// carrying the table-level actions, with the table's dimensions and metrics
// as child rows.
function ModelTreeNode({
  title,
  meta,
  count,
  isBase,
  baseBadgeTitle,
  actions,
  defaultOpen = false,
  children,
}: {
  title: string
  meta?: string
  count: number
  isBase: boolean
  baseBadgeTitle: string
  actions: ReactNode
  defaultOpen?: boolean
  children: ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className={modelingGroupClass(open)}>
      <div className={cn(modelingGroupHeaderClass, 'cursor-default')}>
        <button
          type="button"
          className="font-inherit flex min-w-0 flex-1 cursor-pointer items-center gap-[0.4rem] border-0 bg-transparent p-0 text-left text-inherit"
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
        >
          <span className={modelingGroupChevronClass} aria-hidden="true">
            {open ? '▾' : '▸'}
          </span>
          <span className={modelingGroupTitleClass}>
            {title}
            {isBase && (
              <span className={modelingBaseBadgeClass} title={baseBadgeTitle}>
                {' '}
                ★
              </span>
            )}
          </span>
          {meta && <span className={modelingGroupMetaClass}>{meta}</span>}
          <span className={modelingGroupCountClass}>{count}</span>
        </button>
        <span className={modelingPillActionsClass}>{actions}</span>
      </div>
      {open && <div className={modelingGroupBodyClass}>{children}</div>}
    </div>
  )
}

// TreeLeafRow is one field (dimension or metric) under a table node: a slim
// single-line row with a type glyph and hover-revealed actions.
function TreeLeafRow({
  glyph,
  title,
  subtitle,
  actions,
}: {
  glyph: string
  title: string
  subtitle?: string
  actions: ReactNode
}) {
  return (
    <div className="group/leaf flex items-center gap-1.5 rounded-md px-1.5 py-1 hover:bg-[var(--surface-hover)]">
      <span
        className="text-foreground-faint w-6 shrink-0 text-center text-[0.58rem] font-bold"
        aria-hidden="true"
      >
        {glyph}
      </span>
      <span className="min-w-0 flex-1">
        <span className="text-foreground block overflow-hidden text-[0.78rem] leading-tight text-ellipsis whitespace-nowrap">
          {title}
        </span>
        {subtitle && (
          <span className="text-foreground-faint block overflow-hidden text-[0.65rem] text-ellipsis whitespace-nowrap">
            {subtitle}
          </span>
        )}
      </span>
      <span
        className={cn(
          modelingPillActionsClass,
          'opacity-0 group-focus-within/leaf:opacity-100 group-hover/leaf:opacity-100',
        )}
      >
        {actions}
      </span>
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
      if (!key || !tableByKey.has(key)) {
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

  const dimsByTable = new Map(dimGroups.map((group) => [group.key, group.values]))
  const metricsByTable = new Map(metricGroups.map((group) => [group.key, group.values]))

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
    <aside className={modelingPaletteClass(open)} aria-label={t('modeling.model_summary_aria')}>
      <button
        type="button"
        className={modelingSideToggleClass('left')}
        onClick={onToggle}
        title={open ? t('modeling.collapse_panel') : t('modeling.expand_panel')}
      >
        {open ? '‹' : '›'}
      </button>
      {!open && (
        <button type="button" className={modelingRailLabelClass} onClick={onToggle}>
          {t('modeling.semantic_layer')}
        </button>
      )}
      <div className={modelingPaletteSideBodyClass}>
        <div>
          <span className={modelingKickerClass}>{t('modeling.semantic_layer')}</span>
          <h2>{model?.label ?? model?.name ?? t('modeling.no_model_selected')}</h2>
          <p>{t('modeling.semantic_description')}</p>
        </div>
        <div className={modelingTabsClass}>
          {(['model', 'joins'] as const).map((tab) => {
            const count = tab === 'model' ? usedTableCount : joins.length
            return (
              <button
                className={modelingTabClass(activeTab === tab)}
                key={tab}
                onClick={() => onTabChange(tab)}
                title={t(tab === 'model' ? 'modeling.model_tab_title' : 'modeling.joins_tab')}
              >
                {t(tab === 'model' ? 'modeling.tab_short_model' : 'modeling.tab_short_rel')}
                <span className={modelingTabCountClass(count === 0, activeTab === tab)}>
                  {count}
                </span>
              </button>
            )
          })}
        </div>

        <div className={modelingTabContentClass}>
          {activeTab === 'model' && (
            <div className={modelingJoinListClass}>
              <h3>{t('modeling.schemas_heading')}</h3>
              <div className={modelingSchemaTagListClass}>
                {Array.from(new Set(tables.map((table) => table.schema_name)))
                  .sort()
                  .map((schemaName) => {
                    const isExcluded = excludedSchemas.has(schemaName)
                    return (
                      <div
                        className={modelingSchemaTagClass(!isExcluded)}
                        key={`schema-${schemaName}`}
                      >
                        <span className={modelingSchemaTagNameClass} title={schemaName}>
                          {schemaName}
                        </span>
                        <button
                          type="button"
                          className={modelingSchemaTagToggleClass}
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
                    )
                  })}
              </div>

              <div className={modelingSectionHeaderClass}>
                <h3>{t('modeling.model_tree_heading')}</h3>
                <span className="flex gap-1.5">
                  <button
                    className={cn(buttonClass('ghost', { size: 'sm' }), modelingSectionAddBtnClass)}
                    type="button"
                    onClick={onSyncDimensions}
                    disabled={!model}
                    title={t('modeling.sync_dimensions_title')}
                  >
                    {t('modeling.sync_dimensions_btn')}
                  </button>
                  <button
                    className={cn(
                      buttonClass('primary', { size: 'sm' }),
                      modelingSectionAddBtnClass,
                    )}
                    type="button"
                    onClick={onOpenAddMetric}
                    disabled={!model}
                  >
                    {t('modeling.add_metric_btn')}
                  </button>
                </span>
              </div>
              {tables.length === 0 ? (
                <p className={modelingEmptyClass}>{t('modeling.no_tables_sync')}</p>
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
                  const tableDims = dimsByTable.get(key) ?? []
                  const tableMetrics = metricsByTable.get(key) ?? []
                  return (
                    <ModelTreeNode
                      key={table.id}
                      title={table.label ?? table.table_name}
                      meta={isOnCanvas ? t('modeling.on_canvas') : t('modeling.not_visible')}
                      count={tableDims.length + tableMetrics.length}
                      isBase={isBase}
                      baseBadgeTitle={t('modeling.base_table_label')}
                      defaultOpen={isBase}
                      actions={
                        <>
                          <button
                            className={modelingRenameBtnClass}
                            onClick={() => onRenameTable(table)}
                            title={t('modeling.edit_display_name_title')}
                          >
                            ✎
                          </button>
                          {!isBase && inModel && (
                            <button
                              className={modelingRenameBtnClass}
                              onClick={() => onMakeBase(table.schema_name, table.table_name)}
                              title={t('modeling.make_base_title')}
                            >
                              ★
                            </button>
                          )}
                          {isOnCanvas && !isBase && (
                            <button
                              className={modelingDeleteBtnClass}
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
                              className={modelingAddBtnClass}
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
                              className={modelingDeleteBtnClass}
                              onClick={onOpenBaseSwap}
                              title={t('modeling.change_base_title')}
                            >
                              ×
                            </button>
                          )}
                        </>
                      }
                    >
                      {tableDims.length === 0 && tableMetrics.length === 0 ? (
                        <p className={modelingEmptyClass}>{t('modeling.tree_no_fields')}</p>
                      ) : (
                        <>
                          {tableDims.map((dimension) => (
                            <TreeLeafRow
                              key={dimension.id}
                              glyph={dimTypeGlyph(dimension.type)}
                              title={dimension.label ?? dimension.name}
                              subtitle={dimension.column_ref}
                              actions={
                                <>
                                  <button
                                    className={modelingRenameBtnClass}
                                    onClick={() => onEditDimension(dimension)}
                                    title={t('modeling.edit_display_name_title')}
                                  >
                                    ✎
                                  </button>
                                  <button
                                    className={modelingRenameBtnClass}
                                    onClick={() => onEditDimensionValues(dimension)}
                                    title={t('modeling.enum_values_edit_title')}
                                  >
                                    ≣
                                  </button>
                                  <button
                                    className={modelingDeleteBtnClass}
                                    onClick={() => onDeleteDimension(dimension.id)}
                                    title={t('modeling.delete_dimension_title')}
                                  >
                                    ×
                                  </button>
                                </>
                              }
                            />
                          ))}
                          {tableMetrics.map((metric) => (
                            <TreeLeafRow
                              key={metric.id}
                              glyph="ƒ"
                              title={metric.label ?? metric.name}
                              subtitle={`${metric.aggregation}(${metric.expression})`}
                              actions={
                                <>
                                  <button
                                    className={modelingRenameBtnClass}
                                    onClick={() => onEditMetric(metric)}
                                    title={t('modeling.edit_display_name_title')}
                                  >
                                    ✎
                                  </button>
                                  <button
                                    className={modelingDeleteBtnClass}
                                    onClick={() => onDeleteMetric(metric.id)}
                                    title={t('modeling.delete_metric_title')}
                                  >
                                    ×
                                  </button>
                                </>
                              }
                            />
                          ))}
                        </>
                      )}
                    </ModelTreeNode>
                  )
                })
              )}
              {(inactiveDimGroups.length > 0 || inactiveMetricGroups.length > 0) && (
                <>
                  <h3>{t('modeling.inactive_fields_heading')}</h3>
                  {inactiveDimGroups.map((group) => (
                    <div className={modelingGroupBodyClass} key={`dim-${group.key}`}>
                      {group.values.map((dimension) => (
                        <div
                          className={modelingJoinPillClass({ suggested: true })}
                          key={dimension.id}
                        >
                          <div className={modelingJoinPillHeaderClass}>
                            <strong>{dimension.label ?? dimension.name}</strong>
                            <button
                              className={modelingAddBtnClass}
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
                  {inactiveMetricGroups.map((group) => (
                    <div className={modelingGroupBodyClass} key={`metric-${group.key}`}>
                      {group.values.map((metric) => (
                        <div className={modelingJoinPillClass({ suggested: true })} key={metric.id}>
                          <div className={modelingJoinPillHeaderClass}>
                            <strong>{metric.label ?? metric.name}</strong>
                            <button
                              className={modelingAddBtnClass}
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

          {activeTab === 'joins' && (
            <div className={modelingJoinListClass}>
              <h3>{t('modeling.active_relationships')}</h3>
              {joins.length === 0 ? (
                <p className={modelingEmptyClass}>{t('modeling.no_relationships')}</p>
              ) : (
                joins.map((join) => (
                  <div
                    className={modelingJoinPillClass({ active: highlightJoinId === join.id })}
                    key={join.id}
                    role="button"
                    tabIndex={0}
                    aria-pressed={highlightJoinId === join.id}
                    onClick={() => onHighlightJoin(highlightJoinId === join.id ? null : join.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        onHighlightJoin(highlightJoinId === join.id ? null : join.id)
                      }
                    }}
                  >
                    <div className={modelingJoinPillHeaderClass}>
                      <strong>{join.name}</strong>
                      <button
                        className={modelingDeleteBtnClass}
                        onClick={() => onDeleteJoin(join.id)}
                        title={t('modeling.delete_relationship_title')}
                      >
                        ×
                      </button>
                    </div>
                    <span>
                      {join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}
                    </span>
                    <span className={modelingJoinMetaClass}>
                      {join.join_type} · {join.relationship}
                    </span>
                  </div>
                ))
              )}
              {visibleSuggestedJoins.length > 0 && (
                <>
                  <h3>{t('modeling.suggested_fk_relationships')}</h3>
                  {visibleSuggestedJoins.map((join, index) => (
                    <div className={modelingJoinPillClass({ suggested: true })} key={index}>
                      <div className={modelingJoinPillHeaderClass}>
                        <strong>
                          {join.from_table}.{join.from_column} → {join.to_table}.{join.to_column}
                        </strong>
                        <button
                          className={modelingAddBtnClass}
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
                    <div className={modelingJoinPillClass({ suggested: true })} key={join.id}>
                      <div className={modelingJoinPillHeaderClass}>
                        <strong>{join.name}</strong>
                        <button
                          className={modelingAddBtnClass}
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
        </div>
      </div>
    </aside>
  )
}
