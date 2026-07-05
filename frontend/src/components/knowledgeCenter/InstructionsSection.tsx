import { useCallback, useEffect, useMemo, useState } from 'react'

import { useApi } from '../../hooks/useApi'
import { useConfirm } from '../../hooks/useConfirm'
import { useDatasources } from '../../hooks/useDatasources'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass, cardHeaderRowClass, cardIntroClass, cardLeadClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { formRowClass, legacyFormClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { EmptyState } from '../ui/EmptyState'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'

interface Instruction {
  id: string
  datasource_id: string
  model_id?: string
  title: string
  body_md: string
  is_active: boolean
  created_at: string
  updated_at: string
}

interface InstructionForm {
  title: string
  body_md: string
  is_active: boolean
}

const EMPTY_FORM: InstructionForm = { title: '', body_md: '', is_active: true }

/**
 * InstructionsSection manages free-form, markdown business rules ("instructions")
 * for a datasource. They are injected into the text-to-SQL prompt as a
 * "Business Rules" block. Net-new UI for the Knowledge Center (SP1).
 */
export function InstructionsSection() {
  const t = useT()
  const { get, postData, putData, deleteData, loading: apiLoading, error: apiError } = useApi()
  const confirm = useConfirm()
  const { datasources } = useDatasources()

  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const datasourceId = useMemo(() => {
    if (selectedDatasourceId && datasources.some((d) => d.id === selectedDatasourceId)) {
      return selectedDatasourceId
    }
    return datasources[0]?.id ?? ''
  }, [selectedDatasourceId, datasources])

  const [initLoading, setInitLoading] = useState(true)
  const [instructions, setInstructions] = useState<Instruction[]>([])
  // editingId: undefined = form closed, null = creating new, string = editing that id.
  const [editingId, setEditingId] = useState<string | null | undefined>(undefined)
  const [form, setForm] = useState<InstructionForm>(EMPTY_FORM)
  const [formError, setFormError] = useState<string | null>(null)

  const fetchInstructions = useCallback(
    async (dsId: string) => {
      if (!dsId) {
        setInstructions([])
        setInitLoading(false)
        return
      }
      setInitLoading(true)
      try {
        const data = await get<{ instructions: Instruction[] }>(
          `/api/ai/instructions?datasource_id=${encodeURIComponent(dsId)}`,
        )
        if (data) {
          setInstructions(data.instructions)
        }
      } finally {
        setInitLoading(false)
      }
    },
    [get],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void fetchInstructions(datasourceId)
  }, [datasourceId, fetchInstructions])

  const closeForm = useCallback(() => {
    setEditingId(undefined)
    setForm(EMPTY_FORM)
    setFormError(null)
  }, [])

  const openNew = () => {
    setForm(EMPTY_FORM)
    setFormError(null)
    setEditingId(null)
  }

  const openEdit = (inst: Instruction) => {
    setForm({ title: inst.title, body_md: inst.body_md, is_active: inst.is_active })
    setFormError(null)
    setEditingId(inst.id)
  }

  const handleSave = async () => {
    setFormError(null)
    if (!form.title.trim() || !form.body_md.trim()) {
      setFormError(t('knowledge_center.instructions.err_fields_required'))
      return
    }
    if (editingId) {
      const res = await putData(`/api/ai/instructions/${editingId}`, {
        title: form.title.trim(),
        body_md: form.body_md,
        is_active: form.is_active,
      })
      if (res) {
        closeForm()
        void fetchInstructions(datasourceId)
      }
      return
    }
    const res = await postData<{ id: string }>('/api/ai/instructions', {
      datasource_id: datasourceId,
      title: form.title.trim(),
      body_md: form.body_md,
      is_active: form.is_active,
    })
    if (res) {
      closeForm()
      void fetchInstructions(datasourceId)
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await confirm({
      title: t('knowledge_center.instructions.confirm_delete'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    const res = await deleteData(`/api/ai/instructions/${id}`)
    if (res) {
      if (editingId === id) {
        closeForm()
      }
      void fetchInstructions(datasourceId)
    }
  }

  if (initLoading && instructions.length === 0 && datasourceId) {
    return <LoadingScreen minHeight="300px" />
  }

  const formOpen = editingId !== undefined

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cardClass()}>
        <div className={cardIntroClass}>
          <div className={cardHeaderRowClass}>
            <h2>{t('knowledge_center.instructions.title')}</h2>
            <button
              type="button"
              className={buttonClass('primary', { size: 'sm' })}
              onClick={openNew}
              disabled={!datasourceId}
            >
              {t('knowledge_center.instructions.new')}
            </button>
          </div>
          <p className={cardLeadClass}>{t('knowledge_center.instructions.intro')}</p>
        </div>

        <div className={cn(formRowClass, 'mt-5')}>
          <div className={legacyFormClass('form-field')} style={{ minWidth: '14rem' }}>
            <label htmlFor="instructions-datasource" className={legacyFormClass('form-label')}>
              {t('knowledge_center.instructions.label_select_datasource')}
            </label>
            <Select
              id="instructions-datasource"
              value={datasourceId}
              onChange={(id) => {
                setSelectedDatasourceId(id)
                closeForm()
              }}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
        </div>
      </div>

      {apiError && <ErrorAlert error={apiError} />}

      {formOpen && (
        <div className={cardClass()}>
          <div className={cardIntroClass}>
            <h3>
              {editingId
                ? t('knowledge_center.instructions.form_title_edit')
                : t('knowledge_center.instructions.form_title_new')}
            </h3>
          </div>
          {formError && <ErrorAlert error={formError} />}
          <div className="mt-4 flex flex-col gap-3">
            <div className={legacyFormClass('form-field')}>
              <label htmlFor="instruction-title" className={legacyFormClass('form-label')}>
                {t('knowledge_center.instructions.label_title')}
              </label>
              <input
                id="instruction-title"
                value={form.title}
                onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))}
                placeholder={t('knowledge_center.instructions.placeholder_title')}
                autoComplete="off"
              />
            </div>
            <div className={legacyFormClass('form-field')}>
              <label htmlFor="instruction-body" className={legacyFormClass('form-label')}>
                {t('knowledge_center.instructions.label_body')}
              </label>
              <textarea
                id="instruction-body"
                value={form.body_md}
                onChange={(e) => setForm((prev) => ({ ...prev, body_md: e.target.value }))}
                placeholder={t('knowledge_center.instructions.placeholder_body')}
                rows={6}
                className="border-border bg-card-raised text-foreground w-full rounded-lg border px-3 py-2 font-mono text-[0.85rem] leading-[1.45]"
              />
            </div>
            <label className="flex items-center gap-2 text-[0.85rem]">
              <input
                type="checkbox"
                checked={form.is_active}
                onChange={(e) => setForm((prev) => ({ ...prev, is_active: e.target.checked }))}
              />
              {t('knowledge_center.instructions.active_label')}
            </label>
            <div className="flex gap-2">
              <button
                type="button"
                className={buttonClass('primary', { size: 'sm', autoWidth: true })}
                disabled={apiLoading || !form.title.trim() || !form.body_md.trim()}
                onClick={() => void handleSave()}
              >
                {t('knowledge_center.instructions.save')}
              </button>
              <button
                type="button"
                className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
                onClick={closeForm}
              >
                {t('knowledge_center.instructions.cancel')}
              </button>
            </div>
          </div>
        </div>
      )}

      {instructions.length === 0 ? (
        <div className={cardClass()} style={{ position: 'relative', minHeight: '160px' }}>
          <LoadingOverlay loading={apiLoading} />
          <EmptyState
            description={
              datasourceId
                ? t('knowledge_center.instructions.empty')
                : t('knowledge_center.instructions.select_datasource_first')
            }
            action={
              datasourceId
                ? { label: t('knowledge_center.instructions.empty_cta'), onClick: openNew }
                : undefined
            }
          />
        </div>
      ) : (
        <div className={cardClass()} style={{ position: 'relative' }}>
          <LoadingOverlay loading={apiLoading} />
          <ul className="m-0 flex list-none flex-col gap-3 p-0">
            {instructions.map((inst) => (
              <li key={inst.id} className="border-border bg-card-raised rounded-lg border p-3">
                <div className="flex items-start justify-between gap-2">
                  <span className="text-foreground text-[0.95rem] font-semibold">
                    {inst.title}
                    {!inst.is_active && ` — ${t('knowledge_center.instructions.inactive_badge')}`}
                  </span>
                  <div className="flex shrink-0 gap-2">
                    <button
                      type="button"
                      className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
                      onClick={() => openEdit(inst)}
                    >
                      {t('knowledge_center.instructions.edit')}
                    </button>
                    <button
                      type="button"
                      className={buttonClass('danger-outline', { size: 'sm', autoWidth: true })}
                      onClick={() => void handleDelete(inst.id)}
                    >
                      {t('knowledge_center.instructions.delete')}
                    </button>
                  </div>
                </div>
                <p className="text-foreground-muted mt-2 mb-0 text-[0.85rem] leading-[1.45] whitespace-pre-wrap">
                  {inst.body_md}
                </p>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
