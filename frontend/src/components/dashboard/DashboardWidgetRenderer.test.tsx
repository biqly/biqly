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
  // keys). But DashboardWidgetRenderer's own getWidgetDataStateElement has a
  // separate, pre-existing gate — `if (!savedQueryId) return "Not
  // configured yet..."` — that fires unconditionally, before it even looks
  // at loading/data, so a widget missing saved_query_id can never display
  // fetched data regardless of fetchData. That gate is orthogonal to (and
  // predates) the fetchData/logical_query guard this suite exists to pin
  // down, and this task is test-only (DashboardWidgetRenderer.tsx must not
  // be touched), so these widgets carry a saved_query_id purely to get past
  // that unrelated gate and observe the fetched value render. The
  // logical_query-gated regression this suite actually defends against —
  // fetchData still being called although widget.logical_query is absent —
  // is unaffected by that choice.
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

  it('calls fetchData even when the widget has neither logical_query nor saved_query_id (true sanitized-widget shape), though the result stays gated behind the unrelated "Not configured" message', async () => {
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

    // The core regression guard: fetchData still fires despite no
    // logical_query, regardless of what getWidgetDataStateElement then does
    // with saved_query_id.
    expect(mockFetchData).toHaveBeenCalledWith(trueSanitizedWidget)
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
