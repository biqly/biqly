import { describe, expect, it } from 'vitest'
import type { Conversation } from '../types/ai'
import { loadConversations, saveConversations } from './useConversation'

function conversation(id: string): Conversation {
  return {
    id,
    title: id,
    messages: [],
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

function storage(initial?: string) {
  let value = initial ?? null
  return {
    getItem: () => value,
    setItem: (_key: string, next: string) => {
      value = next
    },
    value: () => value,
  }
}

describe('conversation storage', () => {
  it('loads persisted conversations', () => {
    const expected = [conversation('conv-1')]
    expect(loadConversations(storage(JSON.stringify(expected)))).toEqual(expected)
  })

  it('returns an empty list when persisted data is invalid', () => {
    expect(loadConversations(storage('not json'))).toEqual([])
  })

  it('falls back to saving the most recent 20 conversations when full persistence fails', () => {
    const calls: string[] = []
    const flakyStorage = {
      getItem: () => null,
      setItem: (_key: string, next: string) => {
        calls.push(next)
        if (calls.length === 1) throw new Error('quota exceeded')
      },
    }
    const conversations = Array.from({ length: 25 }, (_, i) => conversation(`conv-${i}`))

    saveConversations(conversations, flakyStorage)

    expect(JSON.parse(calls[1] ?? '[]')).toHaveLength(20)
    expect(calls[1]).toContain('conv-24')
    expect(calls[1]).not.toContain('conv-0')
  })
})
