import type { KeyboardEvent as ReactKeyboardEvent } from 'react'
import { useCallback, useEffect, useMemo, useState } from 'react'

import { useAdminApi, useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useModelDetail } from '../hooks/useModelDetail'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useToast } from '../hooks/useToast'
import { useT } from '../i18n'
import { legacyButtonClass, rowActionsClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import {
  fewShotMainFormClass,
  fewShotModalCardClass,
  fewShotSidebarClass,
  fewShotSidebarHeaderClass,
  fewShotSidebarListClass,
  fieldBadgeBtnClass,
  fieldBadgeBtnTypeClass,
  modalBodyTwoColClass,
} from '../lib/fewShotLayoutClasses'
import { formControlClass, formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import { modalActionsClass, modalFormRowClass } from '../lib/modalClasses'
import { legacyTableClass } from '../lib/tableClasses'
import type { EnrichAnalyzeResult, EnrichApplyResult } from '../types/enrichContext'
import type { BusinessGlossaryTerm, GlossaryAIContext } from '../types/glossary'
import { GlossaryEnrichPanel } from './GlossaryEnrichPanel'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'
export default function Glossary() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { get, postData, putData, deleteData, loading } = useApi()
  const { postData: adminPost, loading: enrichLoading, configured: adminConfigured } = useAdminApi()

  const [terms, setTerms] = useState<BusinessGlossaryTerm[]>([])
  const [initLoading, setInitLoading] = useState(true)

  // Filters
  const { datasources } = useDatasources()
  const { models: allModels } = useSemanticModels(null, { all: true })
  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [searchQuery, setSearchQuery] = useState('')

  // Form States
  const [showForm, setShowForm] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [formDatasourceId, setFormDatasourceId] = useState('')
  const [formModelId, setFormModelId] = useState('')
  const [formTerm, setFormTerm] = useState('')
  const [formDefinition, setFormDefinition] = useState('')
  const [formMapsToType, setFormMapsToType] = useState<'dimension' | 'metric' | 'model'>(
    'dimension',
  )
  const [formMapsToName, setFormMapsToName] = useState('')
  const [formAliases, setFormAliases] = useState<string[]>([])
  const [aliasInput, setAliasInput] = useState('')
  const [formContextSynonyms, setFormContextSynonyms] = useState<string[]>([])
  const [contextSynonymInput, setContextSynonymInput] = useState('')
  const [formUnit, setFormUnit] = useState('')
  const [formNullMeaning, setFormNullMeaning] = useState('')
  const [formBusinessRules, setFormBusinessRules] = useState<string[]>([])
  const [businessRuleInput, setBusinessRuleInput] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  const [showEnrichPanel, setShowEnrichPanel] = useState(false)
  const [enrichResult, setEnrichResult] = useState<EnrichAnalyzeResult | null>(null)
  const [enrichSelections, setEnrichSelections] = useState<
    Record<string, { selected: boolean; value: string }>
  >({})
  const [enrichError, setEnrichError] = useState<string | null>(null)
  const [enrichApplyResult, setEnrichApplyResult] = useState<EnrichApplyResult | null>(null)

  // Sidebar details for selecting model fields
  const { model: activeModelDetail, setModel: setActiveModelDetail } = useModelDetail(formModelId, {
    includeInactive: true,
  })
  const [sidebarSearch, setSidebarSearch] = useState('')

  // Default datasource configuration
  useEffect(() => {
    if (datasources.length > 0 && !selectedDatasourceId) {
      const firstDs = datasources[0]
      if (firstDs) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedDatasourceId(firstDs.id)
      }
    }
  }, [datasources, selectedDatasourceId])

  // Load Glossary Terms
  const loadTerms = useCallback(async () => {
    if (!selectedDatasourceId) {
      return
    }
    setInitLoading(true)
    try {
      const url = `/api/ai/glossary?datasource_id=${encodeURIComponent(selectedDatasourceId)}${
        selectedModelId ? `&model_id=${encodeURIComponent(selectedModelId)}` : ''
      }`
      const res = await get<BusinessGlossaryTerm[]>(url)
      if (res) {
        setTerms(res)
      }
    } catch {
      toast.error(t('glossary.load_error'))
    } finally {
      setInitLoading(false)
    }
  }, [get, selectedDatasourceId, selectedModelId, t, toast])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadTerms()
  }, [loadTerms])

  const resetAIContextForm = useCallback(() => {
    setFormContextSynonyms([])
    setContextSynonymInput('')
    setFormUnit('')
    setFormNullMeaning('')
    setFormBusinessRules([])
    setBusinessRuleInput('')
  }, [])

  const applyAIContextToForm = (ctx?: GlossaryAIContext) => {
    setFormContextSynonyms(ctx?.synonyms ?? [])
    setContextSynonymInput('')
    setFormUnit(ctx?.unit ?? '')
    setFormNullMeaning(ctx?.null_meaning ?? '')
    setFormBusinessRules(ctx?.business_rules ?? [])
    setBusinessRuleInput('')
  }

  const buildAIContextPayload = (): GlossaryAIContext | undefined => {
    const unit = formUnit.trim()
    const nullMeaning = formNullMeaning.trim()
    if (
      formContextSynonyms.length === 0 &&
      unit === '' &&
      nullMeaning === '' &&
      formBusinessRules.length === 0
    ) {
      return undefined
    }
    return {
      synonyms: formContextSynonyms.length > 0 ? formContextSynonyms : undefined,
      unit: unit || undefined,
      null_meaning: nullMeaning || undefined,
      business_rules: formBusinessRules.length > 0 ? formBusinessRules : undefined,
    }
  }

  const resetForm = useCallback(() => {
    setFormTerm('')
    setFormDefinition('')
    setFormMapsToType('dimension')
    setFormMapsToName('')
    setFormAliases([])
    setAliasInput('')
    resetAIContextForm()
    setFormDatasourceId('')
    setFormModelId('')
    setEditId(null)
    setFormError(null)
    setShowForm(false)
    setActiveModelDetail(null)
    setSidebarSearch('')
  }, [resetAIContextForm, setActiveModelDetail])

  const openAdd = () => {
    setFormTerm('')
    setFormDefinition('')
    setFormMapsToType('dimension')
    setFormMapsToName('')
    setFormAliases([])
    setAliasInput('')
    resetAIContextForm()
    setFormDatasourceId(selectedDatasourceId || (datasources[0]?.id ?? ''))
    setFormModelId(selectedModelId || '')
    setEditId(null)
    setFormError(null)
    setShowForm(true)
    setActiveModelDetail(null)
    setSidebarSearch('')
  }

  const openEdit = (term: BusinessGlossaryTerm) => {
    setEditId(term.id)
    setFormTerm(term.term)
    setFormDefinition(term.definition ?? '')
    setFormMapsToType(term.maps_to_type)
    setFormMapsToName(term.maps_to_name)
    setFormAliases(term.aliases ?? [])
    setAliasInput('')
    applyAIContextToForm(term.ai_context)
    setFormDatasourceId(term.datasource_id)
    setFormModelId(term.model_id ?? '')
    setFormError(null)
    setShowForm(true)
    setSidebarSearch('')
  }

  const handleAddAlias = () => {
    const val = aliasInput.trim()
    if (val && !formAliases.includes(val)) {
      setFormAliases([...formAliases, val])
    }
    setAliasInput('')
  }

  const handleAliasKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleAddAlias()
    }
  }

  const handleRemoveAlias = (alias: string) => {
    setFormAliases(formAliases.filter((a) => a !== alias))
  }

  const handleAddContextSynonym = () => {
    const val = contextSynonymInput.trim()
    if (val && !formContextSynonyms.includes(val)) {
      setFormContextSynonyms([...formContextSynonyms, val])
    }
    setContextSynonymInput('')
  }

  const handleContextSynonymKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleAddContextSynonym()
    }
  }

  const handleRemoveContextSynonym = (synonym: string) => {
    setFormContextSynonyms(formContextSynonyms.filter((s) => s !== synonym))
  }

  const handleAddBusinessRule = () => {
    const val = businessRuleInput.trim()
    if (val && !formBusinessRules.includes(val)) {
      setFormBusinessRules([...formBusinessRules, val])
    }
    setBusinessRuleInput('')
  }

  const handleBusinessRuleKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleAddBusinessRule()
    }
  }

  const handleRemoveBusinessRule = (rule: string) => {
    setFormBusinessRules(formBusinessRules.filter((r) => r !== rule))
  }

  const handleSave = async () => {
    setFormError(null)
    if (!formTerm.trim()) {
      setFormError(t('glossary.err_term_required'))
      return
    }
    if (!formMapsToName.trim()) {
      setFormError(t('glossary.err_maps_to_name_required'))
      return
    }

    const payload = {
      term: formTerm.trim(),
      definition: formDefinition.trim(),
      maps_to_type: formMapsToType,
      maps_to_name: formMapsToName.trim(),
      aliases: formAliases,
      ai_context: buildAIContextPayload(),
    }

    if (editId) {
      try {
        await putData(`/api/ai/glossary/${editId}`, {
          ...payload,
          is_active: true,
        })
        void loadTerms()
        resetForm()
      } catch {
        setFormError(t('glossary.save_failed'))
      }
    } else {
      try {
        await postData('/api/ai/glossary', {
          ...payload,
          datasource_id: formDatasourceId,
          model_id: formModelId || undefined,
        })
        void loadTerms()
        resetForm()
      } catch {
        setFormError(t('glossary.save_failed'))
      }
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await confirm({
      title: t('glossary.confirm_delete'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await deleteData(`/api/ai/glossary/${id}`)
      setTerms(terms.filter((t) => t.id !== id))
    } catch {
      toast.error(t('glossary.delete_error'))
    }
  }

  // Set maps to fields based on sidebar selection
  const handleInsertField = (fieldName: string, type: 'dimension' | 'metric' | 'model') => {
    setFormMapsToType(type)
    setFormMapsToName(fieldName)
    if (!formTerm) {
      setFormTerm(fieldName)
    }
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

  // Search logic for items in the glossary table
  const displayedTerms = useMemo(() => {
    const q = searchQuery.toLowerCase().trim()
    if (!q) {
      return terms
    }
    return terms.filter((term) => {
      return (
        term.term.toLowerCase().includes(q) ||
        (term.definition?.toLowerCase().includes(q) ??
          term.maps_to_name.toLowerCase().includes(q)) ||
        term.maps_to_type.toLowerCase().includes(q) ||
        term.aliases?.some((a) => a.toLowerCase().includes(q))
      )
    })
  }, [terms, searchQuery])

  // Sidebar list filters
  const filteredDimensions = useMemo(() => {
    if (!activeModelDetail?.dimensions) {
      return []
    }
    const query = sidebarSearch.toLowerCase().trim()
    const list = activeModelDetail.dimensions.filter((d) => d.is_active !== false)
    if (!query) {
      return list
    }
    return list.filter(
      (d) => d.name.toLowerCase().includes(query) || d.label?.toLowerCase().includes(query),
    )
  }, [activeModelDetail, sidebarSearch])

  const filteredMetrics = useMemo(() => {
    if (!activeModelDetail?.metrics) {
      return []
    }
    const query = sidebarSearch.toLowerCase().trim()
    const list = activeModelDetail.metrics.filter((m) => m.is_active !== false)
    if (!query) {
      return list
    }
    return list.filter(
      (m) => m.name.toLowerCase().includes(query) || m.label?.toLowerCase().includes(query),
    )
  }, [activeModelDetail, sidebarSearch])

  // Populate maps_to_name select options
  const mapsToNameOptions = useMemo(() => {
    if (!formModelId || !activeModelDetail) {
      return []
    }
    if (formMapsToType === 'dimension') {
      return (activeModelDetail.dimensions ?? [])
        .filter((d) => d.is_active !== false)
        .map((d) => ({ value: d.name, label: d.label ? `${d.name} (${d.label})` : d.name }))
    }
    if (formMapsToType === 'metric') {
      return (activeModelDetail.metrics ?? [])
        .filter((m) => m.is_active !== false)
        .map((m) => ({ value: m.name, label: m.label ? `${m.name} (${m.label})` : m.name }))
    }
    return [
      {
        value: activeModelDetail.name,
        label: activeModelDetail.label
          ? `${activeModelDetail.name} (${activeModelDetail.label})`
          : activeModelDetail.name,
      },
    ]
  }, [activeModelDetail, formMapsToType, formModelId])

  const runEnrichAnalyze = useCallback(async () => {
    setEnrichError(null)
    setEnrichApplyResult(null)
    if (!selectedDatasourceId || !selectedModelId) {
      setEnrichError(t('glossary.enrich_context_model_required'))
      return
    }
    if (!adminConfigured) {
      setEnrichError(t('glossary.enrich_context_admin_key'))
      return
    }
    try {
      const result = await adminPost<EnrichAnalyzeResult>('/api/ai/enrich-context', {
        datasource_id: selectedDatasourceId,
        model_id: selectedModelId,
        suggest: true,
      })
      if (!result) {
        return
      }
      setEnrichResult(result)
      setShowEnrichPanel(true)
      const suggestionsByGap = new Map(
        (result.suggestions ?? []).map((suggestion) => [suggestion.gap_id, suggestion.text]),
      )
      const next: Record<string, { selected: boolean; value: string }> = {}
      for (const gap of result.gaps) {
        if (gap.applyable) {
          next[gap.id] = {
            selected: true,
            value: suggestionsByGap.get(gap.id) ?? '',
          }
        }
      }
      setEnrichSelections(next)
    } catch {
      setEnrichError(t('glossary.enrich_failed'))
    }
  }, [adminConfigured, adminPost, selectedDatasourceId, selectedModelId, t])

  const applyEnrichSelected = useCallback(async () => {
    if (!enrichResult || !selectedDatasourceId || !selectedModelId) {
      return
    }
    const items = Object.entries(enrichSelections)
      .filter(([, selection]) => selection.selected && selection.value.trim())
      .map(([gap_id, selection]) => ({ gap_id, value: selection.value.trim() }))
    if (items.length === 0) {
      return
    }
    setEnrichError(null)
    try {
      const result = await adminPost<EnrichApplyResult>('/api/ai/enrich-context/apply', {
        datasource_id: selectedDatasourceId,
        model_id: selectedModelId,
        items,
      })
      await loadTerms()
      await runEnrichAnalyze()
      if (result) {
        setEnrichApplyResult(result)
      }
    } catch {
      setEnrichError(t('glossary.enrich_failed'))
    }
  }, [
    adminPost,
    enrichResult,
    enrichSelections,
    loadTerms,
    runEnrichAnalyze,
    selectedDatasourceId,
    selectedModelId,
    t,
  ])

  if (initLoading && terms.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={legacyCardClass('card')}>
        <div className={legacyCardClass('card-intro')}>
          <div className={legacyCardClass('card-header-row')}>
            <h2>{t('glossary.title')}</h2>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className={legacyButtonClass('btn btn-sm btn-secondary')}
                disabled={!selectedModelId || enrichLoading}
                title={t('glossary.enrich_context_hint')}
                onClick={() => {
                  void runEnrichAnalyze()
                }}
              >
                {t('glossary.enrich_context')}
              </button>
              <button
                type="button"
                className={legacyButtonClass('btn btn-sm btn-primary')}
                onClick={openAdd}
              >
                {t('glossary.new')}
              </button>
            </div>
          </div>
          <p
            className={legacyCardClass('card-lead card-lead--single-line')}
            title={t('glossary.manage_hint')}
          >
            {t('glossary.manage_hint')}
          </p>
        </div>

        {/* Filters & Search */}
        <div className={cn(formRowClass, 'mb-5')}>
          <label className={cn(legacyFormClass('form-field'), 'min-w-56')}>
            <span className={legacyFormClass('form-label')}>{t('glossary.label_datasource')}</span>
            <Select
              value={selectedDatasourceId}
              options={datasources.map((d) => ({ value: d.id, label: d.name }))}
              onChange={(v) => {
                setSelectedDatasourceId(v)
                setSelectedModelId('')
              }}
            />
          </label>
          <label className={cn(legacyFormClass('form-field'), 'min-w-56')}>
            <span className={legacyFormClass('form-label')}>{t('glossary.label_model')}</span>
            <Select
              value={selectedModelId}
              options={[
                { value: '', label: t('glossary.option_all_models') },
                ...filterModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
              ]}
              onChange={setSelectedModelId}
            />
          </label>
          <div className={cn(legacyFormClass('form-field'), 'min-w-[16rem] flex-1')}>
            <span className={legacyFormClass('form-label')}>{t('common.search')}</span>
            <input
              type="text"
              placeholder={t('glossary.search_placeholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className={legacyFormClass('input')}
            />
          </div>
        </div>

        {enrichError && <ErrorAlert error={enrichError} />}

        {showEnrichPanel && enrichResult && (
          <GlossaryEnrichPanel
            result={enrichResult}
            selections={enrichSelections}
            applyResult={enrichApplyResult}
            loading={enrichLoading}
            onClose={() => {
              setShowEnrichPanel(false)
              setEnrichResult(null)
              setEnrichSelections({})
              setEnrichApplyResult(null)
            }}
            onRerun={() => {
              void runEnrichAnalyze()
            }}
            onApply={() => {
              void applyEnrichSelected()
            }}
            onSelectionChange={(gapId, patch) => {
              setEnrichSelections((prev) => ({
                ...prev,
                [gapId]: {
                  selected: patch.selected ?? prev[gapId]?.selected ?? true,
                  value: patch.value ?? prev[gapId]?.value ?? '',
                },
              }))
            }}
            onSelectAll={(selected) => {
              setEnrichSelections((prev) => {
                const next = { ...prev }
                for (const gap of enrichResult.gaps) {
                  if (gap.applyable) {
                    next[gap.id] = {
                      selected,
                      value: prev[gap.id]?.value ?? '',
                    }
                  }
                }
                return next
              })
            }}
          />
        )}

        {displayedTerms.length === 0 && <EmptyState description={t('glossary.empty')} />}

        {displayedTerms.length > 0 && (
          <div className="table-wrap">
            <table className={legacyTableClass('results-table')}>
              <thead>
                <tr>
                  <th>{t('glossary.col_term')}</th>
                  <th>{t('glossary.col_definition')}</th>
                  <th>{t('glossary.col_maps_to')}</th>
                  <th>{t('glossary.col_type')}</th>
                  <th>{t('glossary.col_aliases')}</th>
                  <th className="actions">{t('glossary.col_actions')}</th>
                </tr>
              </thead>
              <tbody>
                {displayedTerms.map((term) => (
                  <tr key={term.id}>
                    <td className="text-foreground font-semibold">{term.term}</td>
                    <td
                      className="max-w-75 overflow-hidden text-ellipsis whitespace-nowrap"
                      title={term.definition}
                    >
                      {term.definition ?? <span className="text-foreground-faint italic">—</span>}
                    </td>
                    <td>
                      <code className="text-caption text-foreground">{term.maps_to_name}</code>
                    </td>
                    <td>
                      <span
                        className={cn(
                          'text-2xs inline-block rounded px-1.5 py-0.5 font-bold uppercase',
                          term.maps_to_type === 'metric' && 'bg-success/10 text-success',
                          term.maps_to_type === 'dimension' && 'bg-blue-500/10 text-blue-500',
                          term.maps_to_type === 'model' && 'bg-purple-500/10 text-purple-500',
                        )}
                      >
                        {t(`glossary.type_${term.maps_to_type}`)}
                      </span>
                    </td>
                    <td>
                      {term.aliases && term.aliases.length > 0 ? (
                        term.aliases.map((alias) => (
                          <span
                            key={alias}
                            className="border-border text-2xs text-foreground-muted mr-1 mb-1 inline-block rounded border bg-white/5 px-1.5 py-0.5"
                          >
                            {alias}
                          </span>
                        ))
                      ) : (
                        <span className="text-foreground-faint">—</span>
                      )}
                    </td>
                    <td className="actions">
                      <div className={rowActionsClass}>
                        <button
                          type="button"
                          className={legacyButtonClass('btn btn-sm btn-ghost')}
                          onClick={() => openEdit(term)}
                        >
                          {t('common.edit')}
                        </button>
                        <button
                          type="button"
                          className={legacyButtonClass('btn btn-sm btn-danger')}
                          onClick={() => {
                            void handleDelete(term.id)
                          }}
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
        <Modal
          open
          title={editId ? t('glossary.form_edit_title') : t('glossary.form_add_title')}
          onClose={resetForm}
          className={fewShotModalCardClass()}
          bodyClassName={modalBodyTwoColClass()}
        >
          <div className={fewShotMainFormClass()}>
            <div className={modalFormRowClass()}>
              <div className={legacyFormClass('form-group')}>
                <label>{t('glossary.label_datasource')}</label>
                <Select
                  value={formDatasourceId}
                  onChange={(val) => {
                    setFormDatasourceId(val)
                    setFormModelId('')
                    setFormMapsToName('')
                  }}
                  options={datasources.map((d) => ({ value: d.id, label: d.name }))}
                  disabled={!!editId}
                />
              </div>
              <div className={legacyFormClass('form-group')}>
                <label>{t('glossary.label_model')}</label>
                <Select
                  value={formModelId}
                  onChange={(val) => {
                    setFormModelId(val)
                    setFormMapsToName('')
                  }}
                  options={[
                    { value: '', label: t('glossary.option_all_models') },
                    ...formModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
                  ]}
                  disabled={!!editId}
                />
              </div>
            </div>

            <div className={legacyFormClass('form-group')}>
              <label htmlFor="gl-term">{t('glossary.label_term')}</label>
              <input
                id="gl-term"
                type="text"
                value={formTerm}
                onChange={(e) => setFormTerm(e.target.value)}
                placeholder={t('glossary.placeholder_term')}
              />
            </div>

            <div className={legacyFormClass('form-group')}>
              <label htmlFor="gl-definition">{t('glossary.label_definition')}</label>
              <textarea
                id="gl-definition"
                value={formDefinition}
                onChange={(e) => setFormDefinition(e.target.value)}
                placeholder={t('glossary.placeholder_definition')}
                rows={2}
              />
            </div>

            <div className={modalFormRowClass()}>
              <div className={legacyFormClass('form-group')}>
                <label>{t('glossary.label_maps_to_type')}</label>
                <Select
                  value={formMapsToType}
                  onChange={(val) => {
                    setFormMapsToType(val)
                    setFormMapsToName('')
                  }}
                  options={[
                    { value: 'dimension', label: t('glossary.type_dimension') },
                    { value: 'metric', label: t('glossary.type_metric') },
                    { value: 'model', label: t('glossary.type_model') },
                  ]}
                />
              </div>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="gl-maps-to-name">{t('glossary.label_maps_to_name')}</label>
                {formModelId && activeModelDetail ? (
                  <Select
                    id="gl-maps-to-name"
                    value={formMapsToName}
                    onChange={setFormMapsToName}
                    options={[{ value: '', label: '— pick mapped target —' }, ...mapsToNameOptions]}
                  />
                ) : (
                  <input
                    id="gl-maps-to-name"
                    type="text"
                    value={formMapsToName}
                    onChange={(e) => setFormMapsToName(e.target.value)}
                    placeholder="e.g. total_amount, order_date"
                  />
                )}
              </div>
            </div>

            <div className={legacyFormClass('form-group')}>
              <label htmlFor="gl-aliases">{t('glossary.label_aliases')}</label>
              <div className="border-border bg-canvas flex flex-wrap items-center gap-1.5 rounded-lg border p-1.5">
                {formAliases.map((alias) => (
                  <span
                    key={alias}
                    className="text-caption text-accent inline-flex items-center gap-1 rounded border border-blue-400/20 bg-blue-400/10 px-2 py-0.5"
                  >
                    {alias}
                    <button
                      type="button"
                      onClick={() => handleRemoveAlias(alias)}
                      className="text-accent text-caption flex cursor-pointer items-center border-0 bg-transparent p-0 font-bold"
                    >
                      ×
                    </button>
                  </span>
                ))}
                <input
                  id="gl-aliases"
                  type="text"
                  value={aliasInput}
                  onChange={(e) => setAliasInput(e.target.value)}
                  onKeyDown={handleAliasKeyDown}
                  onBlur={handleAddAlias}
                  placeholder={t('glossary.placeholder_aliases')}
                  className="text-caption text-foreground w-auto! min-w-30 flex-1 border-0! bg-transparent! px-1.5! py-1! shadow-none! focus-visible:shadow-none! focus-visible:ring-0!"
                />
              </div>
            </div>

            <div className={legacyFormClass('form-group')}>
              <label>{t('glossary.section_ai_context')}</label>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="gl-context-synonyms">{t('glossary.label_context_synonyms')}</label>
                <div className="border-border bg-canvas flex flex-wrap items-center gap-1.5 rounded-lg border p-1.5">
                  {formContextSynonyms.map((synonym) => (
                    <span
                      key={synonym}
                      className="text-caption inline-flex items-center gap-1 rounded border border-purple-400/20 bg-purple-400/10 px-2 py-0.5 text-purple-400"
                    >
                      {synonym}
                      <button
                        type="button"
                        onClick={() => handleRemoveContextSynonym(synonym)}
                        className="text-caption flex cursor-pointer items-center border-0 bg-transparent p-0 font-bold text-purple-400"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                  <input
                    id="gl-context-synonyms"
                    type="text"
                    value={contextSynonymInput}
                    onChange={(e) => setContextSynonymInput(e.target.value)}
                    onKeyDown={handleContextSynonymKeyDown}
                    onBlur={handleAddContextSynonym}
                    placeholder={t('glossary.placeholder_context_synonyms')}
                    className="text-caption text-foreground w-auto! min-w-30 flex-1 border-0! bg-transparent! px-1.5! py-1! shadow-none! focus-visible:shadow-none! focus-visible:ring-0!"
                  />
                </div>
              </div>
              <div className={modalFormRowClass()}>
                <div className={legacyFormClass('form-group')}>
                  <label htmlFor="gl-unit">{t('glossary.label_unit')}</label>
                  <input
                    id="gl-unit"
                    type="text"
                    value={formUnit}
                    onChange={(e) => setFormUnit(e.target.value)}
                    placeholder={t('glossary.placeholder_unit')}
                  />
                </div>
                <div className={legacyFormClass('form-group')}>
                  <label htmlFor="gl-null-meaning">{t('glossary.label_null_meaning')}</label>
                  <input
                    id="gl-null-meaning"
                    type="text"
                    value={formNullMeaning}
                    onChange={(e) => setFormNullMeaning(e.target.value)}
                    placeholder={t('glossary.placeholder_null_meaning')}
                  />
                </div>
              </div>
              <div className={legacyFormClass('form-group')}>
                <label htmlFor="gl-business-rules">{t('glossary.label_business_rules')}</label>
                <div className="border-border bg-canvas flex flex-wrap items-center gap-1.5 rounded-lg border p-1.5">
                  {formBusinessRules.map((rule) => (
                    <span
                      key={rule}
                      className="bg-warning/10 border-warning/20 text-caption text-warning inline-flex items-center gap-1 rounded border px-2 py-0.5"
                    >
                      {rule}
                      <button
                        type="button"
                        onClick={() => handleRemoveBusinessRule(rule)}
                        className="text-warning text-caption flex cursor-pointer items-center border-0 bg-transparent p-0 font-bold"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                  <input
                    id="gl-business-rules"
                    type="text"
                    value={businessRuleInput}
                    onChange={(e) => setBusinessRuleInput(e.target.value)}
                    onKeyDown={handleBusinessRuleKeyDown}
                    onBlur={handleAddBusinessRule}
                    placeholder={t('glossary.placeholder_business_rules')}
                    className="text-caption text-foreground w-auto! min-w-30 flex-1 border-0! bg-transparent! px-1.5! py-1! shadow-none! focus-visible:shadow-none! focus-visible:ring-0!"
                  />
                </div>
              </div>
            </div>

            <ErrorAlert error={formError} />
          </div>

          <div className={fewShotSidebarClass()}>
            <div className={fewShotSidebarHeaderClass()}>
              {t('few_shot.available_fields_title')}
            </div>
            {activeModelDetail ? (
              <>
                <input
                  type="text"
                  className={cn(formControlClass, 'mb-1')}
                  placeholder={t('few_shot.search_fields_placeholder')}
                  value={sidebarSearch}
                  onChange={(e) => setSidebarSearch(e.target.value)}
                />
                <div className={fewShotSidebarListClass()}>
                  {/* Model itself */}
                  <button
                    type="button"
                    className={fieldBadgeBtnClass}
                    onClick={() => handleInsertField(activeModelDetail.name, 'model')}
                    title={
                      activeModelDetail.description ??
                      activeModelDetail.label ??
                      activeModelDetail.name
                    }
                  >
                    <span>{activeModelDetail.name}</span>
                    <span className={fieldBadgeBtnTypeClass}>model</span>
                  </button>
                  {/* Dimensions */}
                  {filteredDimensions.map((d) => (
                    <button
                      key={d.id}
                      type="button"
                      className={fieldBadgeBtnClass}
                      onClick={() => handleInsertField(d.name, 'dimension')}
                      title={d.description ?? d.label ?? d.name}
                    >
                      <span>{d.name}</span>
                      <span className={fieldBadgeBtnTypeClass}>dim</span>
                    </button>
                  ))}
                  {/* Metrics */}
                  {filteredMetrics.map((m) => (
                    <button
                      key={m.id}
                      type="button"
                      className={fieldBadgeBtnClass}
                      onClick={() => handleInsertField(m.name, 'metric')}
                      title={m.description ?? m.label ?? m.name}
                    >
                      <span>{m.name}</span>
                      <span className={fieldBadgeBtnTypeClass}>met</span>
                    </button>
                  ))}
                  {filteredDimensions.length === 0 && filteredMetrics.length === 0 && (
                    <span className="text-caption text-foreground-muted mt-4 text-center">
                      No fields found
                    </span>
                  )}
                </div>
              </>
            ) : (
              <div className="text-caption text-foreground-muted mt-2 leading-normal">
                {t('few_shot.helper_select_model')}
              </div>
            )}
          </div>
          <div className={modalActionsClass()}>
            <button
              type="button"
              className={legacyButtonClass('btn btn-ghost')}
              onClick={resetForm}
            >
              {t('common.cancel')}
            </button>
            <button
              type="button"
              className={legacyButtonClass('btn btn-primary')}
              onClick={() => {
                void handleSave()
              }}
              disabled={loading}
            >
              {loading ? t('common.saving') : t('common.save')}
            </button>
          </div>
        </Modal>
      )}
    </div>
  )
}
