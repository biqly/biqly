import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  backfillKnowledge,
  createKnowledgeFile,
  deleteKnowledgeFile,
  getKnowledgeFile,
  type KnowledgeFile,
  type KnowledgeFileMeta,
  listKnowledgeFiles,
  publishKnowledgeFile,
  updateKnowledgeFile,
} from '../../api/knowledge'
import { useConfirm } from '../../hooks/useConfirm'
import { useDatasources } from '../../hooks/useDatasources'
import { useQueryParam } from '../../hooks/useQueryParam'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { adminAlertSuccessClass } from '../admin/adminClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import { FileMetaPanel } from './FileMetaPanel'
import { FileTree } from './FileTree'
import { FileView } from './FileView'
import { buildKnowledgeTree } from './knowledgeTree'
import { NewFileModal } from './NewFileModal'

// KnowledgeBasePage is the WrenAI-style markdown knowledge base: a file tree
// of datasource-scoped .md documents (glossary/, instructions/, metrics/,
// sql-pairs/), a rendered/source viewer with inline editing, and a meta panel
// showing the frontmatter the AI routes on. Publishing extracts structured
// records into the existing prompt stores; agents also read published files
// directly through the knowledge tools.
export function KnowledgeBasePage() {
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeLanguageTag(locale)
  const confirm = useConfirm()
  const { datasources } = useDatasources()
  const [dsParam, setDsParam] = useQueryParam('ds')
  const datasourceId = useMemo(() => {
    if (dsParam && datasources.some((d) => d.id === dsParam)) {
      return dsParam
    }
    return datasources[0]?.id ?? ''
  }, [dsParam, datasources])

  const [files, setFiles] = useState<KnowledgeFileMeta[]>([])
  const [selected, setSelected] = useState<KnowledgeFile | null>(null)
  const [search, setSearch] = useState('')
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [creating, setCreating] = useState(false)
  const [backfilling, setBackfilling] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const refresh = useCallback(async (): Promise<KnowledgeFileMeta[]> => {
    if (!datasourceId) {
      return []
    }
    try {
      const list = await listKnowledgeFiles(datasourceId)
      setFiles(list)
      return list
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('common.unknown_error'))
      return []
    }
  }, [datasourceId, t])

  useEffect(() => {
    // Datasource switch resets the whole surface before the async reload.
    /* eslint-disable react-hooks/set-state-in-effect */
    setFiles([])
    setSelected(null)
    setEditing(false)
    setError(null)
    /* eslint-enable react-hooks/set-state-in-effect */
    void refresh()
  }, [refresh])

  const openFile = useCallback(
    async (meta: KnowledgeFileMeta) => {
      setError(null)
      setEditing(false)
      try {
        setSelected(await getKnowledgeFile(meta.id))
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : t('common.unknown_error'))
      }
    },
    [t],
  )

  const runAction = useCallback(
    async (setBusy: (busy: boolean) => void, action: () => Promise<void>): Promise<void> => {
      setBusy(true)
      setError(null)
      setMessage(null)
      try {
        await action()
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : t('common.unknown_error'))
      } finally {
        setBusy(false)
      }
    },
    [t],
  )

  const handleSave = (content: string) =>
    runAction(setSaving, async () => {
      if (!selected) {
        return
      }
      await updateKnowledgeFile(selected.id, { path: selected.path, content_md: content })
      setEditing(false)
      setSelected(await getKnowledgeFile(selected.id))
      await refresh()
      setMessage(t('knowledge_base.kb_saved_msg'))
    })

  const handlePublish = () =>
    runAction(setPublishing, async () => {
      if (!selected) {
        return
      }
      await publishKnowledgeFile(selected.id)
      setSelected(await getKnowledgeFile(selected.id))
      await refresh()
      setMessage(t('knowledge_base.kb_published_msg'))
    })

  const handleCreate = (path: string, content: string) =>
    runAction(setCreating, async () => {
      const id = await createKnowledgeFile({
        datasource_id: datasourceId,
        path,
        content_md: content,
      })
      setModalOpen(false)
      await refresh()
      setSelected(await getKnowledgeFile(id))
    })

  const handleDelete = async () => {
    if (!selected) {
      return
    }
    const ok = await confirm({
      title: t('knowledge_base.kb_delete_confirm'),
      variant: 'danger',
      confirmLabel: t('knowledge_base.kb_delete'),
    })
    if (!ok) {
      return
    }
    await runAction(setSaving, async () => {
      await deleteKnowledgeFile(selected.id)
      setSelected(null)
      await refresh()
    })
  }

  const handleBackfill = () =>
    runAction(setBackfilling, async () => {
      const created = await backfillKnowledge(datasourceId)
      await refresh()
      setMessage(t('knowledge_base.kb_backfill_done', { count: created }))
    })

  const tree = useMemo(() => buildKnowledgeTree(files, search), [files, search])

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <p className="text-foreground-muted m-0 flex-1 text-[0.85rem]">
          {t('knowledge_base.kb_intro')}
        </p>
        <label className="text-foreground-muted flex items-center gap-2 text-[0.8rem]">
          {t('knowledge_base.kb_datasource')}
          <select
            className="border-border bg-card-raised h-9 rounded-md border px-2 text-[0.82rem]"
            value={datasourceId}
            onChange={(e) => setDsParam(e.target.value)}
          >
            {datasources.map((ds) => (
              <option key={ds.id} value={ds.id}>
                {ds.name}
              </option>
            ))}
          </select>
        </label>
      </div>

      <ErrorAlert error={error} />
      {message && (
        <div className={adminAlertSuccessClass}>
          <div>{message}</div>
        </div>
      )}

      <div className="grid min-h-[32rem] grid-cols-1 items-start gap-4 lg:grid-cols-[15rem_minmax(0,1fr)_16rem]">
        <div className="border-border bg-card flex max-h-[calc(100vh-16rem)] min-h-[24rem] flex-col rounded-xl border p-3">
          <FileTree
            tree={tree}
            totalCount={files.length}
            selectedId={selected?.id ?? null}
            search={search}
            onSearchChange={setSearch}
            onSelect={(meta) => void openFile(meta)}
            onNewFile={() => setModalOpen(true)}
            onBackfill={files.length === 0 ? () => void handleBackfill() : null}
            backfilling={backfilling}
            t={t}
          />
        </div>

        {selected ? (
          <>
            <FileView
              file={selected}
              editing={editing}
              saving={saving}
              publishing={publishing}
              onCancelEdit={() => setEditing(false)}
              onSave={(content) => void handleSave(content)}
              onPublish={() => void handlePublish()}
              t={t}
            />
            <FileMetaPanel
              file={selected}
              localeTag={localeTag}
              onEdit={() => setEditing(true)}
              onDelete={() => void handleDelete()}
              t={t}
            />
          </>
        ) : (
          <div className="text-foreground-muted border-border bg-card flex min-h-[24rem] items-center justify-center rounded-xl border text-[0.85rem] lg:col-span-2">
            {t('knowledge_base.kb_select_file')}
          </div>
        )}
      </div>

      <NewFileModal
        open={modalOpen}
        datasourceId={datasourceId}
        creating={creating}
        onClose={() => setModalOpen(false)}
        onCreate={(path, content) => void handleCreate(path, content)}
        t={t}
      />
    </div>
  )
}
