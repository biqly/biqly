import { useEffect, useRef, useState } from 'react'

import type { KnowledgeFileMeta } from '../../api/knowledge'
import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import type { KnowledgeTree } from './knowledgeTree'

interface FileTreeProps {
  tree: KnowledgeTree
  totalCount: number
  selectedId: string | null
  search: string
  onSearchChange: (value: string) => void
  onSelect: (file: KnowledgeFileMeta) => void
  onNewFile: () => void
  onNewFolder: (name: string) => void
  onBackfill: (() => void) | null
  backfilling: boolean
  onSeedStarters: () => void
  seeding: boolean
  t: TFunction
}

const treeIconButtonClass = cn(
  'border-border bg-card-raised text-foreground-muted inline-flex h-7 w-7 shrink-0 cursor-pointer',
  'items-center justify-center rounded-md border text-[0.8rem]',
  'hover:text-foreground hover:bg-(--control-hover-bg)',
)

function FileRow({
  file,
  selected,
  onSelect,
}: {
  file: KnowledgeFileMeta
  selected: boolean
  onSelect: () => void
}) {
  const name = file.path.includes('/') ? file.path.slice(file.path.indexOf('/') + 1) : file.path
  return (
    <li>
      <button
        type="button"
        onClick={onSelect}
        aria-current={selected ? 'true' : undefined}
        className={cn(
          'flex w-full cursor-pointer items-center gap-1.5 rounded-md border-0 bg-transparent px-2 py-1 text-left text-[0.8rem]',
          'text-foreground-muted hover:text-foreground hover:bg-(--control-hover-bg)',
          selected &&
            'text-foreground bg-[color-mix(in_srgb,var(--accent)_12%,transparent)] font-medium',
        )}
      >
        <span aria-hidden="true" className="shrink-0 opacity-60">
          🗎
        </span>
        <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{name}</span>
        {file.status === 'draft' && (
          <span
            aria-hidden="true"
            className="bg-warning ml-auto inline-block h-1.5 w-1.5 shrink-0 rounded-full"
          />
        )}
      </button>
    </li>
  )
}

export function FileTree({
  tree,
  totalCount,
  selectedId,
  search,
  onSearchChange,
  onSelect,
  onNewFile,
  onNewFolder,
  onBackfill,
  backfilling,
  onSeedStarters,
  seeding,
  t,
}: FileTreeProps) {
  // Folders the user collapsed; searching temporarily expands everything so
  // matches are never hidden behind a closed folder.
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())
  const [newFolderOpen, setNewFolderOpen] = useState(false)
  const newFolderInputRef = useRef<HTMLInputElement>(null)
  const [newFolderName, setNewFolderName] = useState('')
  const searching = search.trim() !== ''

  useEffect(() => {
    if (newFolderOpen) {
      newFolderInputRef.current?.focus()
    }
  }, [newFolderOpen])

  const toggleFolder = (name: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev)
      if (next.has(name)) {
        next.delete(name)
      } else {
        next.add(name)
      }
      return next
    })
  }

  const submitNewFolder = () => {
    const name = newFolderName
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9-]+/g, '-')
      .replace(/^-+|-+$/g, '')
    if (name) {
      onNewFolder(name)
    }
    setNewFolderName('')
    setNewFolderOpen(false)
  }

  return (
    <nav aria-label={t('knowledge_base.kb_tree_aria')} className="flex min-h-0 flex-col gap-3">
      <div className="flex items-center gap-1.5">
        <h3 className="text-foreground m-0 min-w-0 flex-1 overflow-hidden text-[0.85rem] font-bold text-ellipsis whitespace-nowrap">
          {t('knowledge_base.kb_title')}
        </h3>
        <button
          type="button"
          className={treeIconButtonClass}
          onClick={() => setNewFolderOpen((open) => !open)}
          aria-label={t('knowledge_base.kb_new_folder')}
          title={t('knowledge_base.kb_new_folder')}
        >
          🗀+
        </button>
        <button
          type="button"
          className={cn(treeIconButtonClass, 'border-accent/50 text-accent')}
          onClick={onNewFile}
          aria-label={t('knowledge_base.kb_new_file')}
          title={t('knowledge_base.kb_new_file')}
        >
          🗎+
        </button>
      </div>
      {newFolderOpen && (
        <div className="flex items-center gap-1.5">
          <input
            ref={newFolderInputRef}
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                submitNewFolder()
              } else if (e.key === 'Escape') {
                setNewFolderOpen(false)
                setNewFolderName('')
              }
            }}
            placeholder={t('knowledge_base.kb_new_folder_placeholder')}
            aria-label={t('knowledge_base.kb_new_folder')}
            className="border-accent bg-card-raised h-8 min-w-0 flex-1 rounded-md border px-2.5 text-[0.8rem]"
          />
          <button
            type="button"
            className={treeIconButtonClass}
            onClick={submitNewFolder}
            aria-label={t('knowledge_base.kb_create')}
            title={t('knowledge_base.kb_create')}
          >
            ↵
          </button>
        </div>
      )}
      <input
        type="search"
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={t('knowledge_base.kb_search_placeholder')}
        aria-label={t('knowledge_base.kb_search_placeholder')}
        className="border-border bg-card-raised h-8 w-full rounded-md border px-2.5 text-[0.8rem]"
      />
      <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto pr-1">
        {totalCount === 0 ? (
          <div className="text-foreground-muted flex flex-col items-start gap-2 py-2 text-[0.8rem]">
            <p className="m-0">{t('knowledge_base.kb_empty')}</p>
            {onBackfill && (
              <button
                type="button"
                className={cn(buttonClass('secondary', { size: 'sm' }), 'w-auto!')}
                disabled={backfilling}
                onClick={onBackfill}
              >
                {backfilling ? t('knowledge_base.kb_backfilling') : t('knowledge_base.kb_backfill')}
              </button>
            )}
          </div>
        ) : (
          <ul className="m-0 flex list-none flex-col gap-1 p-0">
            {tree.folders.map((folder) => {
              const isOpen = searching || !collapsed.has(folder.name)
              return (
                <li key={folder.name}>
                  <button
                    type="button"
                    className="text-foreground-muted hover:text-foreground flex w-full cursor-pointer items-center gap-1.5 rounded-md border-0 bg-transparent px-1 py-1 text-left text-[0.78rem] font-semibold hover:bg-(--control-hover-bg)"
                    aria-expanded={isOpen}
                    onClick={() => toggleFolder(folder.name)}
                  >
                    <span aria-hidden="true" className="text-foreground-faint w-3 text-[0.6rem]">
                      {isOpen ? '▾' : '▸'}
                    </span>
                    <span aria-hidden="true">🗀</span>
                    <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
                      {folder.name}
                    </span>
                    <span className="text-foreground-faint ml-auto tabular-nums">
                      {folder.files.length}
                    </span>
                  </button>
                  {isOpen && folder.files.length > 0 && (
                    <ul className="m-0 flex list-none flex-col p-0 pl-4">
                      {folder.files.map((file) => (
                        <FileRow
                          key={file.id}
                          file={file}
                          selected={file.id === selectedId}
                          onSelect={() => onSelect(file)}
                        />
                      ))}
                    </ul>
                  )}
                </li>
              )
            })}
            {tree.rootFiles.map((file) => (
              <FileRow
                key={file.id}
                file={file}
                selected={file.id === selectedId}
                onSelect={() => onSelect(file)}
              />
            ))}
          </ul>
        )}
      </div>
      <div className="border-border border-t pt-2">
        <button
          type="button"
          className="text-foreground-muted hover:text-foreground w-full cursor-pointer rounded-md border-0 bg-transparent px-2 py-1 text-left text-[0.75rem] hover:bg-(--control-hover-bg)"
          onClick={onSeedStarters}
          disabled={seeding}
        >
          {seeding ? t('knowledge_base.kb_backfilling') : `✚ ${t('knowledge_base.kb_starters')}`}
        </button>
      </div>
    </nav>
  )
}
