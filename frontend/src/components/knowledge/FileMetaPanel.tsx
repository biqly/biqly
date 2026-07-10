import type { KnowledgeFile } from '../../api/knowledge'
import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { formatDateTime } from '../../utils/formatters'
import { parseFrontmatter } from '../../utils/frontmatter'

interface FileMetaPanelProps {
  file: KnowledgeFile
  localeTag: string
  onEdit: () => void
  onDelete: () => void
  t: TFunction
}

export function FileMetaPanel({ file, localeTag, onEdit, onDelete, t }: FileMetaPanelProps) {
  const { frontmatter, raw } = parseFrontmatter(file.content_md)
  return (
    <aside className="flex w-full flex-col gap-3" aria-label={t('knowledge_base.kb_file_panel')}>
      <div className="border-border bg-card rounded-xl border p-3">
        <div className="mb-2 flex items-center justify-between gap-2">
          <h4 className="text-foreground-muted m-0 text-[0.7rem] font-bold tracking-widest uppercase">
            {t('knowledge_base.kb_file_panel')}
          </h4>
          <div className="flex gap-1.5">
            <button
              type="button"
              className={cn(buttonClass('secondary', { size: 'sm' }), 'w-auto!')}
              onClick={onEdit}
            >
              ✎ {t('knowledge_base.kb_edit')}
            </button>
            <button
              type="button"
              className={cn(buttonClass('ghost', { size: 'sm' }), 'text-error w-auto!')}
              onClick={onDelete}
              aria-label={t('knowledge_base.kb_delete')}
              title={t('knowledge_base.kb_delete')}
            >
              🗑
            </button>
          </div>
        </div>
        <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-[0.78rem]">
          <dt className="text-foreground-muted">{t('knowledge_base.kb_path')}</dt>
          <dd className="text-foreground m-0 font-mono wrap-anywhere">{file.path}</dd>
          <dt className="text-foreground-muted">{t('knowledge_base.kb_type')}</dt>
          <dd className="text-foreground m-0">
            {file.folder || t('knowledge_base.kb_folder_root')}
          </dd>
          <dt className="text-foreground-muted">{t('knowledge_base.kb_updated')}</dt>
          <dd className="text-foreground m-0">{formatDateTime(file.updated_at, localeTag)}</dd>
        </dl>
      </div>

      <div className="border-border bg-card rounded-xl border p-3">
        <h4 className="text-foreground-muted m-0 mb-2 flex items-center gap-1.5 text-[0.7rem] font-bold tracking-widest uppercase">
          <span aria-hidden="true" className="text-accent">
            ✦
          </span>
          {t('knowledge_base.kb_summary_title')}
        </h4>
        {frontmatter ? (
          <pre className="bg-canvas-subtle custom-scrollbar m-0 max-h-64 overflow-auto rounded-md p-2.5 text-[0.72rem] leading-normal whitespace-pre-wrap">
            {`---\n${raw.trim()}\n---`}
          </pre>
        ) : (
          <p className="text-foreground-muted m-0 text-[0.78rem] italic">
            {t('knowledge_base.kb_no_frontmatter')}
          </p>
        )}
      </div>
    </aside>
  )
}
