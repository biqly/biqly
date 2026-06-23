import type { SemanticJoin } from '../../types/semantic'
import type {
  CTERow,
  FilterRow,
  HavingRow,
  QueryBuilderMode,
  QueryBuilderSourceMode,
  SelectItem,
  WindowFuncRow,
} from './types'

export interface SavedQueryBuilderDraft {
  id: string
  name: string
  datasourceId: string
  source: QueryBuilderSourceMode
  modelId: string
  metadataBaseTableKey: string
  metadataJoins: SemanticJoin[]
  fieldLabelMode: 'human' | 'technical'
  mode: QueryBuilderMode
  selectItems: SelectItem[]
  filters: FilterRow[]
  groupBy: string[]
  having: HavingRow[]
  orderBy: string
  orderDir: string
  limit: number
  isSummarized: boolean
  windowFunctions: WindowFuncRow[]
  ctes: CTERow[]
  updatedAt: string
}

const STORAGE_KEY = 'biqly.queryBuilder.savedDrafts.v1'
const STRING_FIELDS = [
  'id',
  'name',
  'datasourceId',
  'modelId',
  'metadataBaseTableKey',
  'orderBy',
  'orderDir',
  'updatedAt',
]
const ARRAY_FIELDS = [
  'metadataJoins',
  'selectItems',
  'filters',
  'groupBy',
  'having',
  'windowFunctions',
  'ctes',
]

export function newSavedQueryDraftId(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `query-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function readSavedQueryDrafts(): SavedQueryBuilderDraft[] {
  if (typeof window === 'undefined') {
    return []
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return []
    }
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed.filter(isSavedQueryBuilderDraft).sort(byUpdatedDesc)
  } catch {
    return []
  }
}

export function writeSavedQueryDrafts(drafts: SavedQueryBuilderDraft[]): void {
  if (typeof window === 'undefined') {
    return
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(sortSavedQueryDrafts(drafts)))
}

export function upsertSavedQueryDraft(
  drafts: SavedQueryBuilderDraft[],
  draft: SavedQueryBuilderDraft,
): SavedQueryBuilderDraft[] {
  const next = drafts.filter((item) => item.id !== draft.id)
  next.push(draft)
  return sortSavedQueryDrafts(next)
}

export function deleteSavedQueryDraft(
  drafts: SavedQueryBuilderDraft[],
  id: string,
): SavedQueryBuilderDraft[] {
  return drafts.filter((draft) => draft.id !== id)
}

function sortSavedQueryDrafts(drafts: SavedQueryBuilderDraft[]): SavedQueryBuilderDraft[] {
  return [...drafts].sort(byUpdatedDesc)
}

function byUpdatedDesc(a: SavedQueryBuilderDraft, b: SavedQueryBuilderDraft): number {
  return b.updatedAt.localeCompare(a.updatedAt)
}

function isSavedQueryBuilderDraft(value: unknown): value is SavedQueryBuilderDraft {
  if (!isRecord(value)) {
    return false
  }
  return (
    hasStringFields(value, STRING_FIELDS) &&
    hasArrayFields(value, ARRAY_FIELDS) &&
    (value.source === 'semantic' || value.source === 'metadata') &&
    (value.fieldLabelMode === 'human' || value.fieldLabelMode === 'technical') &&
    (value.mode === 'simple' || value.mode === 'advanced') &&
    typeof value.limit === 'number' &&
    typeof value.isSummarized === 'boolean'
  )
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function hasStringFields(value: Record<string, unknown>, fields: string[]): boolean {
  return fields.every((field) => typeof value[field] === 'string')
}

function hasArrayFields(value: Record<string, unknown>, fields: string[]): boolean {
  return fields.every((field) => Array.isArray(value[field]))
}
