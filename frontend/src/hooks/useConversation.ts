import { useCallback, useEffect, useRef, useState } from 'react'

import { apiFetch } from '../api/apiClient'
import type { AIQueryResponse, Conversation, ConversationMessage } from '../types/ai'

const STORAGE_KEY = 'biqly_conversations'
type ConversationStorage = Pick<Storage, 'getItem' | 'setItem'>
type ConversationLoader = (token?: string | null) => Promise<Conversation[]>
type ConversationSaver = (conversation: Conversation, token?: string | null) => Promise<void>
type ConversationDeleter = (id: string, token?: string | null) => Promise<void>
type ConversationScope = Pick<Conversation, 'datasource_id' | 'model_id'>

interface ConversationAPIOptions {
  token?: string | null
  api?: ConversationLoader
}

interface ConversationSaveOptions {
  token?: string | null
  api?: ConversationSaver
}

interface ConversationDeleteOptions {
  token?: string | null
  api?: ConversationDeleter
}

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
    return Array.isArray(parsed) ? (parsed as Conversation[]).map(normalizeConversation) : []
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

function normalizeConversation(conversation: Conversation): Conversation {
  return {
    ...conversation,
    context_enabled: conversation.context_enabled ?? true,
  }
}

async function defaultLoadConversationsAPI(token?: string | null): Promise<Conversation[]> {
  return apiFetch<Conversation[]>('GET', '/api/ai/conversations', undefined, {
    token: token ?? undefined,
  })
}

async function defaultSaveConversationAPI(
  conversation: Conversation,
  token?: string | null,
): Promise<void> {
  if (!conversation.datasource_id) {
    return
  }
  await apiFetch<Conversation>('POST', '/api/ai/conversations', conversation, {
    token: token ?? undefined,
  })
}

async function defaultDeleteConversationAPI(id: string, token?: string | null): Promise<void> {
  await apiFetch<void>('DELETE', `/api/ai/conversations/${encodeURIComponent(id)}`, undefined, {
    token: token ?? undefined,
  })
}

export async function loadConversationSnapshot(
  localConversations: Conversation[],
  options: ConversationAPIOptions = {},
): Promise<Conversation[]> {
  try {
    const loaded = await (options.api ?? defaultLoadConversationsAPI)(options.token)
    return loaded.map(normalizeConversation)
  } catch {
    return localConversations
  }
}

export async function saveConversationSnapshot(
  conversation: Conversation,
  options: ConversationSaveOptions = {},
): Promise<void> {
  try {
    await (options.api ?? defaultSaveConversationAPI)(
      normalizeConversation(conversation),
      options.token,
    )
  } catch {
    // Local storage remains the fallback source of truth when the backend is
    // temporarily unavailable or auth is not configured.
  }
}

export async function deleteConversationSnapshot(
  id: string,
  options: ConversationDeleteOptions = {},
): Promise<void> {
  try {
    await (options.api ?? defaultDeleteConversationAPI)(id, options.token)
  } catch {
    // The local delete still wins while offline; the backend can be reconciled
    // by future sync once conversations are recreated or listed remotely.
  }
}

/** Append a job's assistant message to its conversation, or return null when
 * nothing should change: unknown conversation, no user turn tagged with the
 * job (legacy turns), or the result was already applied (idempotency). */
export function withAssistantMessageForJob(
  conversations: Conversation[],
  conversationId: string,
  jobId: string,
  message: Omit<ConversationMessage, 'timestamp' | 'job_id' | 'role'>,
): Conversation[] | null {
  const conv = conversations.find((c) => c.id === conversationId)
  if (!conv) {
    return null
  }
  const hasUserTurn = conv.messages.some((m) => m.role === 'user' && m.job_id === jobId)
  const alreadyApplied = conv.messages.some((m) => m.role === 'assistant' && m.job_id === jobId)
  if (!hasUserTurn || alreadyApplied) {
    return null
  }
  const ts = new Date().toISOString()
  const msg: ConversationMessage = {
    ...message,
    role: 'assistant',
    job_id: jobId,
    timestamp: ts,
  }
  return conversations.map((c) =>
    c.id === conversationId ? { ...c, messages: [...c.messages, msg], updated_at: ts } : c,
  )
}

export function useConversation(accessToken?: string | null) {
  const [initialConversations] = useState(loadConversations)
  const [conversations, setConversations] = useState<Conversation[]>(initialConversations)
  const [activeConversationId, setActiveConversationId] = useState<string | null>(
    () => initialConversations[0]?.id ?? null,
  )
  const remoteLoadedRef = useRef(false)

  const activeConversation = conversations.find((c) => c.id === activeConversationId) ?? null

  const persistRemote = useCallback(
    (updated: Conversation[]) => {
      if (!accessToken) {
        return
      }
      for (const conversation of updated) {
        void saveConversationSnapshot(conversation, { token: accessToken })
      }
    },
    [accessToken],
  )

  const persist = useCallback(
    (updated: Conversation[]) => {
      setConversations(updated)
      saveConversations(updated)
      persistRemote(updated)
    },
    [persistRemote],
  )

  useEffect(() => {
    if (remoteLoadedRef.current || !accessToken) {
      return
    }
    remoteLoadedRef.current = true
    void loadConversationSnapshot(initialConversations, { token: accessToken }).then((loaded) => {
      setConversations(loaded)
      saveConversations(loaded)
      setActiveConversationId((current) => current ?? loaded[0]?.id ?? null)
    })
  }, [accessToken, initialConversations])

  const createConversation = useCallback(
    (scope: ConversationScope = {}) => {
      const now = new Date().toISOString()
      const conv: Conversation = {
        id: generateId(),
        messages: [],
        created_at: now,
        updated_at: now,
        context_enabled: true,
        datasource_id: scope.datasource_id,
        model_id: scope.model_id,
      }
      setActiveConversationId(conv.id)
      setConversations((prev) => {
        const updated = [conv, ...prev]
        saveConversations(updated)
        persistRemote(updated)
        return updated
      })
      return conv
    },
    [persistRemote],
  )

  const addMessage = useCallback(
    (
      message: Omit<ConversationMessage, 'timestamp'>,
      conversationId?: string,
      scope: ConversationScope = {},
    ) => {
      let targetId = conversationId ?? activeConversationId
      if (!targetId) {
        const conv = createConversation(scope)
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
          return {
            ...c,
            ...scope,
            messages: newMessages,
            title,
            updated_at: ts,
          }
        })
        saveConversations(updated)
        persistRemote(updated)
        return updated
      })
      return targetId
    },
    [activeConversationId, createConversation, persistRemote],
  )

  const appendAssistantForJob = useCallback(
    (
      conversationId: string,
      jobId: string,
      message: Omit<ConversationMessage, 'timestamp' | 'job_id' | 'role'>,
    ) => {
      setConversations((prev) => {
        const updated = withAssistantMessageForJob(prev, conversationId, jobId, message)
        if (!updated) {
          return prev
        }
        saveConversations(updated)
        persistRemote(updated)
        return updated
      })
    },
    [persistRemote],
  )

  const deleteConversation = useCallback(
    (id: string) => {
      const updated = conversations.filter((c) => c.id !== id)
      persist(updated)
      if (accessToken) {
        void deleteConversationSnapshot(id, { token: accessToken })
      }
      if (activeConversationId === id) {
        setActiveConversationId(updated[0]?.id ?? null)
      }
    },
    [accessToken, activeConversationId, conversations, persist],
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

  const updateConversationContext = useCallback(
    (id: string, contextEnabled: boolean) => {
      const updated = conversations.map((c) =>
        c.id === id
          ? { ...c, context_enabled: contextEnabled, updated_at: new Date().toISOString() }
          : c,
      )
      persist(updated)
    },
    [conversations, persist],
  )

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
        persistRemote(updated)
        return updated
      })
    },
    [persistRemote],
  )

  return {
    conversations,
    activeConversation,
    activeConversationId,
    setActiveConversationId,
    createConversation,
    addMessage,
    appendAssistantForJob,
    deleteConversation,
    renameConversation,
    clearConversation,
    updateConversationContext,
    updateMessageResponse,
  }
}
