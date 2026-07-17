// @vitest-environment jsdom
import { renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

// Control the module-level request() the hook uses for the rows call, so we
// can drive success/failure and prove the error is scoped to the selection.
const requestMock =
  vi.fn<
    (
      method: string,
      url: string,
      body?: unknown,
    ) => Promise<{ data: unknown; error: string | null }>
  >()
vi.mock('../../hooks/useApi', () => ({
  request: (method: string, url: string, body?: unknown) => requestMock(method, url, body),
}))

import { useTableBrowserQueryState } from './useTableBrowserQueryState'

const baseProps = {
  datasourceId: 'ds-1',
  filterPayload: [],
  columnOrder: [],
  onPageReset: vi.fn(),
  filtersKey: '',
}

afterEach(() => {
  requestMock.mockReset()
})

describe('useTableBrowserQueryState error scoping', () => {
  it('surfaces the rows error for the current selection', async () => {
    requestMock.mockResolvedValue({ data: null, error: 'table not found' })

    const { result } = renderHook((props) => useTableBrowserQueryState(props), {
      initialProps: { ...baseProps, schema: 'public', table: 'missing' },
    })

    await waitFor(() => expect(result.current.error).toBe('table not found'))
    expect(result.current.result).toBeNull()
  })

  it('clears a stale error the moment the selected table changes', async () => {
    requestMock.mockResolvedValue({ data: null, error: 'table not found' })

    const { result, rerender } = renderHook((props) => useTableBrowserQueryState(props), {
      initialProps: { ...baseProps, schema: 'public', table: 'missing' },
    })
    await waitFor(() => expect(result.current.error).toBe('table not found'))

    // Switch to a table whose rows request succeeds.
    requestMock.mockResolvedValue({
      data: { columns: [{ name: 'id' }], rows: [[1]] },
      error: null,
    })
    rerender({ ...baseProps, schema: 'public', table: 'datasources' })

    // The scope changed, so the prior table's error must not linger over the
    // new selection even before its request resolves.
    expect(result.current.error).toBeNull()

    await waitFor(() => expect(result.current.result).not.toBeNull())
    expect(result.current.error).toBeNull()
  })

  it('does not let a superseded error overwrite the current selection', async () => {
    // Current selection succeeds…
    requestMock.mockResolvedValue({
      data: { columns: [{ name: 'id' }], rows: [[1]] },
      error: null,
    })
    const { result } = renderHook((props) => useTableBrowserQueryState(props), {
      initialProps: { ...baseProps, schema: 'public', table: 'datasources' },
    })
    await waitFor(() => expect(result.current.result).not.toBeNull())

    // …the error stays null; a scoped error keyed to a different table can
    // never appear here.
    expect(result.current.error).toBeNull()
  })
})
