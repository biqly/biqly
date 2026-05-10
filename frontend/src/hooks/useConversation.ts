import { useCallback, useState } from 'react'
import type { Conversation, ConversationMessage } from '../types/ai'

const STORAGE_KEY = 'biqly_conversations'

function loadConversations(): Conversation[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveConversations(conversations: Conversation[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(conversations))
  } catch {
    // quota exceeded — silently drop oldest
    const trimmed = conversations.slice(-20)
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed))
  }
}

function generateId(): string {
  return `conv_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}

export function useConversation() {
  const [conversations, setConversations] = useState<Conversation[]>(loadConversations)
  const [activeConversationId, setActiveConversationId] = useState<string | null>(
    () => loadConversations()[0]?.id ?? null
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
    persist([conv, ...conversations])
    setActiveConversationId(conv.id)
    return conv
  }, [conversations, persist])

  const addMessage = useCallback(
    (message: Omit<ConversationMessage, 'timestamp'>) => {
      if (!activeConversationId) {
        createConversation()
        return
      }
      const updated = conversations.map((c) => {
        if (c.id !== activeConversationId) return c
        const ts = new Date().toISOString()
        const msg: ConversationMessage = { ...message, timestamp: ts }
        const newMessages = [...c.messages, msg]
        const title =
          c.title ??
          (message.role === 'user' ? message.content.slice(0, 60) : undefined)
        return { ...c, messages: newMessages, title, updated_at: ts }
      })
      persist(updated)
    },
    [activeConversationId, conversations, createConversation, persist]
  )

  const deleteConversation = useCallback(
    (id: string) => {
      const updated = conversations.filter((c) => c.id !== id)
      persist(updated)
      if (activeConversationId === id) {
        setActiveConversationId(updated[0]?.id ?? null)
      }
    },
    [activeConversationId, conversations, persist]
  )

  const renameConversation = useCallback(
    (id: string, title: string) => {
      const updated = conversations.map((c) =>
        c.id === id ? { ...c, title, updated_at: new Date().toISOString() } : c
      )
      persist(updated)
    },
    [conversations, persist]
  )

  const clearConversation = useCallback(() => {
    if (!activeConversationId) return
    const updated = conversations.map((c) =>
      c.id === activeConversationId
        ? { ...c, messages: [], updated_at: new Date().toISOString() }
        : c
    )
    persist(updated)
  }, [activeConversationId, conversations, persist])

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
  }
}
