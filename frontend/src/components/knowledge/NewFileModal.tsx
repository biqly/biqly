import { useEffect, useMemo, useRef, useState } from 'react'

import { draftKnowledgeFile } from '../../api/knowledge'
import type { TFunction } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { formLabelClass } from '../../lib/formClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { KNOWLEDGE_FOLDERS } from './knowledgeTree'
import { MarkdownEditor } from './MarkdownEditor'
import { loadNewFileDraft, saveNewFileDraft } from './newFileDraftStorage'

interface NewFileModalProps {
  open: boolean
  datasourceId: string
  /** Virtual folders created in the tree that have no files yet. */
  extraFolders?: string[]
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
  extraFolders = [],
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
  const [draftElapsedS, setDraftElapsedS] = useState(0)
  const [restored, setRestored] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const draftAbortRef = useRef<AbortController | null>(null)

  // Restore a persisted draft when the modal opens; abort any in-flight
  // generation when it closes or unmounts.
  useEffect(() => {
    if (!open) {
      draftAbortRef.current?.abort()
      return
    }
    const persisted = loadNewFileDraft(datasourceId)
    if (persisted && (persisted.content.trim() || persisted.aiPrompt.trim())) {
      /* eslint-disable react-hooks/set-state-in-effect */
      setFolder(persisted.folder)
      setPath(persisted.path)
      setAiPrompt(persisted.aiPrompt)
      setContent(persisted.content || (TEMPLATES[persisted.folder] ?? ''))
      setRestored(true)
      /* eslint-enable react-hooks/set-state-in-effect */
    }
    return () => draftAbortRef.current?.abort()
  }, [open, datasourceId])

  useEffect(() => {
    if (!open) {
      return
    }
    saveNewFileDraft(datasourceId, { folder, path, aiPrompt, content })
  }, [open, datasourceId, folder, path, aiPrompt, content])

  useEffect(() => {
    if (!drafting) {
      return
    }
    const startedAt = Date.now()
    const id = window.setInterval(() => {
      setDraftElapsedS(Math.round((Date.now() - startedAt) / 1000))
    }, 500)
    return () => window.clearInterval(id)
  }, [drafting])

  const folderOptions = useMemo(() => {
    const names = [...new Set([...KNOWLEDGE_FOLDERS, ...extraFolders])].sort()
    return [
      ...names.map((name) => ({ value: name, label: `${name}/` })),
      { value: '', label: t('knowledge_base.kb_folder_root') },
    ]
  }, [extraFolders, t])

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
    const controller = new AbortController()
    draftAbortRef.current = controller
    setDrafting(true)
    setDraftElapsedS(0)
    setError(null)
    try {
      const draft = await draftKnowledgeFile(
        { datasource_id: datasourceId, folder, prompt: aiPrompt.trim() },
        controller.signal,
      )
      setContent(draft.content_md)
      if (!path.trim()) {
        setPath(draft.path)
      }
    } catch (err: unknown) {
      if (!controller.signal.aborted) {
        setError(err instanceof Error ? err.message : t('common.unknown_error'))
      }
    } finally {
      if (draftAbortRef.current === controller) {
        draftAbortRef.current = null
      }
      setDrafting(false)
    }
  }

  const cancelDraft = () => {
    draftAbortRef.current?.abort()
    draftAbortRef.current = null
    setDrafting(false)
  }

  const effectivePath = path.trim() || (folder ? `${folder}/new-file.md` : 'new-file.md')

  return (
    <Modal
      open={open}
      title={t('knowledge_base.kb_new_file_title')}
      onClose={onClose}
      className="max-w-2xl"
      closeOnBackdrop={!drafting}
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
            disabled={drafting}
            className="border-border bg-card w-full resize-y rounded-md border p-2 text-[0.82rem]"
          />
          <div className="flex flex-wrap items-center gap-3">
            {drafting ? (
              <>
                <span
                  className="text-accent inline-flex items-center gap-2 text-[0.8rem] font-medium"
                  role="status"
                  aria-live="polite"
                >
                  <span className="border-accent inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-t-transparent" />
                  {t('knowledge_base.kb_ai_generating')}
                  <span className="text-foreground-muted tabular-nums">
                    {t('knowledge_base.kb_ai_elapsed', { s: draftElapsedS })}
                  </span>
                </span>
                <button
                  type="button"
                  className={cn(buttonClass('ghost', { size: 'sm' }), 'w-auto!')}
                  onClick={cancelDraft}
                >
                  {t('knowledge_base.kb_cancel')}
                </button>
              </>
            ) : (
              <>
                <button
                  type="button"
                  className={cn(buttonClass('secondary', { size: 'sm' }), 'w-auto!')}
                  onClick={() => void handleDraft()}
                  disabled={!aiPrompt.trim() || !datasourceId}
                >
                  {t('knowledge_base.kb_ai_generate')}
                </button>
                <span className="text-foreground-muted text-[0.75rem]">
                  {t('knowledge_base.kb_ai_hint')}
                </span>
              </>
            )}
          </div>
          {restored && !drafting && (
            <p className="text-foreground-faint m-0 text-[0.72rem]">
              {t('knowledge_base.kb_draft_restored')}
            </p>
          )}
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(8rem,auto)_1fr]">
          <div className="flex flex-col gap-1.5">
            <label className={formLabelClass} htmlFor="kb-new-folder">
              {t('knowledge_base.kb_folder_label')}
            </label>
            <Select
              id="kb-new-folder"
              value={folder}
              onChange={applyFolder}
              options={folderOptions}
              ariaLabel={t('knowledge_base.kb_folder_label')}
            />
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
          <span className={formLabelClass}>{t('knowledge_base.kb_content_label')}</span>
          <div className="border-border bg-card-raised max-h-[45vh] overflow-y-auto rounded-md border">
            <MarkdownEditor
              value={content}
              onChange={setContent}
              readOnly={drafting}
              folder={folder}
              ariaLabel={t('knowledge_base.kb_content_label')}
              minHeight="14rem"
            />
          </div>
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
            disabled={creating || drafting || !content.trim()}
          >
            {creating ? t('knowledge_base.kb_creating') : t('knowledge_base.kb_create')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
