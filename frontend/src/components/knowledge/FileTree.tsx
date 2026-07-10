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
  onBackfill: (() => void) | null
  backfilling: boolean
  t: TFunction
}

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
          'text-foreground-muted hover:text-foreground hover:bg-[var(--control-hover-bg)]',
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
  onBackfill,
  backfilling,
  t,
}: FileTreeProps) {
  return (
    <nav aria-label={t('knowledge_base.kb_tree_aria')} className="flex min-h-0 flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-foreground m-0 text-[0.85rem] font-bold">
          {t('knowledge_base.kb_title')}
        </h3>
        <button
          type="button"
          className={cn(buttonClass('primary', { size: 'sm' }), 'w-auto!')}
          onClick={onNewFile}
        >
          {t('knowledge_base.kb_new_file')}
        </button>
      </div>
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
            {tree.folders.map((folder) => (
              <li key={folder.name}>
                <div className="text-foreground-muted flex items-center gap-1.5 px-1 py-1 text-[0.78rem] font-semibold">
                  <span aria-hidden="true">🗀</span>
                  <span>{folder.name}</span>
                  <span className="text-foreground-faint ml-auto tabular-nums">
                    {folder.files.length}
                  </span>
                </div>
                {folder.files.length > 0 && (
                  <ul className="m-0 flex list-none flex-col p-0 pl-3">
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
            ))}
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
    </nav>
  )
}
