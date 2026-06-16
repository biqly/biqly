import { useCallback, useEffect, useMemo, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import type {
  CompositeModelDetail,
  CompositeModelSummary,
  CompositePublishResult,
  CompositeValidationResult,
  CrossModelJoin,
  DimensionConflictResolution,
  SuggestedCrossJoin,
  SuggestedJoinsResponse,
} from '../types/composite'
import type { SemanticModelDetail } from '../types/semantic'
import { pickValidIdOrFirst } from '../utils/effectiveSelection'
import { CompositeDetailPanel } from './composites/CompositeDetailPanel'
import {
  compositeCreateFieldGroupClass,
  compositeCreateFormActionsClass,
  compositeCreateFormClass,
  compositeCreateFormRowClass,
  compositesBtnPrimaryClass,
  compositesBtnSecondaryClass,
  compositesControlsRowClass,
  compositesDetailClass,
  compositesDsSelectClass,
  compositesLayoutClass,
  compositesPageClass,
} from './composites/compositesClasses'
import { CompositesSidebar } from './composites/CompositesSidebar'
import { CrossJoinEditor } from './composites/CrossJoinEditor'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { Modal } from './ui/Modal'
import { Select } from './ui/Select'

export default function Composites() {
  const t = useT()
  const { get, postData, putData, deleteData, loading, error } = useApi()
  const confirm = useConfirm()
  const { datasources } = useDatasources()

  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const [composites, setComposites] = useState<CompositeModelSummary[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [detailState, setDetailState] = useState<{
    id: string | null
    detail: CompositeModelDetail | null
  }>({ id: null, detail: null })

  const { models } = useSemanticModels(datasourceId || null)
  const [componentModels, setComponentModels] = useState<Record<string, SemanticModelDetail>>({})

  // Create form
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newLabel, setNewLabel] = useState('')
  const [newDescription, setNewDescription] = useState('')

  // Component add
  const [addModelId, setAddModelId] = useState('')
  const [addAlias, setAddAlias] = useState('')
  const [addRole, setAddRole] = useState<'primary' | 'secondary'>('secondary')

  // Cross-join editor
  const [showCrossJoin, setShowCrossJoin] = useState(false)
  const [editingJoin, setEditingJoin] = useState<CrossModelJoin | null>(null)

  // Validation / suggestions
  const [validation, setValidation] = useState<CompositeValidationResult | null>(null)
  const [suggestions, setSuggestions] = useState<SuggestedCrossJoin[]>([])

  const loadComposites = useCallback(async () => {
    if (!datasourceId) {
      return
    }
    const list = await get<CompositeModelSummary[]>(
      `/api/semantic/composites?datasource_id=${encodeURIComponent(datasourceId)}`,
    )
    setComposites(list ?? [])
  }, [datasourceId, get])

  useEffect(() => {
    if (!datasourceId) {
      return
    }
    let cancelled = false
    void get<CompositeModelSummary[]>(
      `/api/semantic/composites?datasource_id=${encodeURIComponent(datasourceId)}`,
    ).then((list) => {
      if (!cancelled) {
        setComposites(list ?? [])
      }
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId, get])

  const loadDetail = useCallback(
    async (id: string) => {
      const full = await get<CompositeModelDetail>(`/api/semantic/composites/${id}`)
      if (full) {
        setDetailState({ id, detail: full })
        setValidation(null)
        setSuggestions([])
      }
    },
    [get],
  )

  const detail = selectedId && detailState.id === selectedId ? detailState.detail : null
  const initLoading = Boolean(selectedId && detailState.id !== selectedId)

  useEffect(() => {
    if (!selectedId) {
      return
    }
    let cancelled = false
    void get<CompositeModelDetail>(`/api/semantic/composites/${selectedId}`).then((full) => {
      if (!cancelled && full) {
        setDetailState({ id: selectedId, detail: full })
        setValidation(null)
        setSuggestions([])
      }
    })
    return () => {
      cancelled = true
    }
  }, [selectedId, get])

  // Load full model details for each component (dimensions + names).
  useEffect(() => {
    const comps = detail?.components ?? []
    void (async () => {
      const next: Record<string, SemanticModelDetail> = {}
      for (const c of comps) {
        const m = await get<SemanticModelDetail>(
          `/api/semantic/models/${c.model_id}?include_inactive=true`,
        )
        if (m) {
          next[c.model_id] = m
        }
      }
      setComponentModels(next)
    })()
  }, [detail?.components, get])

  const modelNames = useMemo(() => {
    const m: Record<string, string> = {}
    for (const model of models) {
      m[model.id] = model.label ?? model.name
    }
    return m
  }, [models])

  const dimensionsByAlias = useMemo(() => {
    const out: Record<string, string[]> = {}
    for (const c of detail?.components ?? []) {
      const md = componentModels[c.model_id]
      out[c.alias] = (md?.dimensions ?? []).map((d) => d.name)
    }
    return out
  }, [detail?.components, componentModels])

  const handleCreate = async () => {
    if (!newName.trim()) {
      return
    }
    const created = await postData<CompositeModelDetail>('/api/semantic/composites', {
      datasource_id: datasourceId,
      name: newName.trim(),
      label: newLabel.trim() || undefined,
      description: newDescription.trim() || undefined,
    })
    setShowCreate(false)
    setNewName('')
    setNewLabel('')
    setNewDescription('')
    await loadComposites()
    if (created?.id) {
      setSelectedId(created.id)
    }
  }

  const handleDelete = async (id: string) => {
    if (
      !(await confirm({
        title: t('composites.delete_confirm_title'),
        message: t('composites.delete_confirm_message'),
        variant: 'danger',
      }))
    ) {
      return
    }
    await deleteData(`/api/semantic/composites/${id}`)
    if (selectedId === id) {
      setSelectedId(null)
    }
    await loadComposites()
  }

  const refreshDetail = async () => {
    if (selectedId) {
      await loadDetail(selectedId)
    }
  }

  const handleAddComponent = async () => {
    if (!selectedId || !addModelId || !addAlias.trim()) {
      return
    }
    await postData(`/api/semantic/composites/${selectedId}/components`, {
      model_id: addModelId,
      alias: addAlias.trim(),
      role: addRole,
    })
    setAddModelId('')
    setAddAlias('')
    setAddRole('secondary')
    await refreshDetail()
  }

  const handleRemoveComponent = async (modelId: string) => {
    if (!selectedId) {
      return
    }
    await deleteData(`/api/semantic/composites/${selectedId}/components/${modelId}`)
    await refreshDetail()
  }

  const handleSubmitCrossJoin = async (join: CrossModelJoin) => {
    if (!selectedId) {
      return
    }
    if (editingJoin?.id) {
      await putData(`/api/semantic/composites/${selectedId}/cross-joins/${editingJoin.id}`, join)
    } else {
      await postData(`/api/semantic/composites/${selectedId}/cross-joins`, join)
    }
    setShowCrossJoin(false)
    setEditingJoin(null)
    await refreshDetail()
  }

  const handleRemoveCrossJoin = async (joinId?: string) => {
    if (!selectedId || !joinId) {
      return
    }
    await deleteData(`/api/semantic/composites/${selectedId}/cross-joins/${joinId}`)
    await refreshDetail()
  }

  const handleSetCanonicalDate = async (alias: string, dimension: string) => {
    if (!selectedId) {
      return
    }
    await putData(`/api/semantic/composites/${selectedId}/canonical-date`, {
      model_alias: alias,
      dimension_name: dimension,
    })
    await refreshDetail()
  }

  const handleResolutionChange = async (res: DimensionConflictResolution) => {
    if (!selectedId) {
      return
    }
    await putData(`/api/semantic/composites/${selectedId}/dimension-resolutions`, {
      resolutions: [res],
    })
    await refreshDetail()
  }

  const handleValidate = async () => {
    if (!selectedId) {
      return
    }
    const result = await postData<CompositePublishResult>(
      `/api/semantic/composites/${selectedId}/validate`,
      {},
    )
    setValidation(result?.validation ?? null)
  }

  const handlePublish = async () => {
    if (!selectedId) {
      return
    }
    const result = await postData<CompositePublishResult>(
      `/api/semantic/composites/${selectedId}/publish`,
      {},
    )
    setValidation(result?.validation ?? null)
    await refreshDetail()
    await loadComposites()
  }

  const handleRollback = async () => {
    if (!selectedId) {
      return
    }
    if (
      !(await confirm({
        title: t('composites.rollback_confirm_title'),
        message: t('composites.rollback_confirm_message'),
      }))
    ) {
      return
    }
    await postData(`/api/semantic/composites/${selectedId}/rollback`, {})
    await refreshDetail()
    await loadComposites()
  }

  const loadSuggestions = async () => {
    if (!selectedId) {
      return
    }
    const res = await get<SuggestedJoinsResponse>(
      `/api/semantic/composites/${selectedId}/suggested-joins`,
    )
    setSuggestions(res?.suggestions ?? [])
  }

  const applySuggestion = async (s: SuggestedCrossJoin) => {
    if (!selectedId) {
      return
    }
    await postData(`/api/semantic/composites/${selectedId}/cross-joins`, {
      from_model: s.from_model,
      from_dimension: s.from_dimension,
      to_model: s.to_model,
      to_dimension: s.to_dimension,
      join_type: 'LEFT',
      relationship: 'many_to_one',
    } satisfies CrossModelJoin)
    await refreshDetail()
  }

  const usedAliases = new Set((detail?.components ?? []).map((c) => c.alias))
  const availableModels = models.filter(
    (m) => !(detail?.components ?? []).some((c) => c.model_id === m.id),
  )

  return (
    <div className={compositesPageClass}>
      <div className={compositesControlsRowClass}>
        <div className={compositesDsSelectClass}>
          <Select
            value={datasourceId}
            onChange={setSelectedDatasourceId}
            options={datasources.map((d) => ({ value: d.id, label: d.name }))}
            placeholder={t('composites.datasource_placeholder')}
            ariaLabel={t('composites.datasource_placeholder')}
          />
        </div>
        {composites.length > 0 && (
          <button
            type="button"
            className={compositesBtnPrimaryClass}
            onClick={() => setShowCreate(true)}
            disabled={!datasourceId}
          >
            {t('composites.new')}
          </button>
        )}
      </div>

      {error && <ErrorAlert error={error} />}

      {composites.length === 0 ? (
        <div className="border-border bg-card flex min-h-112 flex-1 items-center justify-center rounded-xl border border-dashed p-8 text-center shadow-(--shadow)">
          <EmptyState
            description={t('composites.empty_list')}
            action={
              datasourceId
                ? {
                    label: t('composites.empty_detail_cta'),
                    onClick: () => setShowCreate(true),
                  }
                : undefined
            }
          />
        </div>
      ) : (
        <div className={compositesLayoutClass}>
          <CompositesSidebar
            t={t}
            composites={composites}
            selectedId={selectedId}
            onSelect={setSelectedId}
            onDelete={(id) => void handleDelete(id)}
          />

          <section className={compositesDetailClass}>
            {!selectedId ? (
              <EmptyState description={t('composites.empty_detail')} />
            ) : initLoading || !detail ? (
              <LoadingScreen />
            ) : (
              <CompositeDetailPanel
                t={t}
                detail={detail}
                loading={loading}
                validation={validation}
                modelNames={modelNames}
                dimensionsByAlias={dimensionsByAlias}
                suggestions={suggestions}
                addModelId={addModelId}
                addAlias={addAlias}
                addRole={addRole}
                availableModels={availableModels}
                usedAliases={usedAliases}
                onValidate={() => void handleValidate()}
                onPublish={() => void handlePublish()}
                onRollback={() => void handleRollback()}
                onAddModelIdChange={setAddModelId}
                onAddAliasChange={setAddAlias}
                onAddRoleChange={setAddRole}
                onAddComponent={() => void handleAddComponent()}
                onRemoveComponent={(modelId) => void handleRemoveComponent(modelId)}
                onLoadSuggestions={() => void loadSuggestions()}
                onAddJoin={() => {
                  setEditingJoin(null)
                  setShowCrossJoin(true)
                }}
                onEditJoin={(join) => {
                  setEditingJoin(join)
                  setShowCrossJoin(true)
                }}
                onRemoveCrossJoin={(joinId) => void handleRemoveCrossJoin(joinId)}
                onApplySuggestion={(s) => void applySuggestion(s)}
                onSetCanonicalDate={(alias, dim) => void handleSetCanonicalDate(alias, dim)}
                onResolutionChange={(res) => void handleResolutionChange(res)}
              />
            )}
          </section>
        </div>
      )}

      <Modal
        open={showCreate}
        title={t('composites.create_title')}
        onClose={() => setShowCreate(false)}
      >
        <div className={compositeCreateFormClass}>
          <div className={compositeCreateFormRowClass}>
            <div className={compositeCreateFieldGroupClass}>
              <label htmlFor="composite-name">{t('composites.field_name')}</label>
              <input
                id="composite-name"
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="e.g. sales_overview"
              />
            </div>
            <div className={compositeCreateFieldGroupClass}>
              <label htmlFor="composite-label">{t('composites.field_label')}</label>
              <input
                id="composite-label"
                type="text"
                value={newLabel}
                onChange={(e) => setNewLabel(e.target.value)}
                placeholder="e.g. Sales Overview"
              />
            </div>
          </div>
          <div className={compositeCreateFieldGroupClass}>
            <label htmlFor="composite-desc">{t('composites.field_description')}</label>
            <textarea
              id="composite-desc"
              value={newDescription}
              onChange={(e) => setNewDescription(e.target.value)}
              rows={3}
              placeholder={t('composites.description_placeholder')}
            />
          </div>
          <div className={compositeCreateFormActionsClass}>
            <button
              type="button"
              className={compositesBtnSecondaryClass}
              onClick={() => setShowCreate(false)}
            >
              {t('composites.cancel')}
            </button>
            <button
              type="button"
              className={compositesBtnPrimaryClass}
              onClick={() => {
                void handleCreate()
              }}
              disabled={!newName.trim()}
            >
              {t('composites.create')}
            </button>
          </div>
        </div>
      </Modal>

      <Modal
        open={showCrossJoin}
        title={editingJoin ? t('composites.edit_join_title') : t('composites.add_join_title')}
        onClose={() => {
          setShowCrossJoin(false)
          setEditingJoin(null)
        }}
      >
        <CrossJoinEditor
          components={detail?.components ?? []}
          dimensionsByAlias={dimensionsByAlias}
          initial={editingJoin ?? undefined}
          onSubmit={(payload) => {
            void handleSubmitCrossJoin(payload)
          }}
          onCancel={() => {
            setShowCrossJoin(false)
            setEditingJoin(null)
          }}
        />
      </Modal>
    </div>
  )
}
