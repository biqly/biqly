import '../styles/composites.css'

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
import { CompositeCanvas } from './composites/CompositeCanvas'
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

  const [datasourceId, setDatasourceId] = useState('')
  const [composites, setComposites] = useState<CompositeModelSummary[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [detail, setDetail] = useState<CompositeModelDetail | null>(null)
  const [initLoading, setInitLoading] = useState(false)

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

  useEffect(() => {
    if (datasources && datasources.length > 0 && !datasourceId) {
      setDatasourceId(datasources[0]?.id ?? '')
    }
  }, [datasources, datasourceId])

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
    loadComposites()
  }, [loadComposites])

  const loadDetail = useCallback(
    async (id: string) => {
      setInitLoading(true)
      try {
        const full = await get<CompositeModelDetail>(`/api/semantic/composites/${id}`)
        setDetail(full)
        setValidation(null)
        setSuggestions([])
      } finally {
        setInitLoading(false)
      }
    },
    [get],
  )

  useEffect(() => {
    if (selectedId) {
      loadDetail(selectedId)
    } else {
      setDetail(null)
    }
  }, [selectedId, loadDetail])

  // Load full model details for each component (dimensions + names).
  useEffect(() => {
    const comps = detail?.components ?? []
    let cancelled = false
    ;(async () => {
      const next: Record<string, SemanticModelDetail> = {}
      for (const c of comps) {
        const m = await get<SemanticModelDetail>(
          `/api/semantic/models/${c.model_id}?include_inactive=true`,
        )
        if (m) {
          next[c.model_id] = m
        }
      }
      if (!cancelled) {
        setComponentModels(next)
      }
    })()
    return () => {
      cancelled = true
    }
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
    <div className="composites-page">
      <div className="composites-controls-row">
        <div className="composites-ds-select">
          <Select
            value={datasourceId}
            onChange={setDatasourceId}
            options={(datasources ?? []).map((d) => ({ value: d.id, label: d.name }))}
            placeholder={t('composites.datasource_placeholder')}
            ariaLabel={t('composites.datasource_placeholder')}
          />
        </div>
        <button
          type="button"
          className="btn-primary"
          onClick={() => setShowCreate(true)}
          disabled={!datasourceId}
        >
          {t('composites.new')}
        </button>
      </div>

      {error && <ErrorAlert error={error} />}

      <div className="composites-layout">
        <aside className="composites-sidebar">
          <div className="composites-sidebar-header">
            <h2>{t('composites.sidebar_title')}</h2>
          </div>
          {composites.length === 0 ? (
            <div style={{ padding: '1rem' }}>
              <EmptyState description={t('composites.empty_list')} />
            </div>
          ) : (
            <ul>
              {composites.map((c) => (
                <li key={c.id} className={c.id === selectedId ? 'active' : ''}>
                  <button
                    type="button"
                    className="composites-list-btn"
                    onClick={() => setSelectedId(c.id)}
                  >
                    <span className="composite-name">{c.label ?? c.name}</span>
                    <span className={`composite-status status-${c.status}`}>{c.status}</span>
                  </button>
                  <button
                    type="button"
                    className="composites-delete-btn"
                    aria-label={t('composites.aria_delete')}
                    onClick={() => {
                      void handleDelete(c.id)
                    }}
                  >
                    ×
                  </button>
                </li>
              ))}
            </ul>
          )}
        </aside>

        <section className="composites-detail">
          {!selectedId ? (
            <EmptyState description={t('composites.empty_detail')} />
          ) : initLoading || !detail ? (
            <LoadingScreen />
          ) : (
            <>
              <div className="composite-detail-head">
                <div>
                  <h2>{detail.label ?? detail.name}</h2>
                  {detail.description && <p>{detail.description}</p>}
                  <span className={`composite-status status-${detail.status}`}>
                    {detail.status} · v{detail.version}
                  </span>
                </div>
                <div className="composite-actions">
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      void handleValidate()
                    }}
                    disabled={loading}
                  >
                    {t('composites.validate')}
                  </button>
                  <button
                    type="button"
                    className="btn-primary"
                    onClick={() => {
                      void handlePublish()
                    }}
                    disabled={loading}
                  >
                    {t('composites.publish')}
                  </button>
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      void handleRollback()
                    }}
                    disabled={loading}
                  >
                    {t('composites.rollback')}
                  </button>
                </div>
              </div>

              {validation && (
                <div className={`composite-validation ${validation.valid ? 'valid' : 'invalid'}`}>
                  <strong>
                    {validation.valid
                      ? t('composites.validation_success')
                      : t('composites.validation_errors')}
                  </strong>
                  {(validation.errors ?? []).map((e, i) => (
                    <div key={`err-${i}`} className="validation-error">
                      {e.field ? `${e.field}: ` : ''}
                      {e.message}
                    </div>
                  ))}
                  {(validation.warnings ?? []).map((wn, i) => (
                    <div key={`warn-${i}`} className="validation-warning">
                      {wn.message}
                    </div>
                  ))}
                </div>
              )}

              <div className="composite-canvas-wrap">
                <CompositeCanvas
                  components={detail.components ?? []}
                  crossJoins={detail.cross_model_joins ?? []}
                  modelNames={modelNames}
                />
              </div>

              <div className="composite-section">
                <h3>{t('composites.components_title')}</h3>
                <ul className="component-list">
                  {(detail.components ?? []).map((c) => (
                    <li key={c.model_id}>
                      <span className="component-alias">{c.alias}</span>
                      <span className="component-model">
                        {modelNames[c.model_id] ?? c.model_id}
                      </span>
                      <span className={`component-role role-${c.role}`}>{c.role}</span>
                      <button
                        type="button"
                        className="btn-icon-danger"
                        onClick={() => {
                          void handleRemoveComponent(c.model_id)
                        }}
                        aria-label={t('composites.aria_remove')}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
                <div className="component-add-row">
                  <Select
                    value={addModelId}
                    onChange={(v) => {
                      setAddModelId(v)
                      const m = availableModels.find((x) => x.id === v)
                      if (m && !addAlias) {
                        setAddAlias(m.name)
                      }
                    }}
                    options={availableModels.map((m) => ({
                      value: m.id,
                      label: m.label ?? m.name,
                    }))}
                    placeholder={t('composites.model_select')}
                  />
                  <input
                    type="text"
                    placeholder={t('composites.alias_placeholder')}
                    value={addAlias}
                    onChange={(e) => setAddAlias(e.target.value)}
                  />
                  <Select
                    value={addRole}
                    onChange={(v) => setAddRole(v)}
                    options={[
                      { value: 'primary', label: 'primary' },
                      { value: 'secondary', label: 'secondary' },
                    ]}
                  />
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      void handleAddComponent()
                    }}
                    disabled={!addModelId || !addAlias.trim() || usedAliases.has(addAlias.trim())}
                  >
                    {t('composites.add')}
                  </button>
                </div>
              </div>

              <div className="composite-section">
                <div className="section-head-row">
                  <h3>{t('composites.cross_joins_title')}</h3>
                  <div>
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => {
                        void loadSuggestions()
                      }}
                    >
                      {t('composites.suggest')}
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => {
                        setEditingJoin(null)
                        setShowCrossJoin(true)
                      }}
                      disabled={(detail.components ?? []).length < 2}
                    >
                      {t('composites.add_join')}
                    </button>
                  </div>
                </div>
                <ul className="cross-join-list">
                  {(detail.cross_model_joins ?? []).map((j) => (
                    <li key={j.id}>
                      <span>
                        {j.from_model}.{j.from_dimension} → {j.to_model}.{j.to_dimension}
                      </span>
                      <span className="join-meta">
                        {j.join_type} · {j.relationship}
                      </span>
                      <button
                        type="button"
                        className="btn-link"
                        onClick={() => {
                          setEditingJoin(j)
                          setShowCrossJoin(true)
                        }}
                      >
                        {t('composites.edit')}
                      </button>
                      <button
                        type="button"
                        className="btn-icon-danger"
                        onClick={() => {
                          void handleRemoveCrossJoin(j.id)
                        }}
                        aria-label={t('composites.aria_delete')}
                      >
                        ×
                      </button>
                    </li>
                  ))}
                </ul>
                {suggestions.length > 0 && (
                  <div className="cross-join-suggestions">
                    <h4>{t('composites.suggested_joins')}</h4>
                    {suggestions.map((s, i) => (
                      <div key={`sug-${i}`} className="suggestion-row">
                        <span>
                          {s.from_model}.{s.from_dimension} → {s.to_model}.{s.to_dimension}
                        </span>
                        <span className="suggestion-reason">{s.reason}</span>
                        <button
                          type="button"
                          className="btn-link"
                          onClick={() => {
                            void applySuggestion(s)
                          }}
                        >
                          {t('composites.apply')}
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <div className="composite-section">
                <h3>{t('composites.canonical_date_title')}</h3>
                <p className="section-hint">{t('composites.canonical_date_hint')}</p>
                <div className="canonical-date-grid">
                  {(detail.components ?? []).map((c) => (
                    <div key={c.alias} className="canonical-date-model">
                      <strong>{c.alias}</strong>
                      <div className="canonical-date-dims">
                        {(dimensionsByAlias[c.alias] ?? []).map((dim) => {
                          const active =
                            detail.canonical_date?.model_alias === c.alias &&
                            detail.canonical_date?.dimension_name === dim
                          return (
                            <button
                              key={dim}
                              type="button"
                              className={`date-dim-chip ${active ? 'active' : ''}`}
                              onClick={() => {
                                void handleSetCanonicalDate(c.alias, dim)
                              }}
                            >
                              {dim}
                            </button>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="composite-section">
                <h3>{t('composites.conflicts_title')}</h3>
                <p className="section-hint">{t('composites.conflicts_hint')}</p>
                <ul className="resolution-list">
                  {(detail.conflict_resolutions ?? []).map((res) => (
                    <li key={res.dimension_name}>
                      <span className="resolution-name">{res.dimension_name}</span>
                      <Select
                        value={res.resolution}
                        onChange={(v) => {
                          void handleResolutionChange({
                            ...res,
                            resolution: v,
                          })
                        }}
                        options={[
                          { value: 'use_primary', label: t('composites.resolution_use_primary') },
                          { value: 'rename', label: t('composites.resolution_rename') },
                          { value: 'merge', label: t('composites.resolution_merge') },
                        ]}
                      />
                    </li>
                  ))}
                  {(detail.conflict_resolutions ?? []).length === 0 && (
                    <li className="resolution-empty">{t('composites.no_conflicts')}</li>
                  )}
                </ul>
              </div>
            </>
          )}
        </section>
      </div>

      <Modal
        open={showCreate}
        title={t('composites.create_title')}
        onClose={() => setShowCreate(false)}
      >
        <div className="composite-create-form">
          <div className="form-row">
            <div className="field-group">
              <label htmlFor="composite-name">{t('composites.field_name')}</label>
              <input
                id="composite-name"
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                autoFocus
                placeholder="e.g. sales_overview"
              />
            </div>
            <div className="field-group">
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
          <div className="field-group">
            <label htmlFor="composite-desc">{t('composites.field_description')}</label>
            <textarea
              id="composite-desc"
              value={newDescription}
              onChange={(e) => setNewDescription(e.target.value)}
              rows={3}
              placeholder={t('composites.description_placeholder')}
            />
          </div>
          <div className="form-actions">
            <button type="button" className="btn-secondary" onClick={() => setShowCreate(false)}>
              {t('composites.cancel')}
            </button>
            <button
              type="button"
              className="btn-primary"
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
