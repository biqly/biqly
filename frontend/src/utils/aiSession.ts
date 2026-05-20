const STORAGE_KEY = 'biqly_ai_client_session'

export function getAIClientSessionId(): string {
  try {
    const existing = localStorage.getItem(STORAGE_KEY)
    if (existing?.trim()) return existing.trim()
    const id = crypto.randomUUID()
    localStorage.setItem(STORAGE_KEY, id)
    return id
  } catch {
    return crypto.randomUUID()
  }
}
