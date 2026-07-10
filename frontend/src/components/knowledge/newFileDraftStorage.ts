// The in-progress "New file" draft survives accidental refreshes/navigation:
// every field mirrors to localStorage (per datasource) and is restored when
// the modal reopens; a successful create clears it via clearNewFileDraft.
const DRAFT_STORAGE_PREFIX = 'biqly.knowledge.newFileDraft.'

export interface PersistedNewFileDraft {
  folder: string
  path: string
  aiPrompt: string
  content: string
}

export function loadNewFileDraft(datasourceId: string): PersistedNewFileDraft | null {
  try {
    const raw = window.localStorage.getItem(DRAFT_STORAGE_PREFIX + datasourceId)
    if (!raw) {
      return null
    }
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') {
      return null
    }
    const d = parsed as Partial<PersistedNewFileDraft>
    return {
      folder: typeof d.folder === 'string' ? d.folder : 'instructions',
      path: typeof d.path === 'string' ? d.path : '',
      aiPrompt: typeof d.aiPrompt === 'string' ? d.aiPrompt : '',
      content: typeof d.content === 'string' ? d.content : '',
    }
  } catch {
    return null
  }
}

export function saveNewFileDraft(datasourceId: string, draft: PersistedNewFileDraft): void {
  try {
    window.localStorage.setItem(DRAFT_STORAGE_PREFIX + datasourceId, JSON.stringify(draft))
  } catch {
    // Best-effort: the modal still works without persistence.
  }
}

export function clearNewFileDraft(datasourceId: string): void {
  try {
    window.localStorage.removeItem(DRAFT_STORAGE_PREFIX + datasourceId)
  } catch {
    // ignore
  }
}
