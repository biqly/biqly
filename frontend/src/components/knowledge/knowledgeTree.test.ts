import { describe, expect, it } from 'vitest'

import type { KnowledgeFileMeta } from '../../api/knowledge'
import { buildKnowledgeTree } from './knowledgeTree'

const file = (path: string, folder: string, title = path): KnowledgeFileMeta => ({
  id: path,
  path,
  folder,
  title,
  status: 'published',
  updated_at: '2026-07-10T00:00:00Z',
})

describe('buildKnowledgeTree', () => {
  it('groups files under canonical folders and keeps empty ones visible', () => {
    const tree = buildKnowledgeTree(
      [file('instructions/rounding.md', 'instructions'), file('README.md', '')],
      '',
    )
    expect(tree.folders.map((f) => f.name)).toEqual([
      'glossary',
      'instructions',
      'metrics',
      'sql-pairs',
    ])
    expect(tree.folders.find((f) => f.name === 'instructions')?.files).toHaveLength(1)
    expect(tree.rootFiles.map((f) => f.path)).toEqual(['README.md'])
  })

  it('filters by search over path and title, hiding empty canonical folders', () => {
    const tree = buildKnowledgeTree(
      [
        file('glossary/money-transfer.md', 'glossary', 'money transfer'),
        file('metrics/fraud-rate.md', 'metrics', 'Fraud rate'),
      ],
      'fraud',
    )
    expect(tree.folders.map((f) => f.name)).toEqual(['metrics'])
    expect(tree.folders[0]!.files[0]!.path).toBe('metrics/fraud-rate.md')
  })

  it('keeps ad-hoc folders alphabetically merged', () => {
    const tree = buildKnowledgeTree([file('runbooks/x.md', 'runbooks')], '')
    expect(tree.folders.map((f) => f.name)).toContain('runbooks')
  })
})
