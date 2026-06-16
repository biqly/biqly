import { useCallback, useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useToast } from '../hooks/useToast'
import { useT } from '../i18n'
import { savedQuestionTagsClass, tagPillClass } from '../lib/badgeClasses'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  fewshotCheckboxClass,
  savedQuestionFavClass,
  savedQuestionItemClass,
  savedQuestionItemMetaPClass,
  savedQuestionItemTitleClass,
  savedQuestionItemTopClass,
  savedQuestionListClass,
  savedQuestionRowClass,
} from '../lib/savedQuestionClasses'
import type { QueryResultPayload } from '../types/ai'
import { pickValidId, pickValidIdOrFirst } from '../utils/effectiveSelection'
import { parseJsonRecord } from '../utils/record'
import { QuestionDetailPane } from './savedQuestions/QuestionDetailPane'
import { SavedQuestionFormModal } from './savedQuestions/SavedQuestionFormModal'
import type { SavedQuestion, SavedQuestionFormState } from './savedQuestions/types'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'

export default function SavedQuestions() {
  const t = useT()
  const toast = useToast()
  const { get, postData, putData, deleteData, loading: apiLoading, error: apiError } = useApi()
  const confirm = useConfirm()

  // Selectors State
  const { datasources } = useDatasources()
  const [prefill] = useState(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.get('prefill') !== '1') {
      return null
    }
    return {
      datasourceId: params.get('datasource_id') ?? '',
      modelId: params.get('model_id') ?? '',
      question: params.get('question') ?? '',
      logicalQuery: params.get('logical_query') ?? '',
    }
  })
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(
    () => prefill?.datasourceId ?? '',
  )
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const { models: semanticModels } = useSemanticModels(datasourceId)
  const [selectedSemanticModelId, setSelectedSemanticModelId] = useState(
    () => prefill?.modelId ?? '',
  )
  const semanticModelId = useMemo(
    () => pickValidId(selectedSemanticModelId, semanticModels),
    [selectedSemanticModelId, semanticModels],
  )

  const [initLoading, setInitLoading] = useState(true)

  // Questions List State
  const [questions, setQuestions] = useState<SavedQuestion[]>([])
  const [search, setSearch] = useState('')
  const [selectedQuestion, setSelectedQuestion] = useState<SavedQuestion | null>(null)

  // Modal State
  const [isNewModalOpen, setIsNewModalOpen] = useState(() => prefill !== null)
  const [isEditModalOpen, setIsEditModalOpen] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  // Form Fields State
  const [form, setForm] = useState<SavedQuestionFormState>(() => {
    if (!prefill) {
      return {
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
      }
    }
    return {
      datasourceId: prefill.datasourceId,
      modelId: prefill.modelId,
      name: prefill.question,
      description: '',
      question: prefill.question,
      logicalQuery: prefill.logicalQuery,
      tags: '',
      dialect: 'postgresql',
      locale: '',
      isFewShot: true,
    }
  })

  const handleFormChange = (patch: Partial<SavedQuestionFormState>) => {
    setForm((prev) => ({ ...prev, ...patch }))
  }

  // Fetch Saved Questions
  const fetchQuestions = useCallback(
    async (dsId: string, mId: string) => {
      if (!dsId) {
        return
      }
      setInitLoading(true)
      try {
        const url = `/api/ai/examples?datasource_id=${encodeURIComponent(dsId)}${mId ? `&model_id=${encodeURIComponent(mId)}` : ''}`
        const data = await get<SavedQuestion[]>(url)
        if (data) {
          setQuestions(data)
        }
      } finally {
        setInitLoading(false)
      }
    },
    [get],
  )

  useEffect(() => {
    if (prefill) {
      window.history.replaceState(null, '', window.location.pathname)
    }
  }, [prefill])

  const questionsScopeKey = `${datasourceId}:${semanticModelId}`
  const [runState, setRunState] = useState<{
    scopeKey: string
    questionId: string | null
    loading: boolean
    error: string | null
    result: {
      columns: { name: string; type?: string }[]
      rows: unknown[][]
      stats?: { duration_ms?: number }
    } | null
  }>({ scopeKey: '', questionId: null, loading: false, error: null, result: null })

  const runLoading =
    runState.scopeKey === questionsScopeKey &&
    runState.questionId === (selectedQuestion?.id ?? null) &&
    runState.loading
  const runError =
    runState.scopeKey === questionsScopeKey &&
    runState.questionId === (selectedQuestion?.id ?? null)
      ? runState.error
      : null
  const runResult =
    runState.scopeKey === questionsScopeKey &&
    runState.questionId === (selectedQuestion?.id ?? null)
      ? runState.result
      : null

  // Reload questions when selected DS or Model changes
  useEffect(() => {
    if (datasourceId) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      void fetchQuestions(datasourceId, semanticModelId)
    }
  }, [datasourceId, semanticModelId, fetchQuestions])

  // Filter list by search term
  const filtered = useMemo(() => {
    const term = search.toLowerCase().trim()
    return questions.filter((q) => {
      if (!term) {
        return true
      }
      return (
        q.name.toLowerCase().includes(term) ||
        q.question.toLowerCase().includes(term) ||
        q.description.toLowerCase().includes(term) ||
        q.tags.some((t) => t.toLowerCase().includes(term))
      )
    })
  }, [questions, search])

  const selectQuestion = useCallback(
    (question: SavedQuestion | null) => {
      setSelectedQuestion(question)
      setRunState({
        scopeKey: questionsScopeKey,
        questionId: question?.id ?? null,
        loading: false,
        error: null,
        result: null,
      })
    },
    [questionsScopeKey],
  )

  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      setSelectedSemanticModelId('')
      selectQuestion(null)
    },
    [selectQuestion],
  )

  const setSemanticModelId = useCallback(
    (id: string) => {
      setSelectedSemanticModelId(id)
      selectQuestion(null)
    },
    [selectQuestion],
  )

  // Toggle IsFewShot checkbox state immediately
  const toggleFewShot = async (q: SavedQuestion) => {
    const updatedIsFewShot = !q.is_few_shot
    // Immediate UI update
    setQuestions((prev) =>
      prev.map((item) => (item.id === q.id ? { ...item, is_few_shot: updatedIsFewShot } : item)),
    )
    if (selectedQuestion?.id === q.id) {
      setSelectedQuestion((prev) => (prev ? { ...prev, is_few_shot: updatedIsFewShot } : null))
    }

    try {
      const payload = {
        question: q.question,
        logical_query: q.logical_query,
        tags: q.tags,
        dialect: q.dialect,
        locale: q.locale ?? '',
        name: q.name,
        description: q.description,
        is_few_shot: updatedIsFewShot,
        is_favorite: q.is_favorite ?? false,
      }
      await putData(`/api/ai/examples/${q.id}`, payload)
    } catch {
      // Revert on failure
      setQuestions((prev) =>
        prev.map((item) => (item.id === q.id ? { ...item, is_few_shot: q.is_few_shot } : item)),
      )
      if (selectedQuestion?.id === q.id) {
        setSelectedQuestion((prev) => (prev ? { ...prev, is_few_shot: q.is_few_shot } : null))
      }
      toast.error(t('saved_questions.toggle_fewshot_error'))
    }
  }

  // Toggle is_favorite state immediately (optimistic), reverting on failure.
  const toggleFavorite = async (q: SavedQuestion) => {
    const updated = !q.is_favorite
    setQuestions((prev) =>
      prev.map((item) => (item.id === q.id ? { ...item, is_favorite: updated } : item)),
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
        locale: q.locale ?? '',
        name: q.name,
        description: q.description,
        is_few_shot: q.is_few_shot,
        is_favorite: updated,
      })
    } catch {
      setQuestions((prev) =>
        prev.map((item) => (item.id === q.id ? { ...item, is_favorite: q.is_favorite } : item)),
      )
      if (selectedQuestion?.id === q.id) {
        setSelectedQuestion((prev) => (prev ? { ...prev, is_favorite: q.is_favorite } : null))
      }
      toast.error(t('saved_questions.favorite_toggle_error'))
    }
  }

  // Inline Execute
  const runQuery = async (logicalQuery: Record<string, unknown>) => {
    const questionId = selectedQuestion?.id ?? null
    setRunState({
      scopeKey: questionsScopeKey,
      questionId,
      loading: true,
      error: null,
      result: null,
    })
    try {
      const res = await postData<QueryResultPayload>('/api/query/run', logicalQuery)
      if (res) {
        setRunState({
          scopeKey: questionsScopeKey,
          questionId,
          loading: false,
          error: null,
          result: res,
        })
      } else {
        setRunState({
          scopeKey: questionsScopeKey,
          questionId,
          loading: false,
          error: 'Failed to run query',
          result: null,
        })
      }
    } catch (err: unknown) {
      setRunState({
        scopeKey: questionsScopeKey,
        questionId,
        loading: false,
        error: err instanceof Error ? err.message : 'Execution failed',
        result: null,
      })
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
      modelId: q.model_id ?? '',
      name: q.name,
      description: q.description,
      question: q.question,
      logicalQuery: JSON.stringify(q.logical_query, null, 2),
      tags: q.tags.join(', '),
      dialect: q.dialect,
      locale: q.locale ?? '',
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

    const parsedLq = parseJsonRecord(form.logicalQuery)
    if (!parsedLq) {
      setFormError(t('saved_questions.validation_error_json'))
      return
    }

    const payload = {
      datasource_id: form.datasourceId,
      model_id: form.modelId || undefined,
      question: form.question,
      logical_query: parsedLq,
      tags: form.tags
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
      dialect: form.dialect,
      locale: form.locale || undefined,
      name: form.name,
      description: form.description,
      is_few_shot: form.isFewShot,
      is_favorite: isEdit ? (selectedQuestion?.is_favorite ?? false) : false,
    }

    if (isEdit && selectedQuestion) {
      const res = await putData<SavedQuestion>(`/api/ai/examples/${selectedQuestion.id}`, payload)
      if (res || apiError === null) {
        void fetchQuestions(datasourceId, semanticModelId)
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
        void fetchQuestions(datasourceId, semanticModelId)
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
    if (!ok) {
      return
    }
    const res = await deleteData(`/api/ai/examples/${id}`)
    if (res || apiError === null) {
      setSelectedQuestion(null)
      void fetchQuestions(datasourceId, semanticModelId)
    }
  }

  const fewShotCount = useMemo(() => questions.filter((q) => q.is_few_shot).length, [questions])

  if (initLoading && questions.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={legacyCardClass('card')}>
        <div className={legacyCardClass('card-intro')}>
          <div className={legacyCardClass('card-header-row card-header-row--spaced')}>
            <h2>{t('saved_questions.title')}</h2>
            <button
              type="button"
              className={legacyButtonClass('btn btn-primary btn-sm')}
              onClick={openAdd}
            >
              {t('saved_questions.new')}
            </button>
          </div>
          <p
            className={legacyCardClass('card-lead saved-question-intro card-lead--single-line')}
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
        <div className={cn(formRowClass, 'mt-5')}>
          <div className={legacyFormClass('form-field')} style={{ minWidth: '14rem' }}>
            <label htmlFor="library-datasource" className={legacyFormClass('form-label')}>
              {t('saved_questions.label_select_datasource')}
            </label>
            <Select
              id="library-datasource"
              value={datasourceId}
              onChange={setDatasourceId}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className={legacyFormClass('form-field')} style={{ minWidth: '14rem' }}>
            <label htmlFor="library-model" className={legacyFormClass('form-label')}>
              {t('saved_questions.label_select_model')}
            </label>
            <Select
              id="library-model"
              value={semanticModelId}
              onChange={setSemanticModelId}
              options={[
                { value: '', label: t('saved_questions.label_all_models') },
                ...semanticModels.map((m) => ({
                  value: m.id,
                  label: m.label ?? m.name,
                  hint: m.status,
                })),
              ]}
            />
          </div>
          <div className={legacyFormClass('form-field')} style={{ flexGrow: 1, minWidth: '16rem' }}>
            <label htmlFor="library-search" className={legacyFormClass('form-label')}>
              {t('common.search')}
            </label>
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
        <div
          className={legacyCardClass('card')}
          style={{
            position: 'relative',
            minHeight: '300px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <LoadingOverlay loading={apiLoading} />
          <EmptyState
            description={
              search.trim() ? t('saved_questions.no_matches') : t('saved_questions.empty')
            }
          />
        </div>
      ) : (
        <div className={savedQuestionListClass()}>
          {/* Left Column: Questions List */}
          <div className={legacyCardClass('card')} style={{ position: 'relative' }}>
            <LoadingOverlay loading={apiLoading} />
            <div className="flex flex-col gap-3">
              {filtered.map((q) => {
                const checked = q.is_few_shot
                const isSelected = selectedQuestion?.id === q.id
                return (
                  <div key={q.id} className={savedQuestionRowClass()}>
                    <button
                      type="button"
                      className={savedQuestionItemClass(isSelected)}
                      onClick={() => selectQuestion(q)}
                    >
                      <div className={savedQuestionItemTopClass()}>
                        <h3 className={savedQuestionItemTitleClass()}>{q.name}</h3>
                        <label
                          className={fewshotCheckboxClass(true, checked)}
                          title={t('saved_questions.fewshot_use_title')}
                          onClick={(e) => e.stopPropagation()}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => {
                              void toggleFewShot(q)
                            }}
                            onClick={(e) => e.stopPropagation()}
                            aria-label={t('saved_questions.fewshot_aria', { name: q.name })}
                          />
                          <span>{t('saved_questions.fewshot_badge')}</span>
                        </label>
                      </div>
                      {q.description && (
                        <p className={savedQuestionItemMetaPClass()}>{q.description}</p>
                      )}
                      <div className={savedQuestionTagsClass}>
                        {q.tags.map((tag) => (
                          <span key={tag} className={tagPillClass}>
                            {tag}
                          </span>
                        ))}
                      </div>
                    </button>
                    <button
                      type="button"
                      className={savedQuestionFavClass(q.is_favorite)}
                      onClick={(e) => {
                        e.stopPropagation()
                        void toggleFavorite(q)
                      }}
                      aria-label={
                        q.is_favorite
                          ? t('saved_questions.favorite_remove')
                          : t('saved_questions.favorite_add')
                      }
                      title={
                        q.is_favorite
                          ? t('saved_questions.favorite_remove')
                          : t('saved_questions.favorite_add')
                      }
                    >
                      {q.is_favorite ? '★' : '☆'}
                    </button>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Right Column: Details & Run pane */}
          <div className={legacyCardClass('card')} style={{ position: 'relative' }}>
            <QuestionDetailPane
              selectedQuestion={selectedQuestion}
              semanticModels={semanticModels}
              runLoading={runLoading}
              runError={runError}
              runResult={runResult}
              onRun={(logicalQuery) => {
                void runQuery(logicalQuery)
              }}
              onOpenEdit={openEdit}
              onDelete={(id) => {
                void handleDelete(id)
              }}
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
        onSave={() => {
          void handleSave(false)
        }}
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
        onSave={() => {
          void handleSave(true)
        }}
        t={t}
      />
    </div>
  )
}
