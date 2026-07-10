import { describe, expect, it } from 'vitest'

import { conversationGroupKey, groupConversationsByRecency } from './conversationGroups'

const now = new Date('2026-07-10T15:00:00')

describe('conversationGroupKey', () => {
  it('buckets timestamps by recency', () => {
    expect(conversationGroupKey('2026-07-10T09:00:00', now)).toBe('today')
    expect(conversationGroupKey('2026-07-09T23:59:00', now)).toBe('yesterday')
    expect(conversationGroupKey('2026-07-06T10:00:00', now)).toBe('week')
    expect(conversationGroupKey('2026-06-20T10:00:00', now)).toBe('month')
    expect(conversationGroupKey('2025-07-10T10:00:00', now)).toBe('older')
    expect(conversationGroupKey(undefined, now)).toBe('older')
    expect(conversationGroupKey('not-a-date', now)).toBe('older')
  })
})

describe('groupConversationsByRecency', () => {
  it('keeps fixed group order and sorts newest first inside groups', () => {
    const groups = groupConversationsByRecency(
      [
        { id: 'old', updated_at: '2026-05-01T10:00:00' },
        { id: 'today-early', updated_at: '2026-07-10T08:00:00' },
        { id: 'today-late', updated_at: '2026-07-10T14:00:00' },
        { id: 'yesterday', updated_at: '2026-07-09T12:00:00' },
      ],
      now,
    )
    expect(groups.map((g) => g.key)).toEqual(['today', 'yesterday', 'older'])
    expect(groups[0]!.items.map((i) => i.id)).toEqual(['today-late', 'today-early'])
  })
})
