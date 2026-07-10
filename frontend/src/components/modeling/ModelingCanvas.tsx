import { useEffect, useRef, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import {
  modelingCanvasClass,
  modelingCanvasWrapClass,
  modelingColumnsMenuClass,
  modelingColumnsMenuItemClass,
  modelingColumnsMenuListClass,
  modelingColumnsMenuTitleClass,
  modelingJoinHitClass,
  modelingJoinLineClass,
  modelingJoinTooltipClass,
  modelingJoinTooltipLabelClass,
  modelingJoinTooltipRowClass,
  modelingJoinTooltipTitleClass,
  modelingJoinTooltipValueClass,
  modelingLinesClass,
  modelingZoomControlsClass,
  modelingZoomReadoutClass,
} from '../../lib/modelingClasses'
import type { ColumnRow, SemanticJoin, TableRow } from '../../types/semantic'
import { cardinalityMarkers } from './canvasMath'
import { ModelingTableCard } from './ModelingTableCard'
import type { ModelingCanvasState } from './useModelingCanvas'
import { columnOptions, relationshipLabel, tableKey } from './utils'

interface JoinHoverState {
  join: SemanticJoin
  x: number
  y: number
}
interface ColumnsMenuState {
  key: string
  anchor: DOMRect
}
interface ModelingCanvasProps {
  canvas: ModelingCanvasState
  tableCards: TableRow[]
  columns: ColumnRow[]
  joins: SemanticJoin[]
  baseKey: string | null
  highlightJoinId: string | null
  highlightedTables: Set<string> | null
  highlightedColumns: Map<string, Set<string>>
  highlightedJoinColumns: { from: string; to: string } | null
  // Column names that back an active model dimension, per table key; null when
  // no model is loaded (membership cannot be shown or edited).
  modelColumnsByTable: Map<string, Set<string>> | null
  // `${tableKey}::${column}` keys with an in-flight dimension toggle request.
  pendingColumnKeys: Set<string>
  onToggleColumnDimension: (table: TableRow, columnName: string) => void
  onDeleteJoin: (joinId: string) => void
  onOpenTableDetail: (table: TableRow) => void
  onAddCalcField: (table: TableRow) => void
  onAddRelationship: (table: TableRow) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

// Per-table model-field picker: a checked column is backed by an active
// dimension in the semantic model, so toggles here edit the model itself and
// stay in sync with the palette's dimensions tab.
function ModelFieldsMenu({
  tableKey: key,
  columns,
  checked,
  pendingColumnKeys,
  anchor,
  onToggle,
  onClose,
  t,
}: {
  tableKey: string
  columns: ColumnRow[]
  checked: Set<string>
  pendingColumnKeys: Set<string>
  anchor: DOMRect
  onToggle: (columnName: string) => void
  onClose: () => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    const onPointerDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onPointerDown)
    }
  }, [onClose])

  const idBase = `col-vis-${key.replace(/[^a-zA-Z0-9_-]+/g, '-')}`
  return (
    <div
      ref={ref}
      className={modelingColumnsMenuClass}
      style={{ left: anchor.right, top: anchor.bottom + 4, transform: 'translateX(-100%)' }}
      role="group"
      aria-label={t('modeling.columns_menu_aria', { name: key })}
    >
      <div className={modelingColumnsMenuTitleClass}>{t('modeling.columns_menu_title')}</div>
      <div className={modelingColumnsMenuListClass}>
        {columns.map((column) => {
          const id = `${idBase}-${column.id}`
          return (
            <label key={column.id} htmlFor={id} className={modelingColumnsMenuItemClass}>
              <input
                id={id}
                type="checkbox"
                checked={checked.has(column.column_name)}
                disabled={pendingColumnKeys.has(`${key}::${column.column_name}`)}
                onChange={() => onToggle(column.column_name)}
              />
              <span>{column.column_name}</span>
            </label>
          )
        })}
      </div>
    </div>
  )
}

// JoinActionPopover is the interactive twin of the hover tooltip: opened by
// clicking a join line, it shows the relationship facts (incl. the optional
// AI description) and lets the user delete the join right on the graph.
function JoinActionPopover({
  join,
  x,
  y,
  onDelete,
  onClose,
  t,
}: {
  join: SemanticJoin
  x: number
  y: number
  onDelete: () => void
  onClose: () => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
      }
    }
    const onPointerDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onPointerDown)
    }
  }, [onClose])

  return (
    <div
      ref={ref}
      className={cn(
        modelingJoinTooltipClass,
        'pointer-events-auto max-w-88 gap-1.5 px-4 py-3 text-[0.8rem]',
      )}
      style={{ left: x + 12, top: y + 12 }}
      role="dialog"
      aria-label={t('modeling.join_tooltip_title')}
    >
      <span className={modelingJoinTooltipTitleClass}>{t('modeling.join_tooltip_title')}</span>
      <span className={modelingJoinTooltipRowClass}>
        <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_from')}</span>
        <span className={modelingJoinTooltipValueClass}>
          {join.from_table}.{join.from_column}
        </span>
      </span>
      <span className={modelingJoinTooltipRowClass}>
        <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_to')}</span>
        <span className={modelingJoinTooltipValueClass}>
          {join.to_table}.{join.to_column}
        </span>
      </span>
      <span className={modelingJoinTooltipRowClass}>
        <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_type')}</span>
        <span className={modelingJoinTooltipValueClass}>
          {relationshipLabel(t, join.relationship)}
        </span>
      </span>
      {join.description?.trim() ? (
        <span className={modelingJoinTooltipRowClass}>
          <span className={modelingJoinTooltipLabelClass}>
            {t('modeling.join_tooltip_description')}
          </span>
          <span className={modelingJoinTooltipValueClass}>{join.description}</span>
        </span>
      ) : (
        <span className="text-foreground-faint text-[0.72rem] italic">
          {t('modeling.join_no_description')}
        </span>
      )}
      <button
        type="button"
        className={cn(
          buttonClass('ghost', { size: 'sm' }),
          'text-error mt-1.5 w-full! justify-center',
        )}
        onClick={onDelete}
      >
        🗑 {t('modeling.delete_join_title')}
      </button>
    </div>
  )
}

export function ModelingCanvas({
  canvas,
  tableCards,
  columns,
  joins,
  baseKey,
  highlightJoinId,
  highlightedTables,
  highlightedColumns,
  highlightedJoinColumns,
  modelColumnsByTable,
  pendingColumnKeys,
  onToggleColumnDimension,
  onDeleteJoin,
  onOpenTableDetail,
  onAddCalcField,
  onAddRelationship,
  t,
}: ModelingCanvasProps) {
  const {
    wrapRef,
    viewport,
    canvasBounds,
    positions,
    cardLayouts,
    onCanvasMouseDown,
    onCardDragStart,
    onCardKeyDown,
    zoomBy,
    fitView,
    resetView,
    getJoinPath,
    setCardScrollTop,
  } = canvas

  const [joinHover, setJoinHover] = useState<JoinHoverState | null>(null)
  // Clicking a join line opens an interactive popover (info + delete) at the
  // click point — the hover tooltip is read-only, this one takes actions.
  const [joinMenu, setJoinMenu] = useState<JoinHoverState | null>(null)
  const [columnsMenu, setColumnsMenu] = useState<ColumnsMenuState | null>(null)

  const emptyModelColumns = new Set<string>()
  const menuTable = columnsMenu
    ? tableCards.find((table) => tableKey(table.schema_name, table.table_name) === columnsMenu.key)
    : undefined

  return (
    <>
      <div className={modelingCanvasWrapClass} ref={wrapRef} onMouseDown={onCanvasMouseDown}>
        <div className={modelingZoomControlsClass} onMouseDown={(e) => e.stopPropagation()}>
          <button type="button" onClick={() => zoomBy(1)} title={t('modeling.zoom_in')}>
            +
          </button>
          <button type="button" onClick={() => zoomBy(-1)} title={t('modeling.zoom_out')}>
            −
          </button>
          <button type="button" onClick={fitView} title={t('modeling.fit_view')}>
            ⤢
          </button>
          <button type="button" onClick={resetView} title={t('modeling.reset_view')}>
            1:1
          </button>
          <span className={modelingZoomReadoutClass}>{Math.round(viewport.scale * 100)}%</span>
        </div>
        <div
          className={modelingCanvasClass}
          style={{
            width: canvasBounds.width,
            height: canvasBounds.height,
            transform: `translate3d(${viewport.tx}px, ${viewport.ty}px, 0) scale(${viewport.scale})`,
            transformOrigin: '0 0',
          }}
        >
          <svg
            className={modelingLinesClass}
            width={canvasBounds.width}
            height={canvasBounds.height}
            aria-hidden="true"
          >
            <defs>
              <marker
                id="modeling-arrow"
                viewBox="0 0 10 10"
                refX="8"
                refY="5"
                markerWidth="6"
                markerHeight="6"
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" />
              </marker>
            </defs>
            {joins.map((join) => {
              const isHi = highlightJoinId === join.id
              if (highlightJoinId && !isHi) {
                return null
              }
              const path = getJoinPath(join)
              if (!path) {
                return null
              }
              const cardinality = cardinalityMarkers(join.relationship)
              return (
                <g key={join.id} className={modelingJoinLineClass(isHi)}>
                  <path
                    d={path.d}
                    className={modelingJoinHitClass}
                    strokeWidth={14}
                    onMouseEnter={(e) => setJoinHover({ join, x: e.clientX, y: e.clientY })}
                    onMouseMove={(e) => setJoinHover({ join, x: e.clientX, y: e.clientY })}
                    onMouseLeave={() => setJoinHover(null)}
                    onMouseDown={(e) => e.stopPropagation()}
                    onClick={(e) => {
                      e.stopPropagation()
                      setJoinHover(null)
                      setJoinMenu({ join, x: e.clientX, y: e.clientY })
                    }}
                  />
                  <path d={path.d} markerEnd="url(#modeling-arrow)" />
                  <circle cx={path.x1} cy={path.y1} r={4} />
                  <circle cx={path.x2} cy={path.y2} r={4} />
                  <text x={path.x1} y={path.y1 - 7} textAnchor="middle">
                    {cardinality.from}
                  </text>
                  <text x={path.x2} y={path.y2 - 7} textAnchor="middle">
                    {cardinality.to}
                  </text>
                </g>
              )
            })}
          </svg>
          {tableCards.map((table) => {
            const key = tableKey(table.schema_name, table.table_name)
            const pos = positions[key] ?? { x: 0, y: 0 }
            const layout = cardLayouts.get(key)
            if (!layout) {
              return null
            }
            const isHi = highlightedTables?.has(key) ?? false
            if (highlightedTables && !isHi) {
              return null
            }
            return (
              <ModelingTableCard
                key={key}
                table={table}
                layout={layout}
                pos={pos}
                isBase={baseKey === key}
                isHi={isHi}
                highlightedColumns={highlightedColumns.get(key)}
                highlightedJoinColumns={highlightedJoinColumns}
                modelColumns={
                  modelColumnsByTable
                    ? (modelColumnsByTable.get(key) ?? emptyModelColumns)
                    : undefined
                }
                onDragStart={onCardDragStart(key)}
                onKeyDown={onCardKeyDown(key)}
                onOpenDetail={() => onOpenTableDetail(table)}
                onOpenColumnsMenu={(anchor) => setColumnsMenu({ key, anchor })}
                onColumnsScroll={(top) => setCardScrollTop(key, top)}
                onAddCalcField={() => onAddCalcField(table)}
                onAddRelationship={() => onAddRelationship(table)}
                t={t}
              />
            )
          })}
        </div>
      </div>
      {joinHover ? (
        <div
          className={modelingJoinTooltipClass}
          style={{ left: joinHover.x + 12, top: joinHover.y + 12 }}
          aria-hidden="true"
        >
          <span className={modelingJoinTooltipTitleClass}>{t('modeling.join_tooltip_title')}</span>
          <span className={modelingJoinTooltipRowClass}>
            <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_from')}</span>
            <span className={modelingJoinTooltipValueClass}>
              {joinHover.join.from_table}.{joinHover.join.from_column}
            </span>
          </span>
          <span className={modelingJoinTooltipRowClass}>
            <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_to')}</span>
            <span className={modelingJoinTooltipValueClass}>
              {joinHover.join.to_table}.{joinHover.join.to_column}
            </span>
          </span>
          <span className={modelingJoinTooltipRowClass}>
            <span className={modelingJoinTooltipLabelClass}>{t('modeling.join_tooltip_type')}</span>
            <span className={modelingJoinTooltipValueClass}>
              {relationshipLabel(t, joinHover.join.relationship)}
            </span>
          </span>
          {joinHover.join.description?.trim() ? (
            <span className={modelingJoinTooltipRowClass}>
              <span className={modelingJoinTooltipLabelClass}>
                {t('modeling.join_tooltip_description')}
              </span>
              <span className={modelingJoinTooltipValueClass}>{joinHover.join.description}</span>
            </span>
          ) : null}
        </div>
      ) : null}
      {joinMenu ? (
        <JoinActionPopover
          join={joinMenu.join}
          x={joinMenu.x}
          y={joinMenu.y}
          onDelete={() => {
            onDeleteJoin(joinMenu.join.id)
            setJoinMenu(null)
          }}
          onClose={() => setJoinMenu(null)}
          t={t}
        />
      ) : null}
      {columnsMenu && modelColumnsByTable && menuTable ? (
        <ModelFieldsMenu
          tableKey={columnsMenu.key}
          columns={columnOptions(columns, columnsMenu.key)}
          checked={modelColumnsByTable.get(columnsMenu.key) ?? emptyModelColumns}
          pendingColumnKeys={pendingColumnKeys}
          anchor={columnsMenu.anchor}
          onToggle={(columnName) => onToggleColumnDimension(menuTable, columnName)}
          onClose={() => setColumnsMenu(null)}
          t={t}
        />
      ) : null}
    </>
  )
}
