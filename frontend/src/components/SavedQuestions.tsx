import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Datasource } from '../types/metadata'
import { useT } from '../i18n'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { Select } from './ui/Select'
import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import type { SavedQuestion, SavedQuestionSemanticModel, SavedQuestionFormState } from './savedQuestions/types'
import { SavedQuestionFormModal } from './savedQuestions/SavedQuestionFormModal'
import { QuestionDetailPane } from './savedQuestions/QuestionDetailPane'

export default function SavedQuestions() {
  const t = useT()
  const { get, postData, putData, deleteData, loading: apiLoading, error: apiError } = useApi()
  const confirm = useConfirm()

  // Selectors State
  const { datasources } = useDatasources()
  const [datasourceId, setDatasourceId] = useState('')
  const { models: semanticModels } = useSemanticModels(datasourceId)
  const [semanticModelId, setSemanticModelId] = useState('')

  // Questions List State
  const [questions, setQuestions] = useState<SavedQuestion[]>([])
  const [search, setSearch] = useState('')
  const [selectedQuestion, setSelectedQuestion] = useState<SavedQuestion | null>(null)

  // Modal State
  const [isNewModalOpen, setIsNewModalOpen] = useState(false)
  const [isEditModalOpen, setIsEditModalOpen] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  // Form Fields State
  const [form, setForm] = useState<SavedQuestionFormState>({
    datasourceId: '',
    modelId: '',
    name: '',
    description: '',
    question: '',
    logicalQuery: '',
    tags: '',
    dialect: 'postgresql',
    locale: '',
    isFewShot: true,
  })

  const handleFormChange = (patch: Partial<SavedQuestionFormState>) => {
    setForm((prev) => ({ ...prev, ...patch }))
  }

  // Inline execution state
  const [runLoading, setRunLoading] = useState(false)
  const [runError, setRunError] = useState<string | null>(null)
  const [runResult, setRunResult] = useState<{
    columns: { name: string; type?: string }[]
    rows: unknown[][]
    stats?: { duration_ms?: number }
  } | null>(null)

  // Fetch Saved Questions
  const fetchQuestions = useCallback(async (dsId: string, mId: string) => {
    if (!dsId) return
    const url = `/api/ai/examples?datasource_id=${encodeURIComponent(dsId)}${mId ? `&model_id=${encodeURIComponent(mId)}` : ''}`
    const data = await get<SavedQuestion[]>(url)
    if (data) {
      setQuestions(data)
    }
  }, [get])

  // Set default datasourceId
  useEffect(() => {
    if (datasources.length > 0 && !datasourceId) {
      setDatasourceId(datasources[0]!.id)
    }
  }, [datasources, datasourceId])

  // Set default semanticModelId when semanticModels change
  useEffect(() => {
    setSemanticModelId((prev) => {
      if (prev && semanticModels.some((m) => m.id === prev)) {
        return prev
      }
      return ''
    })
  }, [semanticModels])

  // Reload questions when selected DS or Model changes
  useEffect(() => {
    if (datasourceId) {
      fetchQuestions(datasourceId, semanticModelId)
      setRunResult(null)
      setRunError(null)
      setRunLoading(false)
    }
  }, [datasourceId, semanticModelId, fetchQuestions])

  // Handle URL Prefill
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const isPrefill = params.get('prefill') === '1'
    if (isPrefill) {
      const prefQuestion = params.get('question') || ''
      const prefLq = params.get('logical_query') || ''
      const prefDs = params.get('datasource_id') || ''
      const prefModel = params.get('model_id') || ''

      if (prefDs) {
        setDatasourceId(prefDs)
        if (prefModel) {
          setSemanticModelId(prefModel)
        }
      }

      setForm({
        name: prefQuestion,
        question: prefQuestion,
        logicalQuery: prefLq,
        description: '',
        tags: '',
        dialect: 'postgresql',
        locale: '',
        isFewShot: true,
        datasourceId: prefDs,
        modelId: prefModel,
      })

      setIsNewModalOpen(true)
      window.history.replaceState(null, '', window.location.pathname)
    }
  }, [])

  // Filter list by search term
  const filtered = useMemo(() => {
    const term = search.toLowerCase().trim()
    return questions.filter((q) => {
      if (!term) return true
      return (
        q.name.toLowerCase().includes(term) ||
        q.question.toLowerCase().includes(term) ||
        q.description.toLowerCase().includes(term) ||
        q.tags.some((t) => t.toLowerCase().includes(term))
      )
    })
  }, [questions, search])

  // Reset Run State when selected question changes
  useEffect(() => {
    setRunResult(null)
    setRunError(null)
    setRunLoading(false)
  }, [selectedQuestion])

  // Toggle IsFewShot checkbox state immediately
  const toggleFewShot = async (q: SavedQuestion) => {
    const updatedIsFewShot = !q.is_few_shot
    // Immediate UI update
    setQuestions((prev) =>
      prev.map((item) => (item.id === q.id ? { ...item, is_few_shot: updatedIsFewShot } : item))
    )
    if (selectedQuestion?.id === q.id) {
      setSelectedQuestion((prev) => prev ? { ...prev, is_few_shot: updatedIsFewShot } : null)
    }

    try {
      const payload = {
        question: q.question,
        logical_query: q.logical_query,
        tags: q.tags,
        dialect: q.dialect,
        locale: q.locale || '',
        name: q.name,
        description: q.description,
        is_few_shot: updatedIsFewShot,
        is_favorite: q.is_favorite ?? false,
      }
      await putData(`/api/ai/examples/${q.id}`, payload)
    } catch {
      // Revert on failure
      setQuestions((prev) =>
        prev.map((item) => (item.id === q.id ? { ...item, is_few_shot: q.is_few_shot } : item))
      )
      if (selectedQuestion?.id === q.id) {
        setSelectedQuestion((prev) => prev ? { ...prev, is_few_shot: q.is_few_shot } : null)
      }
    }
  }

  // Toggle is_favorite state immediately (optimistic), reverting on failure.
  const toggleFavorite = async (q: SavedQuestion) => {
    const updated = !q.is_favorite
    setQuestions((prev) =>
      prev.map((item) => (item.id === q.id ? { ...item, is_favorite: updated } : item))
    )
    if (selectedQuestion?.id === q.id) {
      setSelectedQuestion((prev) => (prev ? { ...prev, is_favorite: updated } : null))
    }
    try {
      await putData(`/api/ai/examples/${q.id}`, {
        question: q.question,
        logical_query: q.logical_query,
        tags: q.tags,
        dialect: q.dialect,
        locale: q.locale || '',
        name: q.name,
        description: q.description,
        is_few_shot: q.is_few_shot,
        is_favorite: updated,
      })
    } catch {
      setQuestions((prev) =>
        prev.map((item) => (item.id === q.id ? { ...item, is_favorite: q.is_favorite } : item))
      )
      if (selectedQuestion?.id === q.id) {
        setSelectedQuestion((prev) => (prev ? { ...prev, is_favorite: q.is_favorite } : null))
      }
    }
  }

  // Inline Execute
  const runQuery = async (logicalQuery: Record<string, unknown>) => {
    setRunLoading(true)
    setRunError(null)
    setRunResult(null)
    try {
      const res = await postData<any>('/api/query/run', logicalQuery)
      if (res) {
        setRunResult(res)
      } else {
        setRunError('Failed to run query')
      }
    } catch (err: unknown) {
      setRunError(err instanceof Error ? err.message : 'Execution failed')
    } finally {
      setRunLoading(false)
    }
  }

  const openAdd = () => {
    setForm({
      datasourceId,
      modelId: semanticModelId,
      name: '',
      description: '',
      question: '',
      logicalQuery: '',
      tags: '',
      dialect: 'postgresql',
      locale: '',
      isFewShot: true,
    })
    setFormError(null)
    setIsNewModalOpen(true)
  }

  const openEdit = (q: SavedQuestion) => {
    setForm({
      datasourceId: q.datasource_id,
      modelId: q.model_id || '',
      name: q.name,
      description: q.description,
      question: q.question,
      logicalQuery: JSON.stringify(q.logical_query, null, 2),
      tags: q.tags.join(', '),
      dialect: q.dialect,
      locale: q.locale || '',
      isFewShot: q.is_few_shot,
    })
    setFormError(null)
    setIsEditModalOpen(true)
  }

  const handleSave = async (isEdit: boolean) => {
    setFormError(null)
    if (!form.name.trim() || !form.question.trim() || !form.logicalQuery.trim()) {
      setFormError(t('saved_questions.err_fields_required'))
      return
    }

    let parsedLq: Record<string, unknown>
    try {
      parsedLq = JSON.parse(form.logicalQuery)
    } catch {
      setFormError(t('saved_questions.validation_error_json'))
      return
    }

    const payload = {
      datasource_id: form.datasourceId,
      model_id: form.modelId || undefined,
      question: form.question,
      logical_query: parsedLq,
      tags: form.tags.split(',').map((t) => t.trim()).filter(Boolean),
      dialect: form.dialect,
      locale: form.locale || undefined,
      name: form.name,
      description: form.description,
      is_few_shot: form.isFewShot,
      is_favorite: isEdit ? (selectedQuestion?.is_favorite ?? false) : false,
    }

    if (isEdit && selectedQuestion) {
      const res = await putData<any>(`/api/ai/examples/${selectedQuestion.id}`, payload)
      if (res || apiError === null) {
        fetchQuestions(datasourceId, semanticModelId)
        setIsEditModalOpen(false)
        setSelectedQuestion({
          ...selectedQuestion,
          name: form.name,
          description: form.description,
          question: form.question,
          logical_query: parsedLq,
          tags: payload.tags,
          dialect: form.dialect,
          locale: form.locale,
          is_few_shot: form.isFewShot,
          datasource_id: form.datasourceId,
          model_id: form.modelId,
        })
      }
    } else {
      const res = await postData<SavedQuestion>('/api/ai/examples', payload)
      if (res) {
        fetchQuestions(datasourceId, semanticModelId)
        setIsNewModalOpen(false)
        // Automatically select the newly created question
        setSelectedQuestion(res)
      }
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await confirm({
      title: t('saved_questions.confirm_delete'),
      variant: 'danger',
    })
    if (!ok) return
    const res = await deleteData(`/api/ai/examples/${id}`)
    if (res || apiError === null) {
      setSelectedQuestion(null)
      fetchQuestions(datasourceId, semanticModelId)
    }
  }

  const fewShotCount = useMemo(() => questions.filter((q) => q.is_few_shot).length, [questions])

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-intro">
          <div className="card-header-row card-header-row--spaced">
            <h2>{t('saved_questions.title')}</h2>
            <button type="button" className="btn btn-primary btn-sm" onClick={openAdd}>
              {t('saved_questions.new')}
            </button>
          </div>
          <p
            className="card-lead saved-question-intro card-lead--single-line"
            title={
              fewShotCount > 0
                ? t('saved_questions.intro_fewshot_active', { count: fewShotCount })
                : t('saved_questions.intro_fewshot_none')
            }
          >
            {fewShotCount > 0
              ? t('saved_questions.intro_fewshot_active', { count: fewShotCount })
              : t('saved_questions.intro_fewshot_none')}
          </p>
        </div>

        {/* Filters Top Bar */}
        <div className="form-row" style={{ marginTop: '1.25rem', flexWrap: 'wrap' }}>
          <div className="form-field" style={{ minWidth: '14rem' }}>
            <label htmlFor="library-datasource" className="form-label">{t('saved_questions.label_select_datasource')}</label>
            <Select
              id="library-datasource"
              value={datasourceId}
              onChange={setDatasourceId}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className="form-field" style={{ minWidth: '14rem' }}>
            <label htmlFor="library-model" className="form-label">{t('saved_questions.label_select_model')}</label>
            <Select
              id="library-model"
              value={semanticModelId}
              onChange={setSemanticModelId}
              options={[
                { value: '', label: t('saved_questions.label_all_models') },
                ...semanticModels.map((m) => ({ value: m.id, label: m.label || m.name, hint: m.status }))
              ]}
            />
          </div>
          <div className="form-field" style={{ flexGrow: 1, minWidth: '16rem' }}>
            <label htmlFor="library-search" className="form-label">{t('common.search')}</label>
            <input
              id="library-search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('saved_questions.search_placeholder')}
              autoComplete="off"
            />
          </div>
        </div>
      </div>

      {apiError && <ErrorAlert error={apiError} />}

      {filtered.length === 0 ? (
        <div className="card" style={{ position: 'relative', minHeight: '300px', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <LoadingOverlay loading={apiLoading} />
          <EmptyState
            description={search.trim() ? t('saved_questions.no_matches') : t('saved_questions.empty')}
          />
        </div>
      ) : (
        <div className="saved-question-list">
          {/* Left Column: Questions List */}
          <div className="card" style={{ position: 'relative' }}>
            <LoadingOverlay loading={apiLoading} />
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
              {filtered.map((q) => {
                const checked = q.is_few_shot
                const isSelected = selectedQuestion?.id === q.id
                return (
                  <div key={q.id} className="saved-question-row">
                    <button
                      type="button"
                      className="saved-question-item"
                      style={{
                        borderColor: isSelected ? 'var(--accent)' : undefined,
                        background: isSelected ? 'var(--bg-card-raised)' : undefined,
                      }}
                      onClick={() => setSelectedQuestion(q)}
                    >
                      <h3>{q.name}</h3>
                      {q.description && <p>{q.description}</p>}
                      <div className="saved-question-tags">
                        {q.tags.map((tag) => (
                          <span key={tag} className="tag-pill">{tag}</span>
                        ))}
                      </div>
                    </button>
                    <label
                      className={`fewshot-checkbox${checked ? ' is-active' : ''}`}
                      title={t('saved_questions.fewshot_use_title')}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleFewShot(q)}
                        onClick={(e) => e.stopPropagation()}
                        aria-label={t('saved_questions.fewshot_aria', { name: q.name })}
                      />
                      <span>{t('saved_questions.fewshot_badge')}</span>
                    </label>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Right Column: Details & Run pane */}
          <div className="card" style={{ position: 'relative' }}>
            <QuestionDetailPane
              selectedQuestion={selectedQuestion}
              semanticModels={semanticModels}
              runLoading={runLoading}
              runError={runError}
              runResult={runResult}
              onRun={runQuery}
              onOpenEdit={openEdit}
              onDelete={handleDelete}
              t={t}
            />
          </div>
        </div>
      )}

      <SavedQuestionFormModal
        mode="new"
        open={isNewModalOpen}
        title={t('saved_questions.modal_title_new')}
        formError={formError}
        datasources={datasources}
        semanticModels={semanticModels}
        form={form}
        onChange={handleFormChange}
        onClose={() => setIsNewModalOpen(false)}
        onSave={() => handleSave(false)}
        t={t}
      />

      <SavedQuestionFormModal
        mode="edit"
        open={isEditModalOpen}
        title={t('saved_questions.modal_title_edit')}
        formError={formError}
        datasources={datasources}
        semanticModels={semanticModels}
        form={form}
        onChange={handleFormChange}
        onClose={() => setIsEditModalOpen(false)}
        onSave={() => handleSave(true)}
        t={t}
      />
    </div>
  )
}
