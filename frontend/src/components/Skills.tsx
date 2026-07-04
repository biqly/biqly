import { useCallback, useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { cardClass, cardHeaderRowClass, cardIntroClass, cardLeadClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  savedQuestionItemMetaPClass,
  savedQuestionItemTitleClass,
  savedQuestionListClass,
} from '../lib/savedQuestionClasses'
import { pickValidIdOrFirst } from '../utils/effectiveSelection'
import { parseJsonRecord } from '../utils/record'
import { SkillDetailPane } from './skills/SkillDetailPane'
import { SkillFormModal } from './skills/SkillFormModal'
import type { Skill, SkillFormState, SkillParameter, SkillRunResult } from './skills/types'
import { paramDefaultText } from './skills/types'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'

const EMPTY_FORM: SkillFormState = {
  datasourceId: '',
  modelId: '',
  name: '',
  description: '',
  question: '',
  logicalQuery: '',
  parameters: '',
  tags: '',
  isActive: true,
}

function readPrefill(): Partial<SkillFormState> | null {
  const params = new URLSearchParams(window.location.search)
  if (params.get('prefill') !== '1') {
    return null
  }
  return {
    datasourceId: params.get('datasource_id') ?? '',
    modelId: params.get('model_id') ?? '',
    name: params.get('question') ?? '',
    question: params.get('question') ?? '',
    logicalQuery: params.get('logical_query') ?? '',
  }
}

function parseSkillParameters(raw: string): SkillParameter[] | null {
  const trimmed = raw.trim()
  if (!trimmed) {
    return []
  }
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!Array.isArray(parsed)) {
      return null
    }
    return parsed as SkillParameter[]
  } catch {
    return null
  }
}

function initialParamValues(skill: Skill | null): Record<string, string> {
  const values: Record<string, string> = {}
  for (const param of skill?.parameters ?? []) {
    const text = paramDefaultText(param.default)
    if (text !== '') {
      values[param.name] = text
    }
  }
  return values
}

function coerceParamValue(param: SkillParameter, raw: string): unknown {
  if (param.type === 'number') {
    const num = Number(raw)
    return Number.isNaN(num) ? raw : num
  }
  return raw
}

export default function Skills() {
  const t = useT()
  const { get, postData, putData, deleteData, loading: apiLoading, error: apiError } = useApi()
  const confirm = useConfirm()

  const { datasources } = useDatasources()
  const [prefill] = useState(readPrefill)
  const [selectedDatasourceId, setSelectedDatasourceId] = useState(
    () => prefill?.datasourceId ?? '',
  )
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const { models: semanticModels } = useSemanticModels(datasourceId)

  const [initLoading, setInitLoading] = useState(true)
  const [skills, setSkills] = useState<Skill[]>([])
  const [search, setSearch] = useState('')
  const [selectedSkill, setSelectedSkill] = useState<Skill | null>(null)

  const [isNewModalOpen, setIsNewModalOpen] = useState(() => prefill !== null)
  const [isEditModalOpen, setIsEditModalOpen] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [form, setForm] = useState<SkillFormState>(() =>
    prefill ? { ...EMPTY_FORM, ...prefill } : EMPTY_FORM,
  )

  const [paramValues, setParamValues] = useState<Record<string, string>>({})
  const [runState, setRunState] = useState<{
    skillId: string | null
    loading: boolean
    error: string | null
    result: SkillRunResult | null
  }>({ skillId: null, loading: false, error: null, result: null })

  useEffect(() => {
    if (prefill) {
      window.history.replaceState(null, '', window.location.pathname)
    }
  }, [prefill])

  const fetchSkills = useCallback(
    async (dsId: string) => {
      if (!dsId) {
        setInitLoading(false)
        return
      }
      setInitLoading(true)
      try {
        const data = await get<{ skills: Skill[] }>(
          `/api/ai/skills?datasource_id=${encodeURIComponent(dsId)}`,
        )
        if (data) {
          setSkills(data.skills)
        }
      } finally {
        setInitLoading(false)
      }
    },
    [get],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void fetchSkills(datasourceId)
  }, [datasourceId, fetchSkills])

  const filtered = useMemo(() => {
    const term = search.toLowerCase().trim()
    if (!term) {
      return skills
    }
    return skills.filter(
      (s) =>
        s.name.toLowerCase().includes(term) ||
        s.description.toLowerCase().includes(term) ||
        s.question.toLowerCase().includes(term) ||
        s.tags.some((tag) => tag.toLowerCase().includes(term)),
    )
  }, [skills, search])

  const selectSkill = useCallback((skill: Skill | null) => {
    setSelectedSkill(skill)
    setParamValues(initialParamValues(skill))
    setRunState({ skillId: skill?.id ?? null, loading: false, error: null, result: null })
  }, [])

  const setDatasourceId = useCallback(
    (id: string) => {
      setSelectedDatasourceId(id)
      selectSkill(null)
    },
    [selectSkill],
  )

  const runSkill = async () => {
    if (!selectedSkill) {
      return
    }
    const skill = selectedSkill
    const parameters: Record<string, unknown> = {}
    for (const param of skill.parameters) {
      const raw = paramValues[param.name]
      if (raw !== undefined && raw !== '') {
        parameters[param.name] = coerceParamValue(param, raw)
      }
    }
    setRunState({ skillId: skill.id, loading: true, error: null, result: null })
    const res = await postData<SkillRunResult>(`/api/ai/skills/${skill.id}/run`, { parameters })
    if (res) {
      setRunState({ skillId: skill.id, loading: false, error: null, result: res })
    } else {
      setRunState({
        skillId: skill.id,
        loading: false,
        error: t('skills.run_failed'),
        result: null,
      })
    }
  }

  const openAdd = () => {
    setForm({ ...EMPTY_FORM, datasourceId })
    setFormError(null)
    setIsNewModalOpen(true)
  }

  const openEdit = (skill: Skill) => {
    setForm({
      datasourceId: skill.datasource_id,
      modelId: skill.model_id ?? '',
      name: skill.name,
      description: skill.description,
      question: skill.question,
      logicalQuery: skill.logical_query ? JSON.stringify(skill.logical_query, null, 2) : '',
      parameters: skill.parameters.length > 0 ? JSON.stringify(skill.parameters, null, 2) : '',
      tags: skill.tags.join(', '),
      isActive: skill.is_active,
    })
    setFormError(null)
    setIsEditModalOpen(true)
  }

  const handleSave = async (isEdit: boolean) => {
    setFormError(null)
    if (!form.name.trim() || !form.logicalQuery.trim()) {
      setFormError(t('skills.err_fields_required'))
      return
    }
    const parsedLq = parseJsonRecord(form.logicalQuery)
    if (!parsedLq) {
      setFormError(t('skills.validation_error_json'))
      return
    }
    const parsedParams = parseSkillParameters(form.parameters)
    if (parsedParams === null) {
      setFormError(t('skills.validation_error_params'))
      return
    }
    const payload = {
      datasource_id: form.datasourceId,
      model_id: form.modelId || undefined,
      name: form.name.trim(),
      description: form.description,
      question: form.question,
      logical_query: parsedLq,
      parameters: parsedParams,
      tags: form.tags
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean),
      is_active: form.isActive,
    }
    if (isEdit && selectedSkill) {
      const res = await putData(`/api/ai/skills/${selectedSkill.id}`, payload)
      if (res) {
        setIsEditModalOpen(false)
        selectSkill(null)
        void fetchSkills(datasourceId)
      }
    } else {
      const res = await postData<{ id: string }>('/api/ai/skills', payload)
      if (res) {
        setIsNewModalOpen(false)
        void fetchSkills(datasourceId)
      }
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await confirm({ title: t('skills.confirm_delete'), variant: 'danger' })
    if (!ok) {
      return
    }
    const res = await deleteData(`/api/ai/skills/${id}`)
    if (res) {
      selectSkill(null)
      void fetchSkills(datasourceId)
    }
  }

  if (initLoading && skills.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cardClass()}>
        <div className={cardIntroClass}>
          <div className={cardHeaderRowClass}>
            <h2>{t('skills.title')}</h2>
            <button
              type="button"
              className={buttonClass('primary', { size: 'sm' })}
              onClick={openAdd}
            >
              {t('skills.new')}
            </button>
          </div>
          <p className={cardLeadClass}>{t('skills.intro')}</p>
        </div>

        <div className={cn(formRowClass, 'mt-5')}>
          <div className={legacyFormClass('form-field')} style={{ minWidth: '14rem' }}>
            <label htmlFor="skills-datasource" className={legacyFormClass('form-label')}>
              {t('skills.label_select_datasource')}
            </label>
            <Select
              id="skills-datasource"
              value={datasourceId}
              onChange={setDatasourceId}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className={legacyFormClass('form-field')} style={{ flexGrow: 1, minWidth: '16rem' }}>
            <label htmlFor="skills-search" className={legacyFormClass('form-label')}>
              {t('common.search')}
            </label>
            <input
              id="skills-search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('skills.search_placeholder')}
              autoComplete="off"
            />
          </div>
        </div>
      </div>

      {apiError && <ErrorAlert error={apiError} />}

      {filtered.length === 0 ? (
        <div className={cardClass()} style={{ position: 'relative', minHeight: '200px' }}>
          <LoadingOverlay loading={apiLoading} />
          <EmptyState
            description={search.trim() ? t('skills.no_matches') : t('skills.empty')}
            action={search.trim() ? undefined : { label: t('skills.empty_cta'), onClick: openAdd }}
          />
        </div>
      ) : (
        <div className={savedQuestionListClass()}>
          <div className={cardClass()} style={{ position: 'relative' }}>
            <LoadingOverlay loading={apiLoading} />
            <div className="flex flex-col gap-3">
              {filtered.map((skill) => (
                <button
                  key={skill.id}
                  type="button"
                  className={cn(
                    'border-border hover:bg-card w-full rounded-lg border p-3 text-left transition-colors',
                    selectedSkill?.id === skill.id && 'border-primary bg-card',
                  )}
                  onClick={() => selectSkill(skill)}
                >
                  <span className={savedQuestionItemTitleClass()}>
                    {skill.name}
                    {!skill.is_active && ` — ${t('skills.inactive_badge')}`}
                  </span>
                  {skill.description && (
                    <p className={savedQuestionItemMetaPClass()}>{skill.description}</p>
                  )}
                </button>
              ))}
            </div>
          </div>

          <div className={cardClass()}>
            <SkillDetailPane
              skill={selectedSkill}
              paramValues={paramValues}
              onParamChange={(name, value) =>
                setParamValues((prev) => ({ ...prev, [name]: value }))
              }
              runLoading={runState.skillId === selectedSkill?.id && runState.loading}
              runError={runState.skillId === selectedSkill?.id ? runState.error : null}
              runResult={runState.skillId === selectedSkill?.id ? runState.result : null}
              onRun={() => void runSkill()}
              onOpenEdit={openEdit}
              onDelete={(id) => void handleDelete(id)}
              t={t}
            />
          </div>
        </div>
      )}

      <SkillFormModal
        mode="new"
        open={isNewModalOpen}
        title={t('skills.modal_title_new')}
        formError={formError}
        datasources={datasources}
        semanticModels={semanticModels}
        form={form}
        onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
        onClose={() => setIsNewModalOpen(false)}
        onSave={() => void handleSave(false)}
        saving={apiLoading}
        t={t}
      />
      <SkillFormModal
        mode="edit"
        open={isEditModalOpen}
        title={t('skills.modal_title_edit')}
        formError={formError}
        datasources={datasources}
        semanticModels={semanticModels}
        form={form}
        onChange={(patch) => setForm((prev) => ({ ...prev, ...patch }))}
        onClose={() => setIsEditModalOpen(false)}
        onSave={() => void handleSave(true)}
        saving={apiLoading}
        t={t}
      />
    </div>
  )
}
