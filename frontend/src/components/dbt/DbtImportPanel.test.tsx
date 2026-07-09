// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type * as I18nModule from '../../i18n'

const { mockImportDbtProject, mockNavigate } = vi.hoisted(() => ({
  mockImportDbtProject: vi.fn(),
  mockNavigate: vi.fn(),
}))

vi.mock('../../api/dbt', () => ({ importDbtProject: mockImportDbtProject }))

vi.mock('../../hooks/useDatasources', () => ({
  useDatasources: () => ({
    datasources: [
      { id: 'warehouse', name: 'Warehouse', type: 'postgres' },
      { id: 'analytics', name: 'Analytics', type: 'bigquery' },
    ],
    loading: false,
  }),
}))

vi.mock('react-router-dom', () => ({ useNavigate: () => mockNavigate }))

vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useT: () => (key: string) => key,
}))

import { DbtImportPanel } from './DbtImportPanel'

afterEach(() => {
  cleanup()
  mockImportDbtProject.mockReset()
  mockNavigate.mockReset()
})

describe('DbtImportPanel', () => {
  it('previews enabled dbt models from the selected manifest', async () => {
    render(<DbtImportPanel />)

    const manifest = new File(
      [
        JSON.stringify({
          nodes: {
            'model.project.orders': {
              resource_type: 'model',
              name: 'orders',
              alias: 'fct_orders',
            },
            'model.project.disabled': {
              resource_type: 'model',
              name: 'disabled',
              config: { enabled: false },
            },
          },
        }),
      ],
      'manifest.json',
      { type: 'application/json' },
    )

    fireEvent.change(screen.getByLabelText('dbt_import.manifest_label'), {
      target: { files: [manifest] },
    })

    expect(await screen.findByText('fct_orders')).toBeTruthy()
    expect(screen.queryByText('disabled')).toBeNull()
  })

  it('uploads selected artifacts and displays created drafts with server warnings', async () => {
    mockImportDbtProject.mockResolvedValue({
      imported_models: [
        { model: { id: 'model-1', name: 'fct_orders' }, validation: { valid: true } },
      ],
      skipped: ['stg_legacy'],
      warnings: ['fct_orders: metadata is not synced'],
    })
    render(<DbtImportPanel />)

    fireEvent.change(screen.getByLabelText('dbt_import.datasource_label'), {
      target: { value: 'warehouse' },
    })
    fireEvent.change(screen.getByLabelText('dbt_import.manifest_label'), {
      target: {
        files: [new File(['{"nodes":{}}'], 'manifest.json', { type: 'application/json' })],
      },
    })
    fireEvent.change(screen.getByLabelText('dbt_import.catalog_label'), {
      target: { files: [new File(['{}'], 'catalog.json', { type: 'application/json' })] },
    })

    fireEvent.click(screen.getByRole('button', { name: 'dbt_import.import_action' }))

    await waitFor(() => {
      expect(mockImportDbtProject).toHaveBeenCalledWith(
        expect.objectContaining({ datasourceId: 'warehouse' }),
      )
    })
    expect(await screen.findByText('fct_orders')).toBeTruthy()
    expect(screen.getByText('stg_legacy')).toBeTruthy()
    expect(screen.getByText('fct_orders: metadata is not synced')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'dbt_import.open_modeling' }))
    expect(mockNavigate).toHaveBeenCalledWith('/modeling?datasource_id=warehouse')
  })
})
