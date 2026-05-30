import { useCallback, useEffect, useMemo, useState } from 'react'
import type { LogicalQuery } from '../types/ai'
import type { Datasource } from '../types/metadata'
import { useT } from '../i18n'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'
import { ResultTable } from './ResultTable'
import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'

interface SavedQuestion {
  id: string
  name: string
  description: string
  datasource_id: string
  model_id?: string
  question: string
  logical_query: Record<string, unknown>
  tags: string[]
  dialect: string
  locale?: string
  is_few_shot: boolean
  created_at?: string
  updated_at?: string
}

interface SavedQuestionSemanticModel {
  id: string
  name: string
  label?: string | null
  status: string
}

const DIALECTS = ['postgresql', 'mysql', 'sqlserver', 'clickhouse']
const LOCALES = ['', 'en', 'tr']

interface SavedQuestionFormModalProps {
  mode: 'new' | 'edit'
  open: boolean
  title: string
  formError: string | null
  datasources: Datasource[]
  semanticModels: SavedQuestionSemanticModel[]
  datasourceId: string
  modelId: string
  name: string
  description: string
  question: string
  logicalQuery: string
  tags: string
  dialect: string
  locale: string
  isFewShot: boolean
  onDatasourceChange: (value: string) => void
  onModelChange: (value: string) => void
  onNameChange: (value: string) => void
  onDescriptionChange: (value: string) => void
  onQuestionChange: (value: string) => void
  onLogicalQueryChange: (value: string) => void
  onTagsChange: (value: string) => void
  onDialectChange: (value: string) => void
  onLocaleChange: (value: string) => void
  onIsFewShotChange: (value: boolean) => void
  onClose: () => void
  onSave: () => void
  t: ReturnType<typeof useT>
}

function SavedQuestionFormModal({
  mode,
  open,
  title,
  formError,
  datasources,
  semanticModels,
  datasourceId,
  modelId,
  name,
  description,
  question,
  logicalQuery,
  tags,
  dialect,
  locale,
  isFewShot,
  onDatasourceChange,
  onModelChange,
  onNameChange,
  onDescriptionChange,
  onQuestionChange,
  onLogicalQueryChange,
  onTagsChange,
  onDialectChange,
  onLocaleChange,
  onIsFewShotChange,
  onClose,
  onSave,
  t,
}: SavedQuestionFormModalProps) {
  const id = (field: string) => `${mode}-${field}`

  return (
    <Modal open={open} title={title} onClose={onClose}>
      <div className="form-stack">
        {formError && <ErrorAlert error={formError} />}

        <div className="form-group">
          <label htmlFor={id('ds')}>{t('saved_questions.label_select_datasource')}</label>
          <Select
            id={id('ds')}
            value={datasourceId}
            onChange={onDatasourceChange}
            options={datasources.map((d) => ({ value: d.id, label: d.name }))}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('model')}>{t('saved_questions.label_select_model')}</label>
          <Select
            id={id('model')}
            value={modelId}
            onChange={onModelChange}
            options={[
              { value: '', label: t('saved_questions.label_all_models') },
              ...semanticModels.map((m) => ({ value: m.id, label: m.label || m.name })),
            ]}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('name')}>{t('saved_questions.label_name')}</label>
          <input
            id={id('name')}
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
            placeholder="e.g. Sales by region"
            autoComplete="off"
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('desc')}>{t('saved_questions.label_description')}</label>
          <textarea
            id={id('desc')}
            value={description}
            onChange={(e) => onDescriptionChange(e.target.value)}
            placeholder="e.g. Shows regional breakdown for orders"
            rows={2}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('question')}>{t('saved_questions.label_question')}</label>
          <textarea
            id={id('question')}
            value={question}
            onChange={(e) => onQuestionChange(e.target.value)}
            placeholder="e.g. ne kadar sipariş aldık ülkelere göre?"
            rows={2}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('lq')}>{t('saved_questions.label_logical_query')}</label>
          <textarea
            id={id('lq')}
            value={logicalQuery}
            onChange={(e) => onLogicalQueryChange(e.target.value)}
            placeholder='{ "select": ... }'
            rows={6}
            style={{ fontFamily: 'monospace' }}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('tags')}>{t('saved_questions.label_tags')}</label>
          <input
            id={id('tags')}
            value={tags}
            onChange={(e) => onTagsChange(e.target.value)}
            placeholder="sales, region"
            autoComplete="off"
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('dialect')}>{t('saved_questions.label_dialect')}</label>
          <Select
            id={id('dialect')}
            value={dialect}
            onChange={onDialectChange}
            options={DIALECTS.map((d) => ({ value: d, label: d }))}
          />
        </div>

        <div className="form-group">
          <label htmlFor={id('locale')}>{t('saved_questions.label_locale')}</label>
          <Select
            id={id('locale')}
            value={locale}
            onChange={onLocaleChange}
            options={LOCALES.map((l) => ({ value: l, label: l || 'Default' }))}
          />
        </div>

        <div className="form-group" style={{ flexDirection: 'row', alignItems: 'center', gap: '0.5rem' }}>
          <input
            type="checkbox"
            id={id('is-few-shot')}
            checked={isFewShot}
            onChange={(e) => onIsFewShotChange(e.target.checked)}
          />
          <label htmlFor={id('is-few-shot')} style={{ margin: 0, cursor: 'pointer' }}>
            {t('saved_questions.label_is_few_shot')}
          </label>
        </div>

        <div className="modal-actions" style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', marginTop: '1rem' }}>
          <button type="button" className="btn btn--neutral" onClick={onClose}>
            {t('saved_questions.btn_cancel')}
          </button>
          <button type="button" className="btn btn-primary" onClick={onSave}>
            {t('saved_questions.btn_save')}
          </button>
        </div>
      </div>
    </Modal>
  )
}

export default function SavedQuestions() {
  const t = useT()
  const { get, postData, putData, deleteData, loading: apiLoading, error: apiError } = useApi()
  const confirm = useConfirm()

  // Selectors State
  const { datasources } = useDatasources()
  const [datasourceId, setDatasourceId] = useState('')
  const { models: semanticModels, setModels: setSemanticModels } = useSemanticModels(datasourceId)
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
  const [formDatasourceId, setFormDatasourceId] = useState('')
  const [formModelId, setFormModelId] = useState('')
  const [formName, setFormName] = useState('')
  const [formDescription, setFormDescription] = useState('')
  const [formQuestion, setFormQuestion] = useState('')
  const [formLq, setFormLq] = useState('')
  const [formTags, setFormTags] = useState('')
  const [formDialect, setFormDialect] = useState('postgresql')
  const [formLocale, setFormLocale] = useState('')
  const [formIsFewShot, setFormIsFewShot] = useState(true)

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

      setFormName(prefQuestion)
      setFormQuestion(prefQuestion)
      setFormLq(prefLq)
      setFormDescription('')
      setFormTags('')
      setFormDialect('postgresql')
      setFormLocale('')
      setFormIsFewShot(true)
      setFormDatasourceId(prefDs)
      setFormModelId(prefModel)

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
    } catch (err: any) {
      setRunError(err?.message || 'Execution failed')
    } finally {
      setRunLoading(false)
    }
  }

  const openAdd = () => {
    setFormName('')
    setFormDescription('')
    setFormQuestion('')
    setFormLq('')
    setFormTags('')
    setFormDialect('postgresql')
    setFormLocale('')
    setFormIsFewShot(true)
    setFormDatasourceId(datasourceId)
    setFormModelId(semanticModelId)
    setFormError(null)
    setIsNewModalOpen(true)
  }

  const openEdit = (q: SavedQuestion) => {
    setFormName(q.name)
    setFormDescription(q.description)
    setFormQuestion(q.question)
    setFormLq(JSON.stringify(q.logical_query, null, 2))
    setFormTags(q.tags.join(', '))
    setFormDialect(q.dialect)
    setFormLocale(q.locale || '')
    setFormIsFewShot(q.is_few_shot)
    setFormDatasourceId(q.datasource_id)
    setFormModelId(q.model_id || '')
    setFormError(null)
    setIsEditModalOpen(true)
  }

  const handleSave = async (isEdit: boolean) => {
    setFormError(null)
    if (!formName.trim() || !formQuestion.trim() || !formLq.trim()) {
      setFormError(t('saved_questions.err_fields_required'))
      return
    }

    let parsedLq: Record<string, unknown>
    try {
      parsedLq = JSON.parse(formLq)
    } catch {
      setFormError(t('saved_questions.validation_error_json'))
      return
    }

    const payload = {
      datasource_id: formDatasourceId,
      model_id: formModelId || undefined,
      question: formQuestion,
      logical_query: parsedLq,
      tags: formTags.split(',').map((t) => t.trim()).filter(Boolean),
      dialect: formDialect,
      locale: formLocale || undefined,
      name: formName,
      description: formDescription,
      is_few_shot: formIsFewShot,
    }

    if (isEdit && selectedQuestion) {
      const res = await putData<any>(`/api/ai/examples/${selectedQuestion.id}`, payload)
      if (res || apiError === null) {
        fetchQuestions(datasourceId, semanticModelId)
        setIsEditModalOpen(false)
        setSelectedQuestion({
          ...selectedQuestion,
          name: formName,
          description: formDescription,
          question: formQuestion,
          logical_query: parsedLq,
          tags: payload.tags,
          dialect: formDialect,
          locale: formLocale,
          is_few_shot: formIsFewShot,
          datasource_id: formDatasourceId,
          model_id: formModelId,
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
            {selectedQuestion ? (
              <div>
                <h2>{selectedQuestion.name}</h2>
                {selectedQuestion.description && (
                  <p className="saved-question-description" style={{ marginTop: '0.5rem' }}>
                    {selectedQuestion.description}
                  </p>
                )}

                <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap', margin: '1rem 0' }}>
                  {selectedQuestion.model_id && (
                    <span className="tag-pill" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
                      <strong>{t('saved_questions.label_select_model')}:</strong>
                      <code>{semanticModels.find((m) => m.id === selectedQuestion.model_id)?.label || selectedQuestion.model_id}</code>
                    </span>
                  )}
                  <span className="tag-pill" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
                    <strong>{t('saved_questions.label_dialect')}:</strong>
                    <code>{selectedQuestion.dialect}</code>
                  </span>
                  {selectedQuestion.locale && (
                    <span className="tag-pill" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.35rem' }}>
                      <strong>{t('saved_questions.label_locale')}:</strong>
                      <code>{selectedQuestion.locale}</code>
                    </span>
                  )}
                </div>

                <h3 style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>{t('saved_questions.label_question')}</h3>
                <p style={{ background: 'var(--bg-card-raised)', padding: '0.75rem 1rem', borderRadius: '0.35rem', fontStyle: 'italic' }}>
                  {selectedQuestion.question}
                </p>

                <h3 style={{ marginTop: '1.5rem', marginBottom: '0.5rem' }}>{t('saved_questions.logical_query_heading')}</h3>
                <pre className="sql-preview" style={{ maxHeight: '250px', overflowY: 'auto' }}>
                  {JSON.stringify(selectedQuestion.logical_query, null, 2)}
                </pre>

                <div className="saved-question-actions">
                  <button
                    type="button"
                    className="btn btn-primary"
                    onClick={() => runQuery(selectedQuestion.logical_query)}
                    disabled={runLoading}
                    aria-label={t('saved_questions.aria_run_query')}
                  >
                    {t('saved_questions.run_query')}
                  </button>
                  <button
                    type="button"
                    className="btn"
                    onClick={() => openEdit(selectedQuestion)}
                    aria-label={t('saved_questions.aria_edit_query')}
                  >
                    {t('saved_questions.edit_query')}
                  </button>
                  <button
                    type="button"
                    className="btn btn-danger"
                    onClick={() => handleDelete(selectedQuestion.id)}
                    aria-label={t('saved_questions.aria_delete_query')}
                  >
                    {t('saved_questions.delete_query')}
                  </button>
                </div>

                {/* Inline query execution results */}
                {runLoading && (
                  <div style={{ marginTop: '1.5rem', display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 100 }}>
                    <LoadingOverlay loading={true} />
                  </div>
                )}

                {runError && (
                  <div style={{ marginTop: '1.5rem' }}>
                    <ErrorAlert error={runError} />
                  </div>
                )}

                {runResult && runResult.columns && runResult.rows && (
                  <div className="results-section" style={{ marginTop: '1.5rem' }}>
                    <ResultTable
                      columns={runResult.columns}
                      rows={runResult.rows}
                      rowCount={runResult.rows.length}
                      durationMs={runResult.stats?.duration_ms}
                    />
                  </div>
                )}
              </div>
            ) : (
              <EmptyState description={t('saved_questions.select_hint')} />
            )}
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
        datasourceId={formDatasourceId}
        modelId={formModelId}
        name={formName}
        description={formDescription}
        question={formQuestion}
        logicalQuery={formLq}
        tags={formTags}
        dialect={formDialect}
        locale={formLocale}
        isFewShot={formIsFewShot}
        onDatasourceChange={setFormDatasourceId}
        onModelChange={setFormModelId}
        onNameChange={setFormName}
        onDescriptionChange={setFormDescription}
        onQuestionChange={setFormQuestion}
        onLogicalQueryChange={setFormLq}
        onTagsChange={setFormTags}
        onDialectChange={setFormDialect}
        onLocaleChange={setFormLocale}
        onIsFewShotChange={setFormIsFewShot}
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
        datasourceId={formDatasourceId}
        modelId={formModelId}
        name={formName}
        description={formDescription}
        question={formQuestion}
        logicalQuery={formLq}
        tags={formTags}
        dialect={formDialect}
        locale={formLocale}
        isFewShot={formIsFewShot}
        onDatasourceChange={setFormDatasourceId}
        onModelChange={setFormModelId}
        onNameChange={setFormName}
        onDescriptionChange={setFormDescription}
        onQuestionChange={setFormQuestion}
        onLogicalQueryChange={setFormLq}
        onTagsChange={setFormTags}
        onDialectChange={setFormDialect}
        onLocaleChange={setFormLocale}
        onIsFewShotChange={setFormIsFewShot}
        onClose={() => setIsEditModalOpen(false)}
        onSave={() => handleSave(true)}
        t={t}
      />
    </div>
  )
}
