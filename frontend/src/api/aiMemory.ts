import { apiFetch } from './apiClient'
import { AI_API_BASE } from './constants'

export interface MemoryEntry {
  id: string
  content: string
  source: string
  created_at: string
  updated_at: string
}

export async function listMemoryEntries(token?: string): Promise<MemoryEntry[]> {
  const res = await apiFetch<{ entries: MemoryEntry[] }>(
    'GET',
    `${AI_API_BASE}/memory/entries`,
    undefined,
    { token },
  )
  return res.entries
}

export async function createMemoryEntry(content: string, token?: string): Promise<void> {
  await apiFetch<{ id: string }>('POST', `${AI_API_BASE}/memory/entries`, { content }, { token })
}

export async function updateMemoryEntry(
  id: string,
  content: string,
  token?: string,
): Promise<void> {
  await apiFetch<void>('PUT', `${AI_API_BASE}/memory/entries/${id}`, { content }, { token })
}

export async function deleteMemoryEntry(id: string, token?: string): Promise<void> {
  await apiFetch<void>('DELETE', `${AI_API_BASE}/memory/entries/${id}`, undefined, { token })
}
