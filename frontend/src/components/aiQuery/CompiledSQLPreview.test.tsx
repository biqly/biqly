// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'
import { CompiledSQLPreview } from './CompiledSQLPreview'
import { tokenizeSQL } from './compiledSQLTokens'

const { compileQuery, dryRunQuery } = vi.hoisted(() => ({
  compileQuery: vi.fn(),
  dryRunQuery: vi.fn(),
}))

vi.mock('../../api/query', () => ({ compileQuery, dryRunQuery }))
vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useT: () => (key: string) => translations[key] ?? key,
}))

const logicalQuery = { datasource_id: 'ds-1', model_id: 'model-1', select: [] }
const translations: Record<string, string> = {
  'ai_query.sql_preview_code_aria': 'SQL preview code',
  'ai_query.sql_preview_compile_failed': 'Could not compile SQL.',
  'ai_query.sql_preview_validate_failed': 'Could not validate SQL.',
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('CompiledSQLPreview', () => {
  it('classifies SQL keywords for syntax highlighting', () => {
    expect(tokenizeSQL('SELECT total FROM orders')).toEqual([
      { value: 'SELECT', kind: 'keyword' },
      { value: ' total ', kind: 'text' },
      { value: 'FROM', kind: 'keyword' },
      { value: ' orders', kind: 'text' },
    ])
  })

  it('loads and displays compiled SQL when its preview is expanded', async () => {
    compileQuery.mockResolvedValue({ sql: 'SELECT total FROM orders', args: [] })
    render(<CompiledSQLPreview logicalQuery={logicalQuery} />)

    fireEvent.click(screen.getByRole('button', { name: 'ai_query.sql_preview_title' }))

    expect((await screen.findByLabelText('SQL preview code')).textContent).toBe(
      'SELECT total FROM orders',
    )
    expect(compileQuery).toHaveBeenCalledWith(logicalQuery)
  })

  it('validates the logical query without executing it and announces success', async () => {
    dryRunQuery.mockResolvedValue({
      sql: 'SELECT total FROM orders',
      args: [],
      fingerprint: 'fp-1',
    })
    render(<CompiledSQLPreview logicalQuery={logicalQuery} initialSQL="SELECT total FROM orders" />)

    fireEvent.click(screen.getByRole('button', { name: 'ai_query.sql_preview_title' }))
    fireEvent.click(screen.getByRole('button', { name: 'ai_query.sql_preview_validate' }))

    expect(await screen.findByText('ai_query.sql_preview_valid')).toBeTruthy()
    expect(dryRunQuery).toHaveBeenCalledWith(logicalQuery)
  })

  it('shows the translated compile failure instead of API error detail', async () => {
    compileQuery.mockRejectedValue(new Error('upstream connection detail'))
    render(<CompiledSQLPreview logicalQuery={logicalQuery} />)

    fireEvent.click(screen.getByRole('button', { name: 'ai_query.sql_preview_title' }))

    expect(await screen.findByText('Could not compile SQL.')).toBeTruthy()
    expect(screen.queryByText('upstream connection detail')).toBeNull()
  })

  it('uses a translated accessible label for highlighted SQL', async () => {
    compileQuery.mockResolvedValue({ sql: 'SELECT 1', args: [] })
    render(<CompiledSQLPreview logicalQuery={logicalQuery} />)

    fireEvent.click(screen.getByRole('button', { name: 'ai_query.sql_preview_title' }))

    expect(await screen.findByLabelText('SQL preview code')).toBeTruthy()
  })

  it('assigns a unique controlled region to each preview', () => {
    render(
      <>
        <CompiledSQLPreview logicalQuery={logicalQuery} initialSQL="SELECT 1" />
        <CompiledSQLPreview logicalQuery={logicalQuery} initialSQL="SELECT 2" />
      </>,
    )

    const toggles = screen.getAllByRole('button', { name: 'ai_query.sql_preview_title' })
    const [firstToggle, secondToggle] = toggles
    if (!firstToggle || !secondToggle) {
      throw new Error('expected two SQL preview toggles')
    }
    expect(firstToggle.getAttribute('aria-controls')).not.toBe(
      secondToggle.getAttribute('aria-controls'),
    )
  })
})
