import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import type { TranslationKey } from '../../i18n'
import type { TableRow } from '../../types/semantic'
import { ModelingTableCard } from './ModelingTableCard'
import type { CardLayout } from './types'

describe('ModelingTableCard', () => {
  it('renders type icons and all card sections', () => {
    const table = {
      id: 'public.orders',
      schema_name: 'public',
      table_name: 'orders',
    } as TableRow
    const layout: CardLayout = {
      columnsShown: [
        {
          id: 'amount',
          schema_name: 'public',
          table_name: 'orders',
          column_name: 'amount',
          data_type: 'numeric',
          nullable: false,
          description: null,
          is_primary_key: false,
          is_foreign_key: false,
          referenced_table: null,
          referenced_column: null,
        },
      ],
      columnIndex: new Map([['amount', 0]]),
      height: 200,
      visibleRowCount: 1,
      calcFieldCount: 2,
      relatedTables: ['users'],
    }
    const t = (key: TranslationKey) => key

    const render = (modelColumns: Set<string> | undefined) =>
      renderToStaticMarkup(
        <ModelingTableCard
          table={table}
          layout={layout}
          pos={{ x: 0, y: 0 }}
          isBase
          isHi={false}
          highlightedColumns={undefined}
          highlightedJoinColumns={null}
          modelColumns={modelColumns}
          onDragStart={vi.fn()}
          onKeyDown={vi.fn()}
          onOpenDetail={vi.fn()}
          onOpenColumnsMenu={vi.fn()}
          onColumnsScroll={vi.fn()}
          onRelRowsMeasure={vi.fn()}
          onAddCalcField={vi.fn()}
          onAddRelationship={vi.fn()}
          t={t}
        />,
      )

    const markup = render(new Set(['amount']))

    expect(markup).toContain('123')
    expect(markup).toContain('modeling.calc_fields_section')
    expect(markup).toContain('modeling.relationships_section')
    expect(markup).toContain('users')
    // Column list scrolls inside a fixed window instead of truncating.
    expect(markup).toContain('modeling-card-columns')

    // A column without a backing model dimension renders dimmed; a model field
    // (or an unknown membership when no model is loaded) does not.
    expect(markup).not.toContain('opacity-45')
    expect(render(new Set())).toContain('opacity-45')
    expect(render(undefined)).not.toContain('opacity-45')
  })
})
