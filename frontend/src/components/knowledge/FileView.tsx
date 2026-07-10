import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import type { KnowledgeFile } from '../../api/knowledge'
import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { parseFrontmatter } from '../../utils/frontmatter'
import { MarkdownEditor } from './MarkdownEditor'

type ViewMode = 'rendered' | 'source'

interface FileViewProps {
  file: KnowledgeFile
  editing: boolean
  saving: boolean
  publishing: boolean
  onCancelEdit: () => void
  onSave: (content: string) => void
  onPublish: () => void
  t: TFunction
}

// Markdown body styling: headings/lists/code get sensible defaults inside the
// card without a global typography plugin.
const markdownBodyClass = cn(
  'text-foreground min-w-0 text-[0.88rem] leading-relaxed',
  '[&_h1]:mt-0 [&_h1]:mb-3 [&_h1]:text-[1.15rem] [&_h1]:font-bold',
  '[&_h2]:mt-5 [&_h2]:mb-2 [&_h2]:text-[1rem] [&_h2]:font-bold',
  '[&_h3]:mt-4 [&_h3]:mb-1.5 [&_h3]:text-[0.92rem] [&_h3]:font-semibold',
  '[&_p]:my-2 [&_ul]:my-2 [&_ol]:my-2 [&_li]:my-0.5',
  '[&_code]:bg-canvas-subtle [&_code]:rounded [&_code]:px-1 [&_code]:py-0.5 [&_code]:text-[0.8rem]',
  '[&_pre]:bg-canvas-subtle [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:p-3',
  '[&_pre_code]:bg-transparent [&_pre_code]:p-0',
  '[&_table]:my-2 [&_table]:border-collapse [&_th]:border [&_th]:border-border [&_th]:px-2 [&_th]:py-1',
  '[&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1',
  '[&_blockquote]:border-border [&_blockquote]:text-foreground-muted [&_blockquote]:my-2 [&_blockquote]:border-l-2 [&_blockquote]:pl-3',
)

export function FileView({
  file,
  editing,
  saving,
  publishing,
  onCancelEdit,
  onSave,
  onPublish,
  t,
}: FileViewProps) {
  const [mode, setMode] = useState<ViewMode>('rendered')
  const [draft, setDraft] = useState(file.content_md)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(file.content_md)
  }, [file.id, file.content_md])

  const { body } = parseFrontmatter(file.content_md)

  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col" aria-label={file.path}>
      <div className="border-border flex flex-wrap items-center gap-2 border-b px-4 py-2.5">
        <span className="text-foreground text-[0.85rem] font-semibold">{file.path}</span>
        <span
          className={cn(
            'rounded-full border px-2 py-0.5 text-[0.68rem] font-semibold',
            file.status === 'published'
              ? 'border-success/40 text-success'
              : 'border-warning/50 text-warning',
          )}
        >
          {file.status === 'published'
            ? t('knowledge_base.kb_published_badge')
            : t('knowledge_base.kb_draft_badge')}
        </span>
        <div className="ml-auto flex items-center gap-2">
          {!editing && (
            <div
              className="border-border flex overflow-hidden rounded-md border"
              role="tablist"
              aria-label={t('knowledge_base.kb_rendered')}
            >
              {(['rendered', 'source'] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  role="tab"
                  aria-selected={mode === m}
                  className={cn(
                    'cursor-pointer border-0 px-2.5 py-1 text-[0.75rem]',
                    mode === m
                      ? 'text-foreground bg-[color-mix(in_srgb,var(--accent)_16%,transparent)] font-semibold'
                      : 'text-foreground-muted bg-transparent',
                  )}
                  onClick={() => setMode(m)}
                >
                  {m === 'rendered'
                    ? t('knowledge_base.kb_rendered')
                    : t('knowledge_base.kb_source')}
                </button>
              ))}
            </div>
          )}
          {editing ? (
            <>
              <button
                type="button"
                className={cn(buttonClass('ghost', { size: 'sm' }), 'w-auto!')}
                onClick={onCancelEdit}
                disabled={saving}
              >
                {t('knowledge_base.kb_cancel')}
              </button>
              <button
                type="button"
                className={cn(buttonClass('primary', { size: 'sm' }), 'w-auto!')}
                onClick={() => onSave(draft)}
                disabled={saving || !draft.trim()}
              >
                {t('knowledge_base.kb_save')}
              </button>
            </>
          ) : (
            file.status !== 'published' && (
              <button
                type="button"
                className={cn(buttonClass('primary', { size: 'sm' }), 'w-auto!')}
                onClick={onPublish}
                disabled={publishing}
              >
                {publishing ? t('knowledge_base.kb_publishing') : t('knowledge_base.kb_publish')}
              </button>
            )
          )}
        </div>
      </div>

      <div className="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-5">
        {editing ? (
          <MarkdownEditor
            value={draft}
            onChange={setDraft}
            folder={file.folder}
            ariaLabel={t('knowledge_base.kb_content_label')}
          />
        ) : mode === 'rendered' ? (
          <div className={markdownBodyClass}>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{body}</ReactMarkdown>
          </div>
        ) : (
          <MarkdownEditor
            value={file.content_md}
            readOnly
            folder={file.folder}
            ariaLabel={t('knowledge_base.kb_source')}
          />
        )}
      </div>
    </section>
  )
}
