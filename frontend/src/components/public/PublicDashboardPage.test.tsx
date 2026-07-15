// @vitest-environment jsdom
import { act, cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { PublicDashboard } from '../../api/publicDashboard'

const { mockGetPublicDashboard, mockRunPublicWidget } = vi.hoisted(() => ({
  mockGetPublicDashboard: vi.fn(),
  mockRunPublicWidget: vi.fn(),
}))

vi.mock('../../api/publicDashboard', () => ({
  getPublicDashboard: mockGetPublicDashboard,
  runPublicWidget: mockRunPublicWidget,
}))

vi.mock('../../i18n/hooks', () => ({
  useT: () => (key: string) => key,
}))

vi.mock('react-router-dom', () => ({
  useParams: () => ({ token: 'tok-123' }),
}))

import PublicDashboardPage from './PublicDashboardPage'

afterEach(() => {
  cleanup()
  mockGetPublicDashboard.mockReset()
  mockRunPublicWidget.mockReset()
})

const sampleDashboard: PublicDashboard = {
  id: 'dash-1',
  name: 'Q3 Revenue Overview',
  description: 'Shared read-only view',
  widgets: [
    {
      id: 'w1',
      type: 'kpi',
      title: 'Total Revenue',
      w: 4,
      h: 'small',
    },
  ],
}

describe('PublicDashboardPage', () => {
  it('renders the loading state while the dashboard is being fetched', () => {
    mockGetPublicDashboard.mockReturnValue(new Promise(() => undefined))

    render(<PublicDashboardPage />)

    expect(screen.getByText('publicDashboard.loading')).toBeTruthy()
  })

  it('renders the dashboard name and widget title on success, with no app-shell landmarks', async () => {
    mockGetPublicDashboard.mockResolvedValue(sampleDashboard)
    mockRunPublicWidget.mockResolvedValue({ columns: [{ name: 'value' }], rows: [[42]] })

    const { container } = render(<PublicDashboardPage />)
    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(screen.getByText('Q3 Revenue Overview')).toBeTruthy()
    expect(screen.getByText('Total Revenue')).toBeTruthy()

    // Shell-less: no sidebar/nav landmarks from the authenticated app shell.
    expect(container.querySelector('aside')).toBeNull()
    expect(container.querySelector('#primary-sidebar')).toBeNull()
    expect(screen.queryByRole('complementary')).toBeNull()
    expect(screen.queryByRole('navigation')).toBeNull()
  })

  it('renders the not-found state when the fetch is rejected (404 token)', async () => {
    mockGetPublicDashboard.mockRejectedValue(new Error('not found'))

    render(<PublicDashboardPage />)
    await act(async () => {
      await Promise.resolve()
    })

    expect(screen.getByText('publicDashboard.not_found_title')).toBeTruthy()
    expect(screen.getByText('publicDashboard.not_found_desc')).toBeTruthy()
  })
})
