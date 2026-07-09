// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'

const { getFunctionBlocklist, updateFunctionBlocklist } = vi.hoisted(() => ({
  getFunctionBlocklist: vi.fn(),
  updateFunctionBlocklist: vi.fn(),
}))

vi.mock('../../api/functionBlocklist', () => ({
  getFunctionBlocklist,
  updateFunctionBlocklist,
}))

vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useT: () => (key: string) => key,
}))

import { FunctionBlocklistPanel } from './FunctionBlocklistPanel'

const datasources = [{ id: 'warehouse', name: 'Warehouse', type: 'postgres' }]

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('FunctionBlocklistPanel', () => {
  it('loads defaults and custom functions for the selected datasource', async () => {
    getFunctionBlocklist.mockResolvedValue({
      defaults: ['pg_read_file'],
      custom: ['unsafe_function'],
    })
    render(<FunctionBlocklistPanel datasources={datasources} />)

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.datasource_label'), {
      target: { value: 'warehouse' },
    })

    expect(await screen.findByText('pg_read_file')).toBeTruthy()
    expect(screen.getByText('unsafe_function')).toBeTruthy()
    expect(getFunctionBlocklist).toHaveBeenCalledWith('warehouse')
  })

  it('adds normalized custom names and removes them before saving', async () => {
    getFunctionBlocklist.mockResolvedValue({ defaults: ['pg_read_file'], custom: [] })
    render(<FunctionBlocklistPanel datasources={datasources} />)

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.datasource_label'), {
      target: { value: 'warehouse' },
    })
    await screen.findByText('pg_read_file')

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.function_name_label'), {
      target: { value: 'Unsafe_Function' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'datasources.function_blocklist.add' }))

    expect(screen.getByText('unsafe_function')).toBeTruthy()
    fireEvent.click(
      screen.getByRole('button', { name: 'datasources.function_blocklist.remove_aria' }),
    )

    expect(screen.queryByText('unsafe_function')).toBeNull()
  })

  it('rejects names that are not simple SQL identifiers', async () => {
    getFunctionBlocklist.mockResolvedValue({ defaults: [], custom: [] })
    render(<FunctionBlocklistPanel datasources={datasources} />)

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.datasource_label'), {
      target: { value: 'warehouse' },
    })
    await screen.findByText('datasources.function_blocklist.defaults_empty')

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.function_name_label'), {
      target: { value: 'pg_read_file()' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'datasources.function_blocklist.add' }))

    expect(screen.getByRole('alert').textContent).toBe(
      'datasources.function_blocklist.invalid_identifier',
    )
  })

  it('saves the custom functions for the selected datasource', async () => {
    getFunctionBlocklist.mockResolvedValue({ defaults: ['pg_read_file'], custom: [] })
    updateFunctionBlocklist.mockResolvedValue({
      defaults: ['pg_read_file'],
      custom: ['unsafe_function'],
    })
    render(<FunctionBlocklistPanel datasources={datasources} />)

    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.datasource_label'), {
      target: { value: 'warehouse' },
    })
    await screen.findByText('pg_read_file')
    fireEvent.change(screen.getByLabelText('datasources.function_blocklist.function_name_label'), {
      target: { value: 'unsafe_function' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'datasources.function_blocklist.add' }))
    fireEvent.click(screen.getByRole('button', { name: 'datasources.function_blocklist.save' }))

    await waitFor(() => {
      expect(updateFunctionBlocklist).toHaveBeenCalledWith('warehouse', ['unsafe_function'])
    })
    expect(await screen.findByText('datasources.function_blocklist.saved')).toBeTruthy()
  })
})
