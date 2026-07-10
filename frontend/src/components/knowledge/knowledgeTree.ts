import type { KnowledgeFileMeta } from '../../api/knowledge'

export interface KnowledgeTreeFolder {
  name: string
  files: KnowledgeFileMeta[]
}

export interface KnowledgeTree {
  folders: KnowledgeTreeFolder[]
  rootFiles: KnowledgeFileMeta[]
}

// The canonical folders always render (even empty) so new users see the
// intended structure; extra ad-hoc folders follow alphabetically.
export const KNOWLEDGE_FOLDERS = ['glossary', 'instructions', 'metrics', 'sql-pairs'] as const

/** buildKnowledgeTree groups a flat file listing into folders + root files,
 * optionally filtered by a case-insensitive search over path and title. */
export function buildKnowledgeTree(files: KnowledgeFileMeta[], search: string): KnowledgeTree {
  const q = search.trim().toLowerCase()
  const matches = q
    ? files.filter((f) => f.path.toLowerCase().includes(q) || f.title.toLowerCase().includes(q))
    : files

  const byFolder = new Map<string, KnowledgeFileMeta[]>()
  const rootFiles: KnowledgeFileMeta[] = []
  for (const file of matches) {
    if (!file.folder) {
      rootFiles.push(file)
      continue
    }
    const bucket = byFolder.get(file.folder)
    if (bucket) {
      bucket.push(file)
    } else {
      byFolder.set(file.folder, [file])
    }
  }

  const names = new Set<string>(q ? [] : KNOWLEDGE_FOLDERS)
  for (const name of byFolder.keys()) {
    names.add(name)
  }
  const folders = [...names].sort().map((name) => ({
    name,
    files: (byFolder.get(name) ?? []).sort((a, b) => a.path.localeCompare(b.path)),
  }))
  return { folders, rootFiles: rootFiles.sort((a, b) => a.path.localeCompare(b.path)) }
}
