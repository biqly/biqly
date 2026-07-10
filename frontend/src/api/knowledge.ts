import { apiFetch } from './apiClient'

// KnowledgeFileMeta is the tree/listing shape (no content).
export interface KnowledgeFileMeta {
  id: string
  path: string
  folder: string
  title: string
  description?: string
  status: 'draft' | 'published'
  updated_at: string
}

// KnowledgeFile is one full markdown document of the knowledge base.
export interface KnowledgeFile extends KnowledgeFileMeta {
  content_md: string
  frontmatter?: Record<string, unknown>
  created_by?: string
  created_at: string
}

export async function listKnowledgeFiles(datasourceId: string): Promise<KnowledgeFileMeta[]> {
  if (!datasourceId) {
    return []
  }
  const data = await apiFetch<{ files: KnowledgeFileMeta[] }>(
    'GET',
    `/api/ai/knowledge/files?datasource_id=${encodeURIComponent(datasourceId)}`,
  )
  return data.files
}

export async function getKnowledgeFile(id: string): Promise<KnowledgeFile> {
  const data = await apiFetch<{ file: KnowledgeFile }>(
    'GET',
    `/api/ai/knowledge/files/${encodeURIComponent(id)}`,
  )
  return data.file
}

export async function createKnowledgeFile(req: {
  datasource_id: string
  path: string
  content_md: string
}): Promise<string> {
  const data = await apiFetch<{ id: string }>('POST', '/api/ai/knowledge/files', req)
  return data.id
}

export async function updateKnowledgeFile(
  id: string,
  req: { path: string; content_md: string },
): Promise<void> {
  await apiFetch('PUT', `/api/ai/knowledge/files/${encodeURIComponent(id)}`, req)
}

export async function deleteKnowledgeFile(id: string): Promise<void> {
  await apiFetch('DELETE', `/api/ai/knowledge/files/${encodeURIComponent(id)}`)
}

export async function publishKnowledgeFile(id: string): Promise<void> {
  await apiFetch('POST', `/api/ai/knowledge/files/${encodeURIComponent(id)}/publish`)
}

export async function backfillKnowledge(datasourceId: string): Promise<number> {
  const data = await apiFetch<{ created: number }>('POST', '/api/ai/knowledge/backfill', {
    datasource_id: datasourceId,
  })
  return data.created
}

// AI drafting can take a while; allow more than the default 30s.
const KNOWLEDGE_DRAFT_TIMEOUT_MS = 120_000

export interface KnowledgeDraft {
  path: string
  content_md: string
}

// draftKnowledgeFile asks the backend (describe-purpose model — the same one
// that writes metadata descriptions) to write a markdown draft for the folder.
// Nothing is persisted; the caller opens the draft in the editor for review.
export async function draftKnowledgeFile(req: {
  datasource_id: string
  folder: string
  prompt: string
}): Promise<KnowledgeDraft> {
  return apiFetch<KnowledgeDraft>('POST', '/api/ai/knowledge/draft', req, {
    timeout: KNOWLEDGE_DRAFT_TIMEOUT_MS,
  })
}
