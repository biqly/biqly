import { useState } from 'react'

import { draftKnowledgeFile } from '../../api/knowledge'
import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { formLabelClass } from '../../lib/formClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import { Modal } from '../ui/Modal'
import { KNOWLEDGE_FOLDERS } from './knowledgeTree'

interface NewFileModalProps {
  open: boolean
  datasourceId: string
  creating: boolean
  onClose: () => void
  onCreate: (path: string, contentMD: string) => void
  t: TFunction
}

// Starter skeletons per folder so a hand-written file begins with valid
// frontmatter for the publish extraction.
const TEMPLATES: Record<string, string> = {
  glossary:
    '---\ntype: glossary\nterm: \naliases: []\ndescription: \n---\n\n# term\n\nDefinition.\n\n## Usage notes\n',
  instructions:
    '---\ntype: instruction\ntitle: \ndescription: \n---\n\n# Rule\n\nDescribe the rule the AI must follow.\n',
  metrics:
    '---\ntype: metric\ntitle: \ndescription: \n---\n\n# Metric\n\nDefinition, unit, grain and calculation steps.\n',
  'sql-pairs':
    '---\ntype: sql-pair\nquestion: \ndescription: \n---\n\n# Example\n\n```sql\nSELECT ...\n```\n',
}

export function NewFileModal({
  open,
  datasourceId,
  creating,
  onClose,
  onCreate,
  t,
}: NewFileModalProps) {
  const [folder, setFolder] = useState<string>('instructions')
  const [path, setPath] = useState('')
  const [content, setContent] = useState(TEMPLATES.instructions ?? '')
  const [aiPrompt, setAiPrompt] = useState('')
  const [drafting, setDrafting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const applyFolder = (next: string) => {
    setFolder(next)
    setContent((prev) => {
      const isUntouchedTemplate = Object.values(TEMPLATES).includes(prev) || prev.trim() === ''
      return isUntouchedTemplate ? (TEMPLATES[next] ?? '') : prev
    })
  }

  const handleDraft = async () => {
    if (!aiPrompt.trim() || drafting) {
      return
    }
    setDrafting(true)
    setError(null)
    try {
      const draft = await draftKnowledgeFile({
        datasource_id: datasourceId,
        folder,
        prompt: aiPrompt.trim(),
      })
      setContent(draft.content_md)
      if (!path.trim()) {
        setPath(draft.path)
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('common.unknown_error'))
    } finally {
      setDrafting(false)
    }
  }

  const effectivePath = path.trim() || (folder ? `${folder}/new-file.md` : 'new-file.md')

  return (
    <Modal
      open={open}
      title={t('knowledge_base.kb_new_file_title')}
      onClose={onClose}
      className="max-w-2xl"
    >
      <div className="flex flex-col gap-4">
        <div className="border-border bg-card-raised flex flex-col gap-2 rounded-lg border p-3">
          <span className={formLabelClass}>{t('knowledge_base.kb_ai_section')}</span>
          <textarea
            value={aiPrompt}
            onChange={(e) => setAiPrompt(e.target.value)}
            placeholder={t('knowledge_base.kb_ai_prompt_placeholder')}
            aria-label={t('knowledge_base.kb_ai_section')}
            rows={2}
            className="border-border bg-card w-full resize-y rounded-md border p-2 text-[0.82rem]"
          />
          <div className="flex items-center gap-3">
            <button
              type="button"
              className={cn(buttonClass('secondary', { size: 'sm' }), 'w-auto!')}
              onClick={() => void handleDraft()}
              disabled={drafting || !aiPrompt.trim() || !datasourceId}
            >
              {drafting ? t('knowledge_base.kb_ai_generating') : t('knowledge_base.kb_ai_generate')}
            </button>
            <span className="text-foreground-muted text-[0.75rem]">
              {t('knowledge_base.kb_ai_hint')}
            </span>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(8rem,auto)_1fr]">
          <div className="flex flex-col gap-1.5">
            <label className={formLabelClass} htmlFor="kb-new-folder">
              {t('knowledge_base.kb_folder_label')}
            </label>
            <select
              id="kb-new-folder"
              className="border-border bg-card-raised h-9 rounded-md border px-2 text-[0.82rem]"
              value={folder}
              onChange={(e) => applyFolder(e.target.value)}
            >
              {KNOWLEDGE_FOLDERS.map((name) => (
                <option key={name} value={name}>
                  {name}/
                </option>
              ))}
              <option value="">{t('knowledge_base.kb_folder_root')}</option>
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label className={formLabelClass} htmlFor="kb-new-path">
              {t('knowledge_base.kb_path_label')}
            </label>
            <input
              id="kb-new-path"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder={folder ? `${folder}/my-rule.md` : 'README.md'}
              className="border-border bg-card-raised h-9 w-full rounded-md border px-2.5 font-mono text-[0.8rem]"
            />
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <label className={formLabelClass} htmlFor="kb-new-content">
            {t('knowledge_base.kb_content_label')}
          </label>
          <textarea
            id="kb-new-content"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            rows={12}
            spellCheck={false}
            className="border-border bg-card-raised w-full resize-y rounded-md border p-3 font-mono text-[0.8rem] leading-normal"
          />
        </div>

        <ErrorAlert error={error} />

        <div className="flex justify-end gap-2">
          <button
            type="button"
            className={cn(buttonClass('ghost', { size: 'sm' }), 'w-auto!')}
            onClick={onClose}
            disabled={creating}
          >
            {t('knowledge_base.kb_cancel')}
          </button>
          <button
            type="button"
            className={cn(buttonClass('primary', { size: 'sm' }), 'w-auto!')}
            onClick={() => onCreate(effectivePath, content)}
            disabled={creating || !content.trim()}
          >
            {creating ? t('knowledge_base.kb_creating') : t('knowledge_base.kb_create')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
