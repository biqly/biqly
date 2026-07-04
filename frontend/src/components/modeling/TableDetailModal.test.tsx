import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { I18nProvider, type TranslationKey } from '../../i18n'
import type { ColumnRow, SemanticModelDetail, TableRow } from '../../types/semantic'
import { TableDetailModal } from './TableDetailModal'

const table = {
  schema_name: 'public',
  table_name: 'orders',
  id: 'public.orders',
} as TableRow
const columns: ColumnRow[] = [
  {
    id: 'c1',
    schema_name: 'public',
    table_name: 'orders',
    column_name: 'amount',
    data_type: 'numeric',
    nullable: false,
    description: 'Order amount',
    is_primary_key: false,
    is_foreign_key: false,
    referenced_table: null,
    referenced_column: null,
  },
]
const model = {
  base_schema: 'public',
  base_table: 'orders',
  joins: [],
} as unknown as SemanticModelDetail
const t = (key: TranslationKey) => key

function renderModal(open: boolean) {
  return renderToStaticMarkup(
    <I18nProvider>
      <TableDetailModal
        open={open}
        table={table}
        model={model}
        columns={columns}
        datasourceId="ds1"
        postData={vi.fn().mockResolvedValue(null)}
        onClose={vi.fn()}
        onEdit={vi.fn()}
        t={t}
      />
    </I18nProvider>,
  )
}

describe('TableDetailModal', () => {
  it('renders the table name and its columns when open', () => {
    const markup = renderModal(true)
    expect(markup).toContain('orders')
    expect(markup).toContain('amount')
    expect(markup.match(/w-auto/g)).toHaveLength(2)
  })

  it('renders nothing when closed', () => {
    expect(renderModal(false)).toBe('')
  })
})
