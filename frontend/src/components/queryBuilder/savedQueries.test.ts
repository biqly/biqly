import { describe, expect, it } from 'vitest'

import {
  deleteSavedQueryDraft,
  type SavedQueryBuilderDraft,
  upsertSavedQueryDraft,
} from './savedQueries'

function draft(id: string, updatedAt: string): SavedQueryBuilderDraft {
  return {
    id,
    name: id,
    datasourceId: 'ds',
    source: 'metadata',
    modelId: '',
    metadataBaseTableKey: 'public.orders',
    metadataJoins: [],
    fieldLabelMode: 'human',
    mode: 'simple',
    selectItems: [],
    filters: [],
    groupBy: [],
    having: [],
    orderBy: '',
    orderDir: 'asc',
    limit: 100,
    isSummarized: true,
    windowFunctions: [],
    ctes: [],
    updatedAt,
  }
}

describe('saved query builder drafts', () => {
  it('upserts drafts and keeps the newest first', () => {
    const existing = draft('a', '2026-06-22T10:00:00.000Z')
    const next = draft('b', '2026-06-23T10:00:00.000Z')

    expect(upsertSavedQueryDraft([existing], next).map((item) => item.id)).toEqual(['b', 'a'])
  })

  it('replaces an existing draft by id', () => {
    const existing = draft('a', '2026-06-22T10:00:00.000Z')
    const renamed = { ...existing, name: 'Renamed', updatedAt: '2026-06-23T10:00:00.000Z' }

    expect(upsertSavedQueryDraft([existing], renamed)).toEqual([renamed])
  })

  it('deletes a draft by id', () => {
    expect(
      deleteSavedQueryDraft([draft('a', ''), draft('b', '')], 'a').map((item) => item.id),
    ).toEqual(['b'])
  })
})
