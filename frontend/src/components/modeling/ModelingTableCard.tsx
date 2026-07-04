import type { KeyboardEvent, MouseEvent } from 'react'

import type { TranslationKey } from '../../i18n'
import {
  modelingCardHeaderRowClass,
  modelingCardSectionAddClass,
  modelingCardSectionClass,
  modelingColumnNameClass,
  modelingKebabClass,
  modelingRelRowClass,
  modelingTableCardClass,
  modelingTableRowClass,
  modelingTypeIconClass,
} from '../../lib/modelingClasses'
import type { TableRow } from '../../types/semantic'
import { columnTypeIcon } from './columnTypeIcon'
import { CARD_WIDTH } from './constants'
import type { CardLayout } from './types'
import { formatDataType } from './utils'

interface ModelingTableCardProps {
  table: TableRow
  layout: CardLayout
  pos: { x: number; y: number }
  isBase: boolean
  isHi: boolean
  highlightedColumns: Set<string> | undefined
  highlightedJoinColumns: { from: string; to: string } | null
  onDragStart: (event: MouseEvent) => void
  onKeyDown: (event: KeyboardEvent) => void
  onOpenDetail: () => void
  onAddCalcField: () => void
  onAddRelationship: () => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}

export function ModelingTableCard({
  table,
  layout,
  pos,
  isBase,
  isHi,
  highlightedColumns,
  highlightedJoinColumns,
  onDragStart,
  onKeyDown,
  onOpenDetail,
  onAddCalcField,
  onAddRelationship,
  t,
}: ModelingTableCardProps) {
  const key = `${table.schema_name}.${table.table_name}`

  return (
    <article
      className={modelingTableCardClass({ base: isBase, hi: isHi })}
      style={{ left: pos.x, top: pos.y, width: CARD_WIDTH, height: layout.height }}
    >
      <header
        role="button"
        tabIndex={0}
        aria-label={t('modeling.table_card_aria', { name: key })}
        onMouseDown={onDragStart}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault()
            onOpenDetail()
            return
          }
          onKeyDown(event)
        }}
      >
        <div className={modelingCardHeaderRowClass}>
          <strong>{table.table_name}</strong>
        </div>
        <span>{table.schema_name}</span>
      </header>
      <button
        type="button"
        className={modelingKebabClass}
        aria-label={t('modeling.table_card_menu', { name: key })}
        onMouseDown={(event) => event.stopPropagation()}
        onClick={onOpenDetail}
      >
        ⋮
      </button>

      <ul>
        {layout.columnsShown.map((column) => {
          const isJoinCol = highlightedColumns?.has(column.column_name)
          const colKey = `${key}::${column.column_name}`
          const isActiveJoinCol =
            highlightedJoinColumns !== null &&
            (highlightedJoinColumns.from === colKey || highlightedJoinColumns.to === colKey)
          const icon = columnTypeIcon(column.data_type)
          return (
            <li
              key={column.id}
              className={modelingTableRowClass({ joined: isJoinCol, active: isActiveJoinCol })}
            >
              <span className={modelingColumnNameClass}>
                <span className={modelingTypeIconClass} aria-hidden="true">
                  {icon.kind === 'timestamp' ? (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 12 12"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="1.3"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="size-[0.65rem]"
                    >
                      <circle cx="6" cy="6" r="5" />
                      <polyline points="6,2.5 6,6 8.5,7.5" />
                    </svg>
                  ) : (
                    icon.glyph
                  )}
                </span>
                {column.is_primary_key && <b>{t('modeling.pk_badge')}</b>}
                {column.is_foreign_key && <b>{t('modeling.fk_badge')}</b>}
                {column.column_name}
              </span>
              <small>{formatDataType(t, column.data_type)}</small>
            </li>
          )
        })}
        {layout.hiddenCount > 0 && (
          <li className={modelingTableRowClass({ more: true })}>
            +{layout.hiddenCount} {t('modeling.more_columns')}
          </li>
        )}
      </ul>

      <div className={modelingCardSectionClass}>
        <span>
          {t('modeling.calc_fields_section')} ({layout.calcFieldCount})
        </span>
        <button
          type="button"
          className={modelingCardSectionAddClass}
          aria-label={t('modeling.add_calc_field')}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={onAddCalcField}
        >
          ＋
        </button>
      </div>

      <div className={modelingCardSectionClass}>
        <span>
          {t('modeling.relationships_section')} ({layout.relatedTables.length})
        </span>
        <button
          type="button"
          className={modelingCardSectionAddClass}
          aria-label={t('modeling.add_relationship')}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={onAddRelationship}
        >
          ＋
        </button>
      </div>
      {layout.relatedTables.map((relatedTable) => (
        <div key={relatedTable} className={modelingRelRowClass}>
          <span aria-hidden="true">⇄</span>
          <span className="overflow-hidden text-ellipsis whitespace-nowrap">{relatedTable}</span>
        </div>
      ))}
    </article>
  )
}
