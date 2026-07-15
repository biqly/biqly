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
  modelingPaletteSideBodyClass,
  modelingPillActionsClass,
  modelingRenameBtnClass,
  modelingSchemaTagClass,
  modelingSchemaTagListClass,
  modelingSchemaTagNameClass,
  modelingSchemaTagToggleClass,
  modelingSectionHeaderClass,
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
  onOpenAddDimension: () => void
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
  dimmed = false,
  children,
}: {
  title: string
  meta?: string
  count: number
  isBase: boolean
  baseBadgeTitle: string
  actions: ReactNode
  defaultOpen?: boolean
  /** Passive look for tables that are not part of the model/canvas. */
  dimmed?: boolean
  children: ReactNode
}) {
  const [open, setOpen] = useState(defaultOpen)
  return (
    <div className={cn(modelingGroupClass(open), dimmed && 'opacity-65 hover:opacity-100')}>
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
      {open && (
        <div
          className={cn(modelingGroupBodyClass, 'border-border/70 ml-[0.55rem] border-l pl-1.5')}
        >
          {children}
        </div>
      )}
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
  inactive = false,
}: {
  glyph: string
  title: string
  subtitle?: string
  actions: ReactNode
  /** Soft-deactivated field: dimmed, with a reactivate action. */
  inactive?: boolean
}) {
  return (
    <div
      className={cn(
        'group/leaf flex items-center gap-1.5 rounded-md px-1.5 py-1 hover:bg-(--surface-hover)',
        inactive && 'opacity-60 hover:opacity-100',
      )}
    >
      <span
        className="text-foreground-faint w-6 shrink-0 text-center text-[0.58rem] font-bold"
        aria-hidden="true"
      >
        {glyph}
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            'block overflow-hidden text-[0.78rem] leading-tight text-ellipsis whitespace-nowrap',
            inactive ? 'text-foreground-muted' : 'text-foreground',
          )}
        >
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
  onOpenAddDimension,
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
  // Inactive (soft-deactivated) fields live under their own table node in the
  // tree; only fields whose table is not in the palette fall back to the
  // flat section at the bottom.
  const inactiveDimsByTable = new Map(inactiveDimGroups.map((group) => [group.key, group.values]))
  const inactiveMetricsByTable = new Map(
    inactiveMetricGroups.map((group) => [group.key, group.values]),
  )
  const includedTableKeys = new Set(
    includedTables.map((table) => tableKey(table.schema_name, table.table_name)),
  )
  const orphanInactiveDimGroups = inactiveDimGroups.filter(
    (group) => !includedTableKeys.has(group.key),
  )
  const orphanInactiveMetricGroups = inactiveMetricGroups.filter(
    (group) => !includedTableKeys.has(group.key),
  )

  // Only suggest FK joins between tables already in the model/canvas. The
  // backend includes any FK with one endpoint in the model (to invite adding
  // related tables), but that surfaces tables the user hasn't selected — which
  // is confusing here.
  const visibleSuggestedJoins = suggestedJoins.filter(
    (join) =>
      scopedTableKeys.has(tableKey(join.from_schema, join.from_table)) &&
      scopedTableKeys.has(tableKey(join.to_schema, join.to_table)),
  )

  const canvasTableKeys = new Set(
    tableCards.map((card) => tableKey(card.schema_name, card.table_name)),
  )
  const activeModelTables = includedTables.filter((table) =>
    canvasTableKeys.has(tableKey(table.schema_name, table.table_name)),
  )
  const passiveModelTables = includedTables.filter(
    (table) => !canvasTableKeys.has(tableKey(table.schema_name, table.table_name)),
  )

  // renderTableActions renders the table-level action buttons (rename, make
  // base, remove/hide, show, change base) for one tree node.
  const renderTableActions = (
    table: (typeof includedTables)[number],
    flags: { isBase: boolean; isOnCanvas: boolean; inModel: boolean },
  ) => (
    <>
      <button
        className={modelingRenameBtnClass}
        onClick={() => onRenameTable(table)}
        title={t('modeling.edit_display_name_title')}
      >
        ✎
      </button>
      {!flags.isBase && flags.inModel && (
        <button
          className={modelingRenameBtnClass}
          onClick={() => onMakeBase(table.schema_name, table.table_name)}
          title={t('modeling.make_base_title')}
        >
          ★
        </button>
      )}
      {flags.isOnCanvas && !flags.isBase && (
        <button
          className={modelingDeleteBtnClass}
          onClick={() => onRemoveTable(table.schema_name, table.table_name)}
          title={
            flags.inModel
              ? t('modeling.remove_from_model_title')
              : t('modeling.hide_from_canvas_title')
          }
        >
          ×
        </button>
      )}
      {!flags.isOnCanvas && (
        <button
          className={modelingAddBtnClass}
          onClick={() => onToggleTableVisibility(table.schema_name, table.table_name, true)}
          title={t('modeling.show_on_canvas_title')}
        >
          +
        </button>
      )}
      {flags.isBase && (
        <button
          className={modelingDeleteBtnClass}
          onClick={onOpenBaseSwap}
          title={t('modeling.change_base_title')}
        >
          ×
        </button>
      )}
    </>
  )

  // renderModelTableNode renders one table node of the model tree; used by
  // both the active (on-canvas) and passive groups below.
  const renderModelTableNode = (table: (typeof includedTables)[number]) => {
    const key = tableKey(table.schema_name, table.table_name)
    const isOnCanvas = canvasTableKeys.has(key)
    const isBase = model ? key === tableKey(model.base_schema, model.base_table) : false
    const impact = model
      ? tableImpact(table.schema_name, table.table_name)
      : { joins: 0, dims: 0, metrics: 0 }
    const inModel = isBase || impact.joins > 0 || impact.dims > 0 || impact.metrics > 0
    const tableDims = dimsByTable.get(key) ?? []
    const tableMetrics = metricsByTable.get(key) ?? []
    const tableInactiveDims = inactiveDimsByTable.get(key) ?? []
    const tableInactiveMetrics = inactiveMetricsByTable.get(key) ?? []
    return (
      <ModelTreeNode
        key={table.id}
        title={table.label ?? table.table_name}
        meta={isOnCanvas ? t('modeling.on_canvas') : t('modeling.not_visible')}
        count={tableDims.length + tableMetrics.length}
        isBase={isBase}
        baseBadgeTitle={t('modeling.base_table_label')}
        defaultOpen={isBase}
        dimmed={!isOnCanvas}
        actions={renderTableActions(table, { isBase, isOnCanvas, inModel })}
      >
        {tableDims.length === 0 &&
        tableMetrics.length === 0 &&
        tableInactiveDims.length === 0 &&
        tableInactiveMetrics.length === 0 ? (
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
            {tableInactiveDims.map((dimension) => (
              <TreeLeafRow
                key={`inactive-${dimension.id}`}
                glyph={dimTypeGlyph(dimension.type)}
                title={dimension.label ?? dimension.name}
                subtitle={dimension.column_ref}
                inactive
                actions={
                  <button
                    className={modelingAddBtnClass}
                    onClick={() => onReactivateDimension(dimension)}
                    title={t('modeling.reactivate_title')}
                  >
                    +
                  </button>
                }
              />
            ))}
            {tableInactiveMetrics.map((metric) => (
              <TreeLeafRow
                key={`inactive-${metric.id}`}
                glyph="ƒ"
                title={metric.label ?? metric.name}
                subtitle={`${metric.aggregation}(${metric.expression})`}
                inactive
                actions={
                  <button
                    className={modelingAddBtnClass}
                    onClick={() => onReactivateMetric(metric)}
                    title={t('modeling.reactivate_title')}
                  >
                    +
                  </button>
                }
              />
            ))}
          </>
        )}
      </ModelTreeNode>
    )
  }

  return (
    <div role="region" aria-label={t('modeling.model_summary_aria')}>
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

              {/* Heading on its own row; the two actions share a full-width
                  row below so neither the label nor the buttons ever wrap
                  awkwardly in the narrow palette. */}
              <div className={modelingSectionHeaderClass}>
                <h3>{t('modeling.model_tree_heading')}</h3>
              </div>
              <div className="mb-2 grid grid-cols-2 gap-1.5">
                <button
                  className={cn(buttonClass('secondary', { size: 'sm' }), 'col-span-2 w-full')}
                  type="button"
                  onClick={onSyncDimensions}
                  disabled={!model}
                  title={t('modeling.sync_dimensions_title')}
                >
                  {t('modeling.sync_dimensions_btn')}
                </button>
                <button
                  className={cn(buttonClass('primary', { size: 'sm' }), 'w-full')}
                  type="button"
                  onClick={onOpenAddMetric}
                  disabled={!model}
                >
                  {t('modeling.add_metric_btn')}
                </button>
                <button
                  className={cn(buttonClass('primary', { size: 'sm' }), 'w-full')}
                  type="button"
                  onClick={onOpenAddDimension}
                  disabled={!model}
                >
                  {t('modeling.add_dimension_btn')}
                </button>
              </div>
              {tables.length === 0 ? (
                <p className={modelingEmptyClass}>{t('modeling.no_tables_sync')}</p>
              ) : (
                <>
                  <h3>{t('modeling.active_tables_heading')}</h3>
                  {activeModelTables.length === 0 ? (
                    <p className={modelingEmptyClass}>{t('modeling.no_active_tables')}</p>
                  ) : (
                    activeModelTables.map(renderModelTableNode)
                  )}
                  {passiveModelTables.length > 0 && (
                    <>
                      <h3>{t('modeling.inactive_tables_heading')}</h3>
                      {passiveModelTables.map(renderModelTableNode)}
                    </>
                  )}
                </>
              )}
              {(orphanInactiveDimGroups.length > 0 || orphanInactiveMetricGroups.length > 0) && (
                <>
                  <h3>{t('modeling.inactive_fields_heading')}</h3>
                  {orphanInactiveDimGroups.map((group) => (
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
                  {orphanInactiveMetricGroups.map((group) => (
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
    </div>
  )
}
