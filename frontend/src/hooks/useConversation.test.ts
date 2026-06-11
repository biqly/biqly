import { describe, expect, it } from 'vitest'

import type { Conversation, ConversationMessage } from '../types/ai'
import { loadConversations, saveConversations, withAssistantMessageForJob } from './useConversation'

function conversation(id: string, messages: ConversationMessage[] = []): Conversation {
  return {
    id,
    title: id,
    messages,
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
        if (calls.length === 1) {
          throw new Error('quota exceeded')
        }
      },
    }
    const conversations = Array.from({ length: 25 }, (_, i) => conversation(`conv-${i}`))

    saveConversations(conversations, flakyStorage)

    expect(JSON.parse(calls[1] ?? '[]')).toHaveLength(20)
    expect(calls[1]).toContain('conv-24')
    expect(calls[1]).not.toContain('conv-0')
  })
})

describe('withAssistantMessageForJob', () => {
  const userTurn: ConversationMessage = {
    role: 'user',
    content: 'how many tweets last month?',
    timestamp: '2026-01-01T00:00:00Z',
    job_id: 'job-1',
  }
  const answer = { content: 'Query executed' }

  it('appends the assistant message after a user turn tagged with the job', () => {
    const updated = withAssistantMessageForJob(
      [conversation('conv-1', [userTurn])],
      'conv-1',
      'job-1',
      answer,
    )

    expect(updated).not.toBeNull()
    const messages = updated?.[0]?.messages ?? []
    expect(messages).toHaveLength(2)
    expect(messages[1]).toMatchObject({ role: 'assistant', job_id: 'job-1', ...answer })
  })

  it('is idempotent: skips when the job result was already applied', () => {
    const applied: ConversationMessage = {
      role: 'assistant',
      content: 'Query executed',
      timestamp: '2026-01-01T00:00:01Z',
      job_id: 'job-1',
    }

    expect(
      withAssistantMessageForJob(
        [conversation('conv-1', [userTurn, applied])],
        'conv-1',
        'job-1',
        answer,
      ),
    ).toBeNull()
  })

  it('skips conversations without a user turn tagged with the job (legacy turns)', () => {
    const legacyTurn: ConversationMessage = { ...userTurn, job_id: undefined }

    expect(
      withAssistantMessageForJob([conversation('conv-1', [legacyTurn])], 'conv-1', 'job-1', answer),
    ).toBeNull()
  })

  it('skips unknown conversations', () => {
    expect(
      withAssistantMessageForJob([conversation('conv-1', [userTurn])], 'gone', 'job-1', answer),
    ).toBeNull()
  })
})
