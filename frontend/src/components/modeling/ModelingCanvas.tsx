import type { TranslationKey } from '../../i18n'
import {
  modelingCanvasClass,
  modelingCanvasWrapClass,
  modelingColumnNameClass,
  modelingJoinLineClass,
  modelingLinesClass,
  modelingTableCardClass,
  modelingTableRowClass,
  modelingZoomControlsClass,
  modelingZoomReadoutClass,
} from '../../lib/modelingClasses'
import type { SemanticJoin, TableRow } from '../../types/semantic'
import { CARD_WIDTH } from './constants'
import type { ModelingCanvasState } from './useModelingCanvas'
import { formatDataType, tableKey } from './utils'
interface ModelingCanvasProps {
  canvas: ModelingCanvasState
  tableCards: TableRow[]
  joins: SemanticJoin[]
  baseKey: string | null
  highlightJoinId: string | null
  highlightedTables: Set<string> | null
  highlightedColumns: Map<string, Set<string>>
  highlightedJoinColumns: { from: string; to: string } | null
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

export function ModelingCanvas({
  canvas,
  tableCards,
  joins,
  baseKey,
  highlightJoinId,
  highlightedTables,
  highlightedColumns,
  highlightedJoinColumns,
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
  } = canvas

  return (
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
            return (
              <g key={join.id} className={modelingJoinLineClass(isHi)}>
                <path d={path.d} markerEnd="url(#modeling-arrow)" />
                <circle cx={path.x1} cy={path.y1} r={4} />
                <circle cx={path.x2} cy={path.y2} r={4} />
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
          const isBase = baseKey === key
          const isHi = highlightedTables?.has(key) ?? false
          if (highlightedTables && !isHi) {
            return null
          }
          const hiCols = highlightedColumns.get(key)
          const hiddenCount = layout.hiddenCount
          return (
            <article
              className={modelingTableCardClass({ base: isBase, hi: isHi })}
              key={key}
              style={{ left: pos.x, top: pos.y, width: CARD_WIDTH, height: layout.height }}
            >
              <header
                role="button"
                tabIndex={0}
                aria-label={t('modeling.table_card_aria', {
                  name: `${table.schema_name}.${table.table_name}`,
                })}
                onMouseDown={onCardDragStart(key)}
                onKeyDown={onCardKeyDown(key)}
              >
                <span>{table.schema_name}</span>
                <strong>{table.table_name}</strong>
              </header>
              <ul>
                {layout.columnsShown.map((column) => {
                  const isJoinCol = hiCols?.has(column.column_name)
                  const colKey = `${key}::${column.column_name}`
                  const isActiveJoinCol =
                    !!highlightedJoinColumns &&
                    (highlightedJoinColumns.from === colKey || highlightedJoinColumns.to === colKey)
                  return (
                    <li
                      key={column.id}
                      className={modelingTableRowClass({
                        joined: isJoinCol,
                        active: isActiveJoinCol,
                      })}
                    >
                      <span className={modelingColumnNameClass}>
                        {column.is_primary_key && <b>{t('modeling.pk_badge')}</b>}
                        {column.is_foreign_key && <b>{t('modeling.fk_badge')}</b>}
                        {column.column_name}
                      </span>
                      <small>{formatDataType(t, column.data_type)}</small>
                    </li>
                  )
                })}
                {hiddenCount > 0 && (
                  <li className={modelingTableRowClass({ more: true })}>
                    +{hiddenCount} {t('modeling.more_columns')}
                  </li>
                )}
              </ul>
            </article>
          )
        })}
      </div>
    </div>
  )
}
