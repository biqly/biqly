// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '../../api/apiClient'
import type { CreatedPublicShare, PublicShareStatus } from '../../api/dashboardShare'
import type * as I18nModule from '../../i18n'

const { mockGetStatus, mockCreate, mockRevoke } = vi.hoisted(() => ({
  mockGetStatus: vi.fn(),
  mockCreate: vi.fn(),
  mockRevoke: vi.fn(),
}))

vi.mock('../../api/dashboardShare', () => ({
  getDashboardPublicShare: mockGetStatus,
  createDashboardPublicShare: mockCreate,
  revokeDashboardPublicShare: mockRevoke,
}))

vi.mock('../../i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof I18nModule>()),
  useT: () => (key: string) => key,
  useLocale: () => ['en', () => undefined],
}))

import { PublicShareModal } from './PublicShareModal'

afterEach(() => {
  cleanup()
  mockGetStatus.mockReset()
  mockCreate.mockReset()
  mockRevoke.mockReset()
})

const inactiveStatus: PublicShareStatus = { active: false }
const activeStatus: PublicShareStatus = { active: true, created_at: '2026-01-01T00:00:00Z' }
const createdShare: CreatedPublicShare = {
  token: 'tok-abc123',
  url_path: '/public/dashboard/tok-abc123',
  created_at: '2026-01-02T00:00:00Z',
}

async function flush() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

describe('PublicShareModal', () => {
  it('shows the enable button when no share exists yet', async () => {
    mockGetStatus.mockResolvedValue(inactiveStatus)

    render(<PublicShareModal dashboardId="dash-1" open={true} onClose={() => undefined} />)
    await flush()

    expect(screen.getByText('publicShare.enable')).toBeTruthy()
    expect(screen.queryByText('publicShare.rotate')).toBeNull()
    expect(screen.queryByText('publicShare.revoke')).toBeNull()
  })

  it('shows the link and an iframe snippet after creating a share', async () => {
    mockGetStatus.mockResolvedValue(inactiveStatus)
    mockCreate.mockResolvedValue(createdShare)

    render(<PublicShareModal dashboardId="dash-1" open={true} onClose={() => undefined} />)
    await flush()

    fireEvent.click(screen.getByText('publicShare.enable'))
    await flush()

    const expectedUrl = `${window.location.origin}${createdShare.url_path}`
    expect(screen.getByDisplayValue(expectedUrl)).toBeTruthy()
    expect(
      screen.getByText((text) => text.includes('/public/dashboard/') && text.includes('<iframe')),
    ).toBeTruthy()
  })

  it('shows the disabled_by_admin message on a 409 from create', async () => {
    mockGetStatus.mockResolvedValue(inactiveStatus)
    mockCreate.mockRejectedValue(new ApiError('public sharing is disabled for this workspace', 409))

    render(<PublicShareModal dashboardId="dash-1" open={true} onClose={() => undefined} />)
    await flush()

    fireEvent.click(screen.getByText('publicShare.enable'))
    await flush()

    expect(screen.getByText('publicShare.disabled_by_admin')).toBeTruthy()
    expect(screen.queryByDisplayValue(/public\/dashboard/)).toBeNull()
  })

  it('shows rotate and revoke (not enable) when a share is already active', async () => {
    mockGetStatus.mockResolvedValue(activeStatus)

    render(<PublicShareModal dashboardId="dash-1" open={true} onClose={() => undefined} />)
    await flush()

    expect(screen.getByText('publicShare.rotate')).toBeTruthy()
    expect(screen.getByText('publicShare.revoke')).toBeTruthy()
    expect(screen.queryByText('publicShare.enable')).toBeNull()
    // The plaintext token is never returned by a mere status check, so no
    // link/iframe should be visible for a pre-existing active share.
    expect(screen.queryByDisplayValue(/public\/dashboard/)).toBeNull()
  })
})
