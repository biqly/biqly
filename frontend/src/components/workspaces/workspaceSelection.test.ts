import { describe, expect, it } from 'vitest'

import type { Workspace } from '../../types/auth'
import { resolveActiveWorkspace } from './workspaceSelection'

const workspaces: Workspace[] = [
  {
    id: 'team-1',
    name: 'Team',
    slug: 'team',
    is_personal: false,
    mfa_required: false,
    created_by: 'user-1',
    created_at: '2026-05-25T10:00:00Z',
    updated_at: '2026-05-25T10:00:00Z',
  },
  {
    id: 'personal-1',
    name: 'Personal',
    slug: 'personal',
    is_personal: true,
    mfa_required: false,
    created_by: 'user-1',
    created_at: '2026-05-25T10:00:00Z',
    updated_at: '2026-05-25T10:00:00Z',
  },
]

describe('resolveActiveWorkspace', () => {
  it('keeps the preferred workspace when it is available', () => {
    expect(resolveActiveWorkspace(workspaces, 'team-1')?.id).toBe('team-1')
  })

  it('falls back to the personal workspace', () => {
    expect(resolveActiveWorkspace(workspaces, 'missing')?.id).toBe('personal-1')
  })

  it('returns null when no workspace exists', () => {
    expect(resolveActiveWorkspace([], 'missing')).toBeNull()
  })
})
