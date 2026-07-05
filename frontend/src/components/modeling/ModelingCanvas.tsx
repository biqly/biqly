import { useState } from 'react'

import type { TranslationKey } from '../../i18n'
import {
  modelingCanvasClass,
  modelingCanvasWrapClass,
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
import type { SemanticJoin, TableRow } from '../../types/semantic'
import { ModelingTableCard } from './ModelingTableCard'
import type { ModelingCanvasState } from './useModelingCanvas'
import { relationshipLabel, tableKey } from './utils'

interface JoinHoverState {
  join: SemanticJoin
  x: number
  y: number
}
interface ModelingCanvasProps {
  canvas: ModelingCanvasState
  tableCards: TableRow[]
  joins: SemanticJoin[]
  baseKey: string | null
  highlightJoinId: string | null
  highlightedTables: Set<string> | null
  highlightedColumns: Map<string, Set<string>>
  highlightedJoinColumns: { from: string; to: string } | null
  onOpenTableDetail: (table: TableRow) => void
  onAddCalcField: (table: TableRow) => void
  onAddRelationship: (table: TableRow) => void
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
  } = canvas

  const [joinHover, setJoinHover] = useState<JoinHoverState | null>(null)

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
              return (
                <g key={join.id} className={modelingJoinLineClass(isHi)}>
                  <path
                    d={path.d}
                    className={modelingJoinHitClass}
                    strokeWidth={14}
                    onMouseEnter={(e) => setJoinHover({ join, x: e.clientX, y: e.clientY })}
                    onMouseMove={(e) => setJoinHover({ join, x: e.clientX, y: e.clientY })}
                    onMouseLeave={() => setJoinHover(null)}
                  />
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
                onDragStart={onCardDragStart(key)}
                onKeyDown={onCardKeyDown(key)}
                onOpenDetail={() => onOpenTableDetail(table)}
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
        </div>
      ) : null}
    </>
  )
}
