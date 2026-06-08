import { useCallback, useState } from 'react'

import type { AIQueryResponse, Conversation, ConversationMessage } from '../types/ai'

const STORAGE_KEY = 'biqly_conversations'
type ConversationStorage = Pick<Storage, 'getItem' | 'setItem'>

function defaultConversationStorage(): ConversationStorage | null {
  return typeof localStorage === 'undefined' ? null : localStorage
}

export function loadConversations(
  storage: ConversationStorage | null = defaultConversationStorage(),
): Conversation[] {
  if (!storage) {
    return []
  }
  try {
    const raw = storage.getItem(STORAGE_KEY)
    if (!raw) {
      return []
    }
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as Conversation[]) : []
  } catch {
    return []
  }
}

export function saveConversations(
  conversations: Conversation[],
  storage: ConversationStorage | null = defaultConversationStorage(),
) {
  if (!storage) {
    return
  }
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(conversations))
  } catch {
    try {
      storage.setItem(STORAGE_KEY, JSON.stringify(conversations.slice(-20)))
    } catch {
      // Ignore storage quota/private-mode failures; in-memory state still updates.
    }
  }
}

function generateId(): string {
  return `conv_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

export function useConversation() {
  const [initialConversations] = useState(loadConversations)
  const [conversations, setConversations] = useState<Conversation[]>(initialConversations)
  const [activeConversationId, setActiveConversationId] = useState<string | null>(
    () => initialConversations[0]?.id ?? null,
  )

  const activeConversation = conversations.find((c) => c.id === activeConversationId) ?? null

  const persist = useCallback((updated: Conversation[]) => {
    setConversations(updated)
    saveConversations(updated)
  }, [])

  const createConversation = useCallback(() => {
    const now = new Date().toISOString()
    const conv: Conversation = {
      id: generateId(),
      messages: [],
      created_at: now,
      updated_at: now,
    }
    setActiveConversationId(conv.id)
    setConversations((prev) => {
      const updated = [conv, ...prev]
      saveConversations(updated)
      return updated
    })
    return conv
  }, [])

  const addMessage = useCallback(
    (message: Omit<ConversationMessage, 'timestamp'>) => {
      let targetId = activeConversationId
      if (!targetId) {
        const conv = createConversation()
        targetId = conv.id
      }
      setConversations((prev) => {
        const updated = prev.map((c) => {
          if (c.id !== targetId) {
            return c
          }
          const ts = new Date().toISOString()
          const msg: ConversationMessage = { ...message, timestamp: ts }
          const newMessages = [...c.messages, msg]
          const title =
            c.title ?? (message.role === 'user' ? message.content.slice(0, 60) : undefined)
          return { ...c, messages: newMessages, title, updated_at: ts }
        })
        saveConversations(updated)
        return updated
      })
    },
    [activeConversationId, createConversation],
  )

  const deleteConversation = useCallback(
    (id: string) => {
      const updated = conversations.filter((c) => c.id !== id)
      persist(updated)
      if (activeConversationId === id) {
        setActiveConversationId(updated[0]?.id ?? null)
      }
    },
    [activeConversationId, conversations, persist],
  )

  const renameConversation = useCallback(
    (id: string, title: string) => {
      const updated = conversations.map((c) =>
        c.id === id ? { ...c, title, updated_at: new Date().toISOString() } : c,
      )
      persist(updated)
    },
    [conversations, persist],
  )

  const clearConversation = useCallback(() => {
    if (!activeConversationId) {
      return
    }
    const updated = conversations.map((c) =>
      c.id === activeConversationId
        ? { ...c, messages: [], updated_at: new Date().toISOString() }
        : c,
    )
    persist(updated)
  }, [activeConversationId, conversations, persist])

  const updateMessageResponse = useCallback(
    (conversationId: string, messageIndex: number, aiResponse: AIQueryResponse) => {
      setConversations((prev) => {
        const updated = prev.map((c) => {
          if (c.id !== conversationId) {
            return c
          }
          const newMessages = c.messages.map((m, idx) =>
            idx === messageIndex ? { ...m, ai_response: aiResponse } : m,
          )
          return { ...c, messages: newMessages, updated_at: new Date().toISOString() }
        })
        saveConversations(updated)
        return updated
      })
    },
    [],
  )

  return {
    conversations,
    activeConversation,
    activeConversationId,
    setActiveConversationId,
    createConversation,
    addMessage,
    deleteConversation,
    renameConversation,
    clearConversation,
    updateMessageResponse,
  }
}
