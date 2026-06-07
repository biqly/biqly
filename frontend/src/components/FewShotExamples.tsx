import { useEffect, useMemo, useRef, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'

interface FewShotExample {
  id: string
  datasource_id: string
  model_id?: string
  question: string
  logical_query: Record<string, unknown>
  tags: string[]
  dialect: string
  created_at?: string
}

const DIALECTS = ['postgresql', 'mysql', 'bigquery', 'snowflake', 'duckdb']

export default function FewShotExamples() {
  const t = useT()
  const confirm = useConfirm()
  const { get, postData, putData, deleteData, loading, error } = useApi()
  const [examples, setExamples] = useState<FewShotExample[]>([])

  // Filtering & Metadata States
  const { datasources } = useDatasources()
  const { models: allModels } = useSemanticModels(null, { all: true })
  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')

  // Form States
  const [showForm, setShowForm] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [formDatasourceId, setFormDatasourceId] = useState('')
  const [formModelId, setFormModelId] = useState('')
  const [formQuestion, setFormQuestion] = useState('')
  const [formLq, setFormLq] = useState('')
  const [formTags, setFormTags] = useState('')
  const [formDialect, setFormDialect] = useState('postgresql')
  const [formError, setFormError] = useState<string | null>(null)
  const [apiReady, setApiReady] = useState(true)

  // Sidebar States & Refs
  const { model: activeModelDetail, setModel: setActiveModelDetail } = useModelDetail(formModelId, {
    includeInactive: true,
  })
  const [sidebarSearch, setSidebarSearch] = useState('')
  const questionRef = useRef<HTMLTextAreaElement | null>(null)
  const lqRef = useRef<HTMLTextAreaElement | null>(null)
  const [lastFocusedInput, setLastFocusedInput] = useState<'question' | 'lq'>('lq')

  // Load & Filter Examples
  const [initLoading, setInitLoading] = useState(true)

  useEffect(() => {
    let url = '/api/ai/examples'
    const queryParts: string[] = []
    if (selectedDatasourceId) {
      queryParts.push(`datasource_id=${encodeURIComponent(selectedDatasourceId)}`)
      if (selectedModelId && selectedModelId !== 'raw_tables') {
        queryParts.push(`model_id=${encodeURIComponent(selectedModelId)}`)
      }
    }
    if (queryParts.length > 0) {
      url += `?${queryParts.join('&')}`
    }

    setInitLoading(true)
    get<FewShotExample[]>(url)
      .then((data) => {
        if (data) {
          setExamples(data)
          setApiReady(true)
        } else {
          // Fallback to offline local storage
          try {
            let local = JSON.parse(
              localStorage.getItem('biqly_fewshot') ?? '[]',
            ) as FewShotExample[]
            if (selectedDatasourceId) {
              local = local.filter((ex) => ex.datasource_id === selectedDatasourceId)
              if (selectedModelId) {
                if (selectedModelId === 'raw_tables') {
                  local = local.filter((ex) => !ex.model_id)
                } else {
                  local = local.filter((ex) => ex.model_id === selectedModelId)
                }
              }
            }
            setExamples(local)
          } catch {
            /* empty */
          }
          setApiReady(false)
        }
      })
      .finally(() => {
        setInitLoading(false)
      })
  }, [selectedDatasourceId, selectedModelId, get])

  const persist = (updated: FewShotExample[]) => {
    setExamples(updated)
    if (!apiReady) {
      localStorage.setItem('biqly_fewshot', JSON.stringify(updated))
    }
  }

  const resetForm = () => {
    setFormQuestion('')
    setFormLq('')
    setFormTags('')
    setFormDialect('postgresql')
    setFormDatasourceId('')
    setFormModelId('')
    setEditId(null)
    setFormError(null)
    setShowForm(false)
    setActiveModelDetail(null)
    setSidebarSearch('')
  }

  const openAdd = () => {
    setFormQuestion('')
    setFormLq('')
    setFormTags('')
    setFormDialect('postgresql')
    setFormDatasourceId(selectedDatasourceId || (datasources[0]?.id ?? ''))
    setFormModelId(selectedModelId && selectedModelId !== 'raw_tables' ? selectedModelId : '')
    setEditId(null)
    setFormError(null)
    setShowForm(true)
    setActiveModelDetail(null)
    setSidebarSearch('')
  }

  const openEdit = (ex: FewShotExample) => {
    setEditId(ex.id)
    setFormQuestion(ex.question)
    setFormLq(JSON.stringify(ex.logical_query, null, 2))
    setFormTags(ex.tags.join(', '))
    setFormDialect(ex.dialect)
    setFormDatasourceId(ex.datasource_id)
    setFormModelId(ex.model_id ?? '')
    setFormError(null)
    setShowForm(true)
    setSidebarSearch('')
  }

  const handleSave = async () => {
    setFormError(null)
    let lq: Record<string, unknown>
    try {
      lq = JSON.parse(formLq)
    } catch {
      setFormError(t('few_shot.err_invalid_lq'))
      return
    }
    if (!formQuestion.trim()) {
      setFormError(t('few_shot.err_question_required'))
      return
    }
    if (!formDatasourceId) {
      setFormError(t('few_shot.err_datasource_required'))
      return
    }

    if (editId) {
      // Update
      const existing = examples.find((e) => e.id === editId)
      const updatedItem: FewShotExample = {
        ...existing,
        id: editId,
        datasource_id: formDatasourceId,
        model_id: formModelId || undefined,
        question: formQuestion,
        logical_query: lq,
        tags: formTags
          .split(',')
          .map((tok) => tok.trim())
          .filter(Boolean),
        dialect: formDialect,
      }
      const updated = examples.map((e) => (e.id === editId ? updatedItem : e))
      if (apiReady) {
        await putData(`/api/ai/examples/${editId}`, updatedItem)
      }
      persist(updated)
    } else {
      const newEx: FewShotExample = {
        id: `ex_${Date.now()}`,
        datasource_id: formDatasourceId,
        model_id: formModelId || undefined,
        question: formQuestion,
        logical_query: lq,
        tags: formTags
          .split(',')
          .map((tok) => tok.trim())
          .filter(Boolean),
        dialect: formDialect,
      }
      if (apiReady) {
        const res = await postData<FewShotExample>('/api/ai/examples', newEx)
        if (res) {
          persist([...examples, res])
          resetForm()
          return
        }
        setApiReady(false)
      }
      persist([...examples, newEx])
    }
    resetForm()
  }

  const handleDelete = async (id: string) => {
    const ok = await confirm({
      title: t('few_shot.confirm_delete'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    if (apiReady) {
      await deleteData(`/api/ai/examples/${id}`)
    }
    persist(examples.filter((e) => e.id !== id))
  }

  // Insert field badge at cursor
  const handleInsertField = (fieldName: string) => {
    const inputNode = lastFocusedInput === 'question' ? questionRef.current : lqRef.current
    if (!inputNode) {
      return
    }

    const start = inputNode.selectionStart
    const end = inputNode.selectionEnd
    const text = inputNode.value

    const insertText = lastFocusedInput === 'lq' ? `"${fieldName}"` : fieldName
    const newText = text.substring(0, start) + insertText + text.substring(end)

    if (lastFocusedInput === 'question') {
      setFormQuestion(newText)
    } else {
      setFormLq(newText)
    }

    setTimeout(() => {
      inputNode.focus()
      const newCursorPos = start + insertText.length
      inputNode.setSelectionRange(newCursorPos, newCursorPos)
    }, 0)
  }

  // Filter lists inside main view
  const filterModels = useMemo(() => {
    if (!selectedDatasourceId) {
      return []
    }
    return allModels.filter((m) => m.datasource_id === selectedDatasourceId)
  }, [allModels, selectedDatasourceId])

  // Filter lists inside form
  const formModels = useMemo(() => {
    if (!formDatasourceId) {
      return []
    }
    return allModels.filter((m) => m.datasource_id === formDatasourceId)
  }, [allModels, formDatasourceId])

  // Client side model filtering for "Raw tables" option
  const displayedExamples = useMemo(() => {
    if (selectedModelId === 'raw_tables') {
      return examples.filter((ex) => !ex.model_id)
    }
    return examples
  }, [examples, selectedModelId])

  // Sidebar search filtering
  const filteredDimensions = useMemo(() => {
    if (!activeModelDetail?.dimensions) {
      return []
    }
    const query = sidebarSearch.toLowerCase().trim()
    if (!query) {
      return activeModelDetail.dimensions.filter((d) => d.is_active !== false)
    }
    return activeModelDetail.dimensions.filter(
      (d) =>
        d.is_active !== false &&
        (d.name.toLowerCase().includes(query) || d.label?.toLowerCase().includes(query)),
    )
  }, [activeModelDetail, sidebarSearch])

  const filteredMetrics = useMemo(() => {
    if (!activeModelDetail?.metrics) {
      return []
    }
    const query = sidebarSearch.toLowerCase().trim()
    if (!query) {
      return activeModelDetail.metrics.filter((m) => m.is_active !== false)
    }
    return activeModelDetail.metrics.filter(
      (m) =>
        m.is_active !== false &&
        (m.name.toLowerCase().includes(query) || m.label?.toLowerCase().includes(query)),
    )
  }, [activeModelDetail, sidebarSearch])

  const getDatasourceName = (dsId: string) => {
    return datasources.find((d) => d.id === dsId)?.name ?? dsId
  }

  const getModelName = (modelId: string | undefined) => {
    if (!modelId) {
      return t('few_shot.option_raw_tables')
    }
    const m = allModels.find((model) => model.id === modelId)
    return m ? (m.label ?? m.name) : modelId
  }

  if (initLoading && examples.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className="page-stack">
      {!apiReady && <ErrorAlert error={t('few_shot.api_offline_alert')} />}

      <div className="card">
        <div className="card-intro">
          <div className="card-header-row">
            <h2>{t('few_shot.title')}</h2>
            <button type="button" className="btn btn-sm btn-primary" onClick={openAdd}>
              {t('few_shot.new')}
            </button>
          </div>
          <p className="card-lead card-lead--single-line" title={t('few_shot.manage_hint')}>
            {t('few_shot.manage_hint')}
          </p>
        </div>

        {/* Filters */}
        <div
          className="form-row"
          style={{ gap: '1rem', flexWrap: 'wrap', alignItems: 'flex-end', marginBottom: '1.25rem' }}
        >
          <label className="form-field" style={{ minWidth: '14rem' }}>
            <span className="form-label">{t('few_shot.label_datasource')}</span>
            <Select
              value={selectedDatasourceId}
              options={[
                { value: '', label: t('few_shot.option_all_datasources') },
                ...datasources.map((d) => ({ value: d.id, label: d.name })),
              ]}
              onChange={(v) => {
                setSelectedDatasourceId(v)
                setSelectedModelId('')
              }}
            />
          </label>
          {selectedDatasourceId && (
            <label className="form-field" style={{ minWidth: '14rem' }}>
              <span className="form-label">{t('few_shot.label_model')}</span>
              <Select
                value={selectedModelId}
                options={[
                  { value: '', label: t('few_shot.option_all_models') },
                  { value: 'raw_tables', label: t('few_shot.option_raw_tables') },
                  ...filterModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
                ]}
                onChange={setSelectedModelId}
              />
            </label>
          )}
        </div>

        {displayedExamples.length === 0 && <EmptyState description={t('few_shot.empty')} />}

        {displayedExamples.length > 0 && (
          <div className="table-wrap">
            <table className="results-table">
              <thead>
                <tr>
                  <th>{t('few_shot.col_question')}</th>
                  <th>{t('few_shot.label_datasource')}</th>
                  <th>{t('few_shot.label_model')}</th>
                  <th>{t('few_shot.col_dialect')}</th>
                  <th>{t('few_shot.col_tags')}</th>
                  <th className="actions">{t('few_shot.col_actions')}</th>
                </tr>
              </thead>
              <tbody>
                {displayedExamples.map((ex) => (
                  <tr key={ex.id}>
                    <td
                      style={{
                        maxWidth: 280,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                      title={ex.question}
                    >
                      {ex.question}
                    </td>
                    <td>{getDatasourceName(ex.datasource_id)}</td>
                    <td>{getModelName(ex.model_id)}</td>
                    <td>
                      <code style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>
                        {ex.dialect}
                      </code>
                    </td>
                    <td>
                      {ex.tags.map((tag) => (
                        <span
                          key={tag}
                          style={{
                            display: 'inline-block',
                            padding: '0.15rem 0.5rem',
                            background: 'rgba(96,165,250,0.1)',
                            borderRadius: '0.3rem',
                            fontSize: '0.72rem',
                            marginRight: '0.3rem',
                            color: 'var(--accent)',
                          }}
                        >
                          {tag}
                        </span>
                      ))}
                    </td>
                    <td className="actions">
                      <div className="row-actions">
                        <button
                          type="button"
                          className="btn btn-sm btn-ghost"
                          onClick={() => openEdit(ex)}
                        >
                          {t('common.edit')}
                        </button>
                        <button
                          type="button"
                          className="btn btn-sm btn-danger"
                          onClick={() => handleDelete(ex.id)}
                        >
                          {t('common.delete')}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Form Modal */}
      {showForm && (
        <div className="modal-backdrop" onClick={resetForm}>
          <div className="modal-card modal-card--few-shot" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editId ? t('few_shot.form_edit_title') : t('few_shot.form_add_title')}</h2>
              <button className="modal-close" onClick={resetForm}>
                ×
              </button>
            </div>
            <div className="modal-body modal-body--two-col">
              <div className="few-shot-main-form">
                <div className="modal-form-row">
                  <div className="form-group">
                    <label htmlFor="fs-datasource">{t('few_shot.label_datasource')}</label>
                    <Select
                      id="fs-datasource"
                      value={formDatasourceId}
                      onChange={(val) => {
                        setFormDatasourceId(val)
                        setFormModelId('')
                      }}
                      options={datasources.map((d) => ({ value: d.id, label: d.name }))}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="fs-model">{t('few_shot.label_model')}</label>
                    <Select
                      id="fs-model"
                      value={formModelId}
                      onChange={setFormModelId}
                      options={[
                        { value: '', label: t('few_shot.option_raw_tables') },
                        ...formModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
                      ]}
                    />
                  </div>
                </div>

                <div className="form-group">
                  <label htmlFor="fs-question">{t('few_shot.label_nl_question')}</label>
                  <textarea
                    ref={questionRef}
                    id="fs-question"
                    value={formQuestion}
                    onChange={(e) => setFormQuestion(e.target.value)}
                    onFocus={() => setLastFocusedInput('question')}
                    placeholder={t('few_shot.placeholder_nl_question')}
                    rows={3}
                  />
                </div>

                <div
                  className="form-group"
                  style={{ flex: 1, display: 'flex', flexDirection: 'column' }}
                >
                  <label htmlFor="fs-lq">{t('few_shot.label_lq_json')}</label>
                  <textarea
                    ref={lqRef}
                    id="fs-lq"
                    value={formLq}
                    onChange={(e) => setFormLq(e.target.value)}
                    onFocus={() => setLastFocusedInput('lq')}
                    placeholder='{"select": [{"type": "metric", "name": "revenue"}]}'
                    style={{
                      fontFamily: 'monospace',
                      fontSize: '0.8rem',
                      flex: 1,
                      minHeight: '120px',
                    }}
                  />
                </div>

                <div className="modal-form-row">
                  <div className="form-group">
                    <label htmlFor="fs-tags">{t('few_shot.label_tags')}</label>
                    <input
                      id="fs-tags"
                      value={formTags}
                      onChange={(e) => setFormTags(e.target.value)}
                      placeholder={t('few_shot.placeholder_tags')}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="fs-dialect">{t('few_shot.label_dialect')}</label>
                    <Select
                      id="fs-dialect"
                      value={formDialect}
                      onChange={setFormDialect}
                      options={DIALECTS.map((d) => ({ value: d, label: d }))}
                    />
                  </div>
                </div>

                <ErrorAlert error={formError} />
              </div>

              <div className="few-shot-sidebar">
                <div className="few-shot-sidebar__header">
                  {t('few_shot.available_fields_title')}
                </div>
                {activeModelDetail ? (
                  <>
                    <input
                      type="text"
                      className="input input-sm few-shot-sidebar__search"
                      placeholder={t('few_shot.search_fields_placeholder')}
                      value={sidebarSearch}
                      onChange={(e) => setSidebarSearch(e.target.value)}
                    />
                    <div className="few-shot-sidebar__list">
                      {filteredDimensions.map((d) => (
                        <button
                          key={d.id}
                          type="button"
                          className="field-badge-btn"
                          onClick={() => handleInsertField(d.name)}
                          title={d.description ?? d.label ?? d.name}
                        >
                          <span>{d.name}</span>
                          <span className="field-badge-btn__type">dim</span>
                        </button>
                      ))}
                      {filteredMetrics.map((m) => (
                        <button
                          key={m.id}
                          type="button"
                          className="field-badge-btn"
                          onClick={() => handleInsertField(m.name)}
                          title={m.description ?? m.label ?? m.name}
                        >
                          <span>{m.name}</span>
                          <span className="field-badge-btn__type">met</span>
                        </button>
                      ))}
                      {filteredDimensions.length === 0 && filteredMetrics.length === 0 && (
                        <span
                          style={{
                            fontSize: '0.75rem',
                            color: 'var(--text-secondary)',
                            textAlign: 'center',
                            marginTop: '1rem',
                          }}
                        >
                          No fields found
                        </span>
                      )}
                    </div>
                  </>
                ) : (
                  <div
                    style={{
                      fontSize: '0.75rem',
                      color: 'var(--text-secondary)',
                      lineHeight: '1.4',
                      marginTop: '0.5rem',
                    }}
                  >
                    {t('few_shot.helper_select_model')}
                  </div>
                )}
              </div>
            </div>
            <div className="modal-actions">
              <button type="button" className="btn btn-ghost" onClick={resetForm}>
                {t('common.cancel')}
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={handleSave}
                disabled={loading}
              >
                {loading ? t('common.saving') : t('common.save')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
