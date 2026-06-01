import { useCallback, useEffect, useMemo, useState } from 'react'
import { useT } from '../../i18n'
import { useToast } from '../../hooks/useToast'
import { useConfirm } from '../../hooks/useConfirm'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import {
  type AIModel,
  type AIProvider,
  type AIProviderType,
  type AIPurpose,
  type ConnectionTestResult,
  createModel,
  createProvider,
  deleteModel,
  deleteProvider,
  listActiveModels,
  listModels,
  listProviderRemoteModels,
  listProviders,
  setDefaultModel,
  testProvider,
  updateModel,
  updateProvider,
  type RemoteModelOption,
} from '../../api/aiProviders'

const PROVIDER_TYPES: AIProviderType[] = ['openai', 'openai-compatible', 'anthropic']
const PURPOSES: AIPurpose[] = ['query', 'describe', 'embedding', 'translation', 'judge']

function defaultBaseURL(type: AIProviderType): string {
  switch (type) {
    case 'openai':
      return 'https://api.openai.com/v1'
    case 'anthropic':
      return 'https://api.anthropic.com/v1'
    default:
      return ''
  }
}

export function AIProvidersPanel() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()

  const [providers, setProviders] = useState<AIProvider[]>([])
  const [activeModels, setActiveModels] = useState<AIModel[]>([])
  const [selectedProvider, setSelectedProvider] = useState<AIProvider | null>(null)
  const [models, setModels] = useState<AIModel[]>([])
  const [loading, setLoading] = useState(true)
  const [modelsLoading, setModelsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [providerModalOpen, setProviderModalOpen] = useState(false)
  const [editingProvider, setEditingProvider] = useState<AIProvider | null>(null)
  const [modelModalOpen, setModelModalOpen] = useState(false)
  const [editingModel, setEditingModel] = useState<AIModel | null>(null)

  const reloadTop = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [provs, active] = await Promise.all([listProviders(), listActiveModels()])
      setProviders(provs ?? [])
      setActiveModels(active ?? [])
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const reloadModels = useCallback(async (providerID: string) => {
    setModelsLoading(true)
    try {
      const rows = await listModels(providerID)
      setModels(rows ?? [])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setModelsLoading(false)
    }
  }, [toast])

  useEffect(() => {
    reloadTop()
  }, [reloadTop])

  useEffect(() => {
    if (selectedProvider) reloadModels(selectedProvider.id)
    else setModels([])
  }, [selectedProvider, reloadModels])

  const handleDeleteProvider = async (p: AIProvider) => {
    const ok = await confirm({
      title: t('admin.ai_providers.title'),
      message: t('admin.ai_providers.confirm_delete_provider', { name: p.name }),
      variant: 'danger',
    })
    if (!ok) return
    try {
      await deleteProvider(p.id)
      toast.success(t('admin.ai_providers.deleted'))
      if (selectedProvider?.id === p.id) setSelectedProvider(null)
      await reloadTop()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleDeleteModel = async (m: AIModel) => {
    const ok = await confirm({
      title: t('admin.ai_providers.title'),
      message: t('admin.ai_providers.confirm_delete_model', { name: m.display_name }),
      variant: 'danger',
    })
    if (!ok) return
    try {
      await deleteModel(m.id)
      toast.success(t('admin.ai_providers.deleted'))
      if (selectedProvider) await reloadModels(selectedProvider.id)
      await reloadTop()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleSetDefault = async (m: AIModel) => {
    try {
      await setDefaultModel(m.id)
      toast.success(t('admin.ai_providers.saved'))
      if (selectedProvider) await reloadModels(selectedProvider.id)
      await reloadTop()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const onProviderSaved = async () => {
    setProviderModalOpen(false)
    setEditingProvider(null)
    await reloadTop()
    if (selectedProvider) {
      const refreshed = providers.find((p) => p.id === selectedProvider.id)
      if (refreshed) setSelectedProvider(refreshed)
    }
  }

  const onModelSaved = async () => {
    setModelModalOpen(false)
    setEditingModel(null)
    if (selectedProvider) await reloadModels(selectedProvider.id)
    await reloadTop()
  }

  const activeByPurpose = (purpose: AIPurpose) => activeModels.find((m) => m.purpose === purpose)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
      <div>
        <h2 style={{ margin: 0, fontSize: 18 }}>{t('admin.ai_providers.title')}</h2>
        <p style={{ margin: '6px 0 0', fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
          {t('admin.ai_providers.description')}
        </p>
      </div>

      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}

      {/* Active models by purpose */}
      <section style={cardStyle}>
        <div style={cardHeaderStyle}>{t('admin.ai_providers.active_models_title')}</div>
        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {PURPOSES.map((purpose) => {
            const m = activeByPurpose(purpose)
            return (
              <div key={purpose} style={purposeRowStyle}>
                <span style={{ fontWeight: 600, fontSize: 13, minWidth: 160 }}>
                  {t(`admin.ai_providers.purposes.${purpose}`)}
                </span>
                {m ? (
                  <span style={{ fontSize: 13, color: 'var(--text-primary, #f4f4f5)' }}>
                    {m.display_name}{' '}
                    <span style={{ color: 'var(--text-secondary, #a1a1aa)' }}>({m.provider_name})</span>
                  </span>
                ) : (
                  <span style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>—</span>
                )}
              </div>
            )
          })}
          {activeModels.length === 0 && (
            <div style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
              {t('admin.ai_providers.active_models_empty')}
            </div>
          )}
        </div>
      </section>

      {/* Providers grid */}
      <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h3 style={{ margin: 0, fontSize: 15 }}>{t('admin.ai_providers.providers_title')}</h3>
          <button style={primaryBtn} onClick={() => { setEditingProvider(null); setProviderModalOpen(true) }}>
            + {t('admin.ai_providers.add_provider')}
          </button>
        </div>

        <LoadingOverlay loading={loading}>
          <div style={gridStyle}>
            {providers.map((p) => (
              <ProviderCard
                key={p.id}
                provider={p}
                selected={selectedProvider?.id === p.id}
                onSelect={() => setSelectedProvider((cur) => (cur?.id === p.id ? null : p))}
                onEdit={() => { setEditingProvider(p); setProviderModalOpen(true) }}
                onDelete={() => handleDeleteProvider(p)}
              />
            ))}
            {!loading && providers.length === 0 && (
              <div style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)', padding: 16 }}>
                {t('admin.ai_providers.no_providers')}
              </div>
            )}
          </div>
        </LoadingOverlay>
      </section>

      {/* Models for selected provider */}
      {selectedProvider && (
        <section style={cardStyle}>
          <div style={{ ...cardHeaderStyle, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span>{t('admin.ai_providers.models_for', { name: selectedProvider.name })}</span>
            <button style={secondaryBtn} onClick={() => { setEditingModel(null); setModelModalOpen(true) }}>
              + {t('admin.ai_providers.add_model')}
            </button>
          </div>
          <LoadingOverlay loading={modelsLoading}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={theadRow}>
                  <th style={thStyle}>{t('admin.ai_providers.fields.display_name')}</th>
                  <th style={thStyle}>{t('admin.ai_providers.fields.model_id')}</th>
                  <th style={thStyle}>{t('admin.ai_providers.fields.purpose')}</th>
                  <th style={thStyle}>{t('admin.ai_providers.fields.max_tokens')}</th>
                  <th style={{ ...thStyle, textAlign: 'right' }} />
                </tr>
              </thead>
              <tbody>
                {models.map((m) => (
                  <tr key={m.id} style={trRow}>
                    <td style={tdStyle}>
                      {m.display_name}
                      {m.is_default && <span style={defaultBadge}>{t('admin.ai_providers.default_badge')}</span>}
                      {!m.is_active && <span style={inactiveBadge}>{t('admin.ai_providers.inactive')}</span>}
                    </td>
                    <td style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>{m.model_id}</td>
                    <td style={tdStyle}>{t(`admin.ai_providers.purposes.${m.purpose}`)}</td>
                    <td style={tdStyle}>{m.max_tokens}</td>
                    <td style={{ ...tdStyle, textAlign: 'right', whiteSpace: 'nowrap' }}>
                      {!m.is_default && (
                        <button style={linkBtn} onClick={() => handleSetDefault(m)}>
                          {t('admin.ai_providers.set_default')}
                        </button>
                      )}
                      <button style={linkBtn} onClick={() => { setEditingModel(m); setModelModalOpen(true) }}>
                        {t('common.edit')}
                      </button>
                      <button style={{ ...linkBtn, color: 'var(--error, #ef4444)' }} onClick={() => handleDeleteModel(m)}>
                        {t('common.delete')}
                      </button>
                    </td>
                  </tr>
                ))}
                {!modelsLoading && models.length === 0 && (
                  <tr>
                    <td colSpan={5} style={{ padding: 20, textAlign: 'center', color: 'var(--text-secondary, #a1a1aa)' }}>
                      {t('admin.ai_providers.no_models')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </LoadingOverlay>
        </section>
      )}

      {providerModalOpen && (
        <ProviderModal
          provider={editingProvider}
          onClose={() => { setProviderModalOpen(false); setEditingProvider(null) }}
          onSaved={onProviderSaved}
        />
      )}

      {modelModalOpen && selectedProvider && (
        <ModelModal
          provider={selectedProvider}
          model={editingModel}
          onClose={() => { setModelModalOpen(false); setEditingModel(null) }}
          onSaved={onModelSaved}
        />
      )}
    </div>
  )
}

function ProviderCard({
  provider, selected, onSelect, onEdit, onDelete,
}: {
  provider: AIProvider
  selected: boolean
  onSelect: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const t = useT()
  return (
    <div
      style={{
        ...providerCardStyle,
        borderColor: selected ? 'var(--accent, #6366f1)' : 'var(--border, rgba(255,255,255,0.06))',
      }}
      onClick={onSelect}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start' }}>
        <strong style={{ fontSize: 14 }}>{provider.name}</strong>
        <span style={provider.is_active ? activeDot : inactiveDot}>
          {provider.is_active ? t('admin.ai_providers.active') : t('admin.ai_providers.inactive')}
        </span>
      </div>
      <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>
        {t(`admin.ai_providers.types.${provider.provider_type}`)}
      </div>
      <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)', fontFamily: 'var(--font-mono, monospace)', wordBreak: 'break-all' }}>
        {provider.base_url || '—'}
      </div>
      <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>
        {t('admin.ai_providers.model_count', { count: provider.model_count })}
        {provider.has_api_key && <span> · {provider.api_key_masked}</span>}
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 4 }} onClick={(e) => e.stopPropagation()}>
        <button style={linkBtn} onClick={onEdit}>{t('common.edit')}</button>
        <button style={{ ...linkBtn, color: 'var(--error, #ef4444)' }} onClick={onDelete}>{t('common.delete')}</button>
      </div>
    </div>
  )
}

function ProviderModal({
  provider, onClose, onSaved,
}: {
  provider: AIProvider | null
  onClose: () => void
  onSaved: () => void
}) {
  const t = useT()
  const toast = useToast()
  const editing = !!provider
  const [name, setName] = useState(provider?.name ?? '')
  const [type, setType] = useState<AIProviderType>(provider?.provider_type ?? 'openai')
  const [baseURL, setBaseURL] = useState(provider?.base_url ?? defaultBaseURL('openai'))
  const [apiKey, setApiKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [isActive, setIsActive] = useState(provider?.is_active ?? true)
  const [timeout, setTimeoutVal] = useState(provider?.http_timeout_seconds ?? 120)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)

  const onTypeChange = (next: AIProviderType) => {
    setType(next)
    // Autofill base URL only when empty or still the previous type's default.
    if (!baseURL || baseURL === defaultBaseURL(type)) setBaseURL(defaultBaseURL(next))
  }

  const save = async () => {
    if (!name.trim()) { toast.error(t('admin.ai_providers.fields.name')); return }
    setSaving(true)
    try {
      if (editing) {
        await updateProvider(provider.id, {
          name, provider_type: type, base_url: baseURL,
          api_key: apiKey ? apiKey : null,
          is_active: isActive, http_timeout_seconds: timeout,
        })
      } else {
        await createProvider({
          name, provider_type: type, base_url: baseURL,
          api_key: apiKey || undefined, is_active: isActive, http_timeout_seconds: timeout,
        })
      }
      toast.success(t('admin.ai_providers.saved'))
      onSaved()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  const runTest = async () => {
    if (!editing) {
      toast.info(t('admin.ai_providers.test_connection'))
      return
    }
    setTesting(true)
    setTestResult(null)
    try {
      const res = await testProvider(provider.id)
      setTestResult(res)
    } catch (e) {
      setTestResult({ status: 'error', message: e instanceof Error ? e.message : String(e) })
    } finally {
      setTesting(false)
    }
  }

  return (
    <Modal open title={editing ? t('admin.ai_providers.edit_provider') : t('admin.ai_providers.add_provider')} onClose={onClose}>
      <div style={formStyle}>
        <Field label={t('admin.ai_providers.fields.name')}>
          <input style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} autoFocus />
        </Field>
        <Field label={t('admin.ai_providers.fields.type')}>
          <Select<AIProviderType>
            value={type}
            onChange={onTypeChange}
            options={PROVIDER_TYPES.map((pt) => ({ value: pt, label: t(`admin.ai_providers.types.${pt}`) }))}
          />
        </Field>
        <Field label={t('admin.ai_providers.fields.base_url')}>
          <input style={inputStyle} value={baseURL} onChange={(e) => setBaseURL(e.target.value)} placeholder="https://…/v1" />
        </Field>
        <Field label={t('admin.ai_providers.fields.api_key')}>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              style={{ ...inputStyle, flex: 1 }}
              type={showKey ? 'text' : 'password'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={editing ? t('admin.ai_providers.fields.api_key_keep') : ''}
            />
            <button type="button" style={secondaryBtn} onClick={() => setShowKey((s) => !s)}>
              {showKey ? t('admin.ai_providers.hide_key') : t('admin.ai_providers.show_key')}
            </button>
          </div>
        </Field>
        <Field label={t('admin.ai_providers.fields.http_timeout')}>
          <input style={inputStyle} type="number" value={timeout} onChange={(e) => setTimeoutVal(Number(e.target.value))} />
        </Field>
        <label style={checkboxRow}>
          <input type="checkbox" checked={isActive} onChange={(e) => setIsActive(e.target.checked)} />
          {t('admin.ai_providers.fields.is_active')}
        </label>

        {editing && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <button type="button" style={secondaryBtn} disabled={testing} onClick={runTest}>
              {testing ? t('admin.ai_providers.testing') : t('admin.ai_providers.test_connection')}
            </button>
            {testResult && (
              <span style={{ fontSize: 13, color: testResult.status === 'connected' ? 'var(--success, #10b981)' : 'var(--error, #ef4444)' }}>
                {testResult.status === 'connected'
                  ? t('admin.ai_providers.test_connected', { ms: testResult.latency_ms ?? 0 })
                  : `${t('admin.ai_providers.test_failed')}: ${testResult.message ?? ''}`}
              </span>
            )}
          </div>
        )}

        <div style={modalActions}>
          <button style={secondaryBtn} onClick={onClose}>{t('common.cancel')}</button>
          <button style={primaryBtn} disabled={saving} onClick={save}>{t('common.save')}</button>
        </div>
      </div>
    </Modal>
  )
}

function ModelModal({
  provider, model, onClose, onSaved,
}: {
  provider: AIProvider
  model: AIModel | null
  onClose: () => void
  onSaved: () => void
}) {
  const t = useT()
  const toast = useToast()
  const editing = !!model
  const [modelID, setModelID] = useState(model?.model_id ?? '')
  const [displayName, setDisplayName] = useState(model?.display_name ?? '')
  const [purpose, setPurpose] = useState<AIPurpose>(model?.purpose ?? 'query')
  const [maxTokens, setMaxTokens] = useState(model?.max_tokens ?? 4096)
  const [temperature, setTemperature] = useState(model?.temperature ?? 0)
  const [maxPromptRunes, setMaxPromptRunes] = useState(model?.max_prompt_input_runes ?? 80000)
  const [isDefault, setIsDefault] = useState(model?.is_default ?? false)
  const [saving, setSaving] = useState(false)
  const [remoteModels, setRemoteModels] = useState<RemoteModelOption[]>([])
  const [loadingRemote, setLoadingRemote] = useState(false)
  const [remoteError, setRemoteError] = useState<string | null>(null)
  const [useManualModelID, setUseManualModelID] = useState(false)

  const fetchRemoteModels = useCallback(async () => {
    setLoadingRemote(true)
    setRemoteError(null)
    try {
      const rows = await listProviderRemoteModels(provider.id)
      setRemoteModels(rows ?? [])
      if ((rows?.length ?? 0) === 0) {
        setRemoteError(t('admin.ai_providers.fields.remote_models_failed'))
        setUseManualModelID(true)
      }
    } catch (e) {
      setRemoteModels([])
      setRemoteError(e instanceof Error ? e.message : String(e))
      setUseManualModelID(true)
    } finally {
      setLoadingRemote(false)
    }
  }, [provider.id, t])

  useEffect(() => {
    void fetchRemoteModels()
  }, [fetchRemoteModels])

  const remoteModelOptions = useMemo(() => {
    const opts = remoteModels.map((m) => ({
      value: m.id,
      label: m.owned_by ? `${m.id} · ${m.owned_by}` : m.id,
    }))
    const cur = modelID.trim()
    if (cur && !opts.some((o) => o.value === cur)) {
      opts.unshift({ value: cur, label: cur })
    }
    return opts
  }, [remoteModels, modelID])

  const pickModelID = (next: string) => {
    setModelID(next)
    if (!displayName.trim() && next.trim()) {
      setDisplayName(next.trim())
    }
  }

  const save = async () => {
    if (!modelID.trim()) { toast.error(t('admin.ai_providers.fields.model_id')); return }
    setSaving(true)
    try {
      if (editing) {
        await updateModel(model.id, {
          model_id: modelID, display_name: displayName || modelID, purpose,
          max_tokens: maxTokens, temperature, max_prompt_input_runes: maxPromptRunes,
        })
      } else {
        await createModel({
          provider_id: provider.id, model_id: modelID, display_name: displayName || modelID,
          purpose, max_tokens: maxTokens, temperature, max_prompt_input_runes: maxPromptRunes,
          is_default: isDefault,
        })
      }
      toast.success(t('admin.ai_providers.saved'))
      onSaved()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal open title={editing ? t('admin.ai_providers.edit_model') : t('admin.ai_providers.add_model')} onClose={onClose}>
      <div style={formStyle}>
        <Field label={t('admin.ai_providers.fields.model_id')}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div style={{ display: 'flex', gap: 8, alignItems: 'stretch' }}>
              {remoteModels.length > 0 && !useManualModelID ? (
                <div style={{ flex: 1, minWidth: 0 }}>
                  <Select
                    value={modelID}
                    onChange={pickModelID}
                    options={remoteModelOptions}
                    placeholder={t('admin.ai_providers.fields.model_id')}
                  />
                </div>
              ) : (
                <input
                  style={{ ...inputStyle, flex: 1 }}
                  value={modelID}
                  onChange={(e) => pickModelID(e.target.value)}
                  autoFocus
                  placeholder="gpt-4o"
                  list={remoteModels.length > 0 ? `remote-models-${provider.id}` : undefined}
                />
              )}
              <button
                type="button"
                style={{ ...secondaryBtn, flexShrink: 0, alignSelf: 'stretch' }}
                disabled={loadingRemote}
                onClick={() => void fetchRemoteModels()}
              >
                {loadingRemote ? t('admin.ai_providers.fields.fetching_remote_models') : t('admin.ai_providers.fields.fetch_remote_models')}
              </button>
            </div>
            {remoteModels.length > 0 && !remoteError && (
              <small className="form-hint" style={{ margin: 0 }}>
                {t('admin.ai_providers.fields.remote_models_count', { count: remoteModels.length })}
                {' · '}
                <button
                  type="button"
                  style={{ ...linkBtn, padding: 0, fontSize: 'inherit' }}
                  onClick={() => setUseManualModelID((v) => !v)}
                >
                  {useManualModelID
                    ? t('admin.ai_providers.fields.pick_from_list')
                    : t('admin.ai_providers.fields.enter_model_manually')}
                </button>
              </small>
            )}
            {remoteModels.length > 0 && useManualModelID && (
              <datalist id={`remote-models-${provider.id}`}>
                {remoteModels.map((m) => (
                  <option key={m.id} value={m.id} label={m.owned_by} />
                ))}
              </datalist>
            )}
            {remoteError && (
              <small className="form-hint form-hint--warning" style={{ margin: 0 }}>
                {remoteError}
              </small>
            )}
            <small className="form-hint" style={{ margin: 0 }}>
              {t('admin.ai_providers.fields.model_id_hint')}
            </small>
          </div>
        </Field>
        <Field label={t('admin.ai_providers.fields.display_name')}>
          <input style={inputStyle} value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </Field>
        <Field label={t('admin.ai_providers.fields.purpose')}>
          <Select<AIPurpose>
            value={purpose}
            onChange={setPurpose}
            options={PURPOSES.map((p) => ({ value: p, label: t(`admin.ai_providers.purposes.${p}`) }))}
          />
        </Field>
        <div style={{ display: 'flex', gap: 12 }}>
          <Field label={t('admin.ai_providers.fields.max_tokens')} style={{ flex: 1 }}>
            <input style={inputStyle} type="number" value={maxTokens} onChange={(e) => setMaxTokens(Number(e.target.value))} />
          </Field>
          <Field label={t('admin.ai_providers.fields.temperature')} style={{ flex: 1 }}>
            <input style={inputStyle} type="number" step="0.1" value={temperature} onChange={(e) => setTemperature(Number(e.target.value))} />
          </Field>
        </div>
        <Field label={t('admin.ai_providers.fields.max_prompt_runes')}>
          <input style={inputStyle} type="number" value={maxPromptRunes} onChange={(e) => setMaxPromptRunes(Number(e.target.value))} />
        </Field>
        {!editing && (
          <label style={checkboxRow}>
            <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />
            {t('admin.ai_providers.fields.set_default')}
          </label>
        )}
        <div style={modalActions}>
          <button style={secondaryBtn} onClick={onClose}>{t('common.cancel')}</button>
          <button style={primaryBtn} disabled={saving} onClick={save}>{t('common.save')}</button>
        </div>
      </div>
    </Modal>
  )
}

function Field({ label, children, style }: { label: string; children: React.ReactNode; style?: React.CSSProperties }) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 4, ...style }}>
      <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-secondary, #a1a1aa)' }}>{label}</span>
      {children}
    </label>
  )
}

const cardStyle: React.CSSProperties = {
  background: 'var(--bg-card, #18181b)',
  border: '1px solid var(--border, rgba(255,255,255,0.06))',
  borderRadius: 8,
  overflow: 'hidden',
}
const cardHeaderStyle: React.CSSProperties = {
  padding: '12px 16px',
  borderBottom: '1px solid var(--border, rgba(255,255,255,0.06))',
  fontWeight: 600,
  fontSize: 14,
}
const purposeRowStyle: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 12 }
const gridStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
  gap: 12,
}
const providerCardStyle: React.CSSProperties = {
  background: 'var(--bg-card, #18181b)',
  border: '1px solid',
  borderRadius: 8,
  padding: 14,
  display: 'flex',
  flexDirection: 'column',
  gap: 6,
  cursor: 'pointer',
  transition: 'border-color 150ms',
}
const theadRow: React.CSSProperties = {
  background: 'var(--table-header-bg, rgba(255,255,255,0.03))',
  borderBottom: '1px solid var(--border, rgba(255,255,255,0.06))',
  textAlign: 'left',
}
const thStyle: React.CSSProperties = { padding: '10px 16px', fontWeight: 600, color: 'var(--text-secondary, #a1a1aa)' }
const trRow: React.CSSProperties = { borderBottom: '1px solid var(--border, rgba(255,255,255,0.06))' }
const tdStyle: React.CSSProperties = { padding: '10px 16px', color: 'var(--text-primary, #f4f4f5)' }
const formStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 14 }
const inputStyle: React.CSSProperties = {
  padding: '8px 10px',
  background: 'var(--bg-input, rgba(255,255,255,0.04))',
  border: '1px solid var(--border, rgba(255,255,255,0.1))',
  borderRadius: 6,
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 13,
  width: '100%',
}
const checkboxRow: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }
const modalActions: React.CSSProperties = { display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }
const primaryBtn: React.CSSProperties = {
  padding: '8px 14px',
  background: 'var(--accent, #6366f1)',
  color: '#fff',
  border: 'none',
  borderRadius: 6,
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
}
const secondaryBtn: React.CSSProperties = {
  padding: '8px 14px',
  background: 'transparent',
  color: 'var(--text-primary, #f4f4f5)',
  border: '1px solid var(--border-strong, rgba(255,255,255,0.12))',
  borderRadius: 6,
  fontSize: 13,
  cursor: 'pointer',
}
const linkBtn: React.CSSProperties = {
  padding: '4px 8px',
  background: 'transparent',
  border: 'none',
  color: 'var(--accent, #6366f1)',
  fontSize: 12,
  cursor: 'pointer',
}
const defaultBadge: React.CSSProperties = {
  marginLeft: 8,
  padding: '1px 6px',
  borderRadius: 10,
  fontSize: 10,
  fontWeight: 600,
  background: 'rgba(16,185,129,0.14)',
  color: 'var(--success, #10b981)',
}
const inactiveBadge: React.CSSProperties = {
  marginLeft: 8,
  padding: '1px 6px',
  borderRadius: 10,
  fontSize: 10,
  fontWeight: 600,
  background: 'rgba(107,114,128,0.15)',
  color: 'var(--text-secondary, #a1a1aa)',
}
const activeDot: React.CSSProperties = { fontSize: 11, color: 'var(--success, #10b981)', fontWeight: 600 }
const inactiveDot: React.CSSProperties = { fontSize: 11, color: 'var(--text-secondary, #a1a1aa)', fontWeight: 600 }
const errStyle: React.CSSProperties = { color: 'var(--error, crimson)', padding: 12, fontWeight: 600 }
