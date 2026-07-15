// @vitest-environment jsdom
//
// Review finding: fetchData must be called for EVERY non-text widget, even
// ones with NO widget.logical_query — sanitized public dashboard widgets
// (the entire point of the injectable fetchData prop) never carry
// logical_query, only the legacy in-app path requires it. The component's
// fetchData branch already runs before the `!widget.logical_query` guard
// that only gates the legacy path, but until this file there was zero test
// coverage of DashboardWidgetRenderer — a future refactor could silently
// reintroduce a logical_query-gated check on the fetchData branch and
// nothing would catch it.
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { QueryResultPayload } from '../../types/ai'
import type { DashboardWidget } from './DashboardWidgetRenderer'
import { DashboardWidgetRenderer } from './DashboardWidgetRenderer'

const mockPostData = vi.fn((): Promise<QueryResultPayload | null> => Promise.resolve(null))
// abort must be a stable reference across renders — DashboardWidgetRenderer
// lists it in its data-fetch effect's dependency array, so a fresh vi.fn()
// per useApi() call (i.e. per render) would re-fire the effect on every
// render and infinite-loop.
const mockAbort = vi.fn()

// DashboardWidgetRenderer calls useApi() internally for the legacy fetch
// path; stubbed here the same way AIQuery.test.tsx stubs it, so the legacy
// case can assert postData was invoked without a real network layer.
vi.mock('../../hooks/useApi', () => ({
  useApi: () => ({
    get: vi.fn(() => Promise.resolve(null)),
    postData: mockPostData,
    loading: false,
    error: null,
    abort: mockAbort,
  }),
}))

afterEach(() => {
  cleanup()
  mockPostData.mockClear()
  mockAbort.mockClear()
})

const samplePayload: QueryResultPayload = {
  columns: [{ name: 'value' }],
  rows: [[42]],
}

describe('DashboardWidgetRenderer fetchData branching', () => {
  // NOTE on saved_query_id: a *real* sanitized public-dashboard widget has
  // neither logical_query nor saved_query_id (confirmed against
  // internal/dashboard/public.go's SanitizeWidgets, which deletes both
  // keys). getWidgetDataStateElement's "Not configured yet" gate used to
  // fire whenever saved_query_id was absent, regardless of fetchData —
  // which meant a true sanitized widget could never display fetched data.
  // That has been fixed: the gate now also checks whether fetchData was
  // provided, so it only fires when BOTH saved_query_id and fetchData are
  // absent. Most cases below still carry a saved_query_id for historical
  // reasons, but the "true sanitized-public-widget shape" test further down
  // omits it entirely and asserts the fetched value renders — that's the
  // regression this fix closes.
  it('calls fetchData for a kpi widget with no logical_query, and renders its result', async () => {
    const widgetWithNoLogicalQuery: DashboardWidget = {
      id: 'w1',
      type: 'kpi',
      title: 'Revenue',
      w: 4,
      h: 'small',
      saved_query_id: 'sq-public-1',
    }
    const mockFetchData = vi.fn(() => Promise.resolve(samplePayload))

    render(<DashboardWidgetRenderer widget={widgetWithNoLogicalQuery} fetchData={mockFetchData} />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockFetchData).toHaveBeenCalledWith(widgetWithNoLogicalQuery)
    expect(screen.getByText('42')).toBeTruthy()
    // The legacy fetch path must not have fired.
    expect(mockPostData).not.toHaveBeenCalled()
  })

  it('calls fetchData for a table widget with no logical_query, and renders its result', async () => {
    const widgetWithNoLogicalQuery: DashboardWidget = {
      id: 'w2',
      type: 'table',
      title: 'Orders',
      w: 6,
      h: 'medium',
      saved_query_id: 'sq-public-2',
    }
    const mockFetchData = vi.fn(() => Promise.resolve(samplePayload))

    render(<DashboardWidgetRenderer widget={widgetWithNoLogicalQuery} fetchData={mockFetchData} />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockFetchData).toHaveBeenCalledWith(widgetWithNoLogicalQuery)
    expect(screen.getByText('42')).toBeTruthy()
    expect(mockPostData).not.toHaveBeenCalled()
  })

  it('renders fetched data for a true sanitized-public-widget shape (no logical_query, no saved_query_id) when fetchData is provided', async () => {
    // Regression guard: getWidgetDataStateElement's "Not configured yet" gate
    // used to fire unconditionally whenever saved_query_id was absent, even
    // when fetchData was supplied and had already resolved real data. Public
    // dashboard widgets NEVER carry saved_query_id (SanitizeWidgets strips
    // it), so that gate made every kpi/table widget on a shared dashboard
    // permanently show "Not configured yet" instead of its data. The fix
    // makes the gate require the absence of BOTH saved_query_id and
    // fetchData before showing that message.
    const trueSanitizedWidget: DashboardWidget = {
      id: 'w4',
      type: 'kpi',
      title: 'Revenue',
      w: 4,
      h: 'small',
    }
    const mockFetchData = vi.fn(() => Promise.resolve(samplePayload))

    render(<DashboardWidgetRenderer widget={trueSanitizedWidget} fetchData={mockFetchData} />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockFetchData).toHaveBeenCalledWith(trueSanitizedWidget)
    expect(mockPostData).not.toHaveBeenCalled()
    // The core regression assertion: the resolved value renders...
    expect(screen.getByText('42')).toBeTruthy()
    // ...instead of the "Not configured yet" message.
    expect(screen.queryByText(/Not configured yet/i)).toBeNull()
  })

  it('legacy behavior unchanged: a widget with no saved_query_id and no fetchData prop still shows "Not configured yet"', async () => {
    const unconfiguredWidget: DashboardWidget = {
      id: 'w5',
      type: 'kpi',
      title: 'Revenue',
      w: 4,
      h: 'small',
    }

    render(<DashboardWidgetRenderer widget={unconfiguredWidget} />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText(/Not configured yet/i)).toBeTruthy()
    expect(mockPostData).not.toHaveBeenCalled()
  })

  it('legacy behavior unchanged: with no fetchData prop and a widget that has logical_query, the legacy postData path fires instead', async () => {
    const legacyWidget: DashboardWidget = {
      id: 'w3',
      type: 'kpi',
      title: 'Revenue',
      w: 4,
      h: 'small',
      saved_query_id: 'sq-1',
      logical_query: { datasource_id: 'ds-1', model_id: 'm-1' },
    }
    mockPostData.mockResolvedValueOnce(samplePayload)

    render(<DashboardWidgetRenderer widget={legacyWidget} />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(mockPostData).toHaveBeenCalledWith('/api/query/run', legacyWidget.logical_query)
    expect(screen.getByText('42')).toBeTruthy()
  })
})
