import { useCallback, useEffect, useState } from 'react'

import {
  type AIModel,
  type AIProvider,
  type AIPurpose,
  deleteModel,
  deleteProvider,
  listActiveModels,
  listModels,
  listProviders,
  setDefaultModel,
} from '../../api/aiProviders'
import { useConfirm } from '../../hooks/useConfirm'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { AIModelSharingPanel } from './AIModelSharingPanel'
import { PURPOSES } from './aiProviderModalShared'
import { ModelModal } from './ModelModal'
import { ProviderModal } from './ProviderModal'
import { ReadOnlyNote } from './ReadOnlyNote'

const adminAiCardClass = 'border border-border rounded-[10px] bg-card overflow-hidden'
const adminAiCardHeaderClass = 'py-3.5 px-[18px] border-b border-border'
const adminAiCardTitleClass = 'block m-0 text-[0.95rem] font-semibold text-foreground'
const adminAiCardDescClass =
  'mt-1.5 mb-0 text-caption leading-[1.45] text-foreground-muted max-w-[52rem]'
const adminAiCardBodyClass = 'py-4 px-[18px] pb-[18px]'
const adminAiPurposeGridClass = 'grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3'
const adminAiPurposeCardClass =
  'flex flex-col gap-2.5 py-3 px-3.5 border border-border rounded-[10px] bg-card-raised min-h-[92px]'
const adminAiPurposePillBase =
  'inline-flex items-center max-w-[11.5rem] py-0.5 px-2 rounded-full text-[0.7rem] font-semibold leading-[1.35] tracking-wide whitespace-nowrap overflow-hidden text-ellipsis'

export function AIProvidersPanel() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { hasPermission } = useAuth()
  // Managing AI providers/models is a platform-config action (admin:settings).
  const canEdit = hasPermission('admin:settings')

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
      setProviders(provs)
      setActiveModels(active)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const reloadModels = useCallback(
    async (providerID: string) => {
      setModelsLoading(true)
      try {
        const rows = await listModels(providerID)
        setModels(rows)
      } catch (e) {
        toast.error(e instanceof Error ? e.message : String(e))
      } finally {
        setModelsLoading(false)
      }
    },
    [toast],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void reloadTop()
  }, [reloadTop])

  useEffect(() => {
    if (selectedProvider) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      void reloadModels(selectedProvider.id)
    } else {
      setModels([])
    }
  }, [selectedProvider, reloadModels])

  const handleDeleteProvider = async (p: AIProvider) => {
    const ok = await confirm({
      title: t('admin.ai_providers.title'),
      message: t('admin.ai_providers.confirm_delete_provider', { name: p.name }),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await deleteProvider(p.id)
      toast.success(t('admin.ai_providers.deleted'))
      if (selectedProvider?.id === p.id) {
        setSelectedProvider(null)
      }
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
    if (!ok) {
      return
    }
    try {
      await deleteModel(m.id)
      toast.success(t('admin.ai_providers.deleted'))
      if (selectedProvider) {
        await reloadModels(selectedProvider.id)
      }
      await reloadTop()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const handleSetDefault = async (m: AIModel) => {
    try {
      await setDefaultModel(m.id)
      toast.success(t('admin.ai_providers.saved'))
      if (selectedProvider) {
        await reloadModels(selectedProvider.id)
      }
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
      if (refreshed) {
        setSelectedProvider(refreshed)
      }
    }
  }

  const onModelSaved = async () => {
    setModelModalOpen(false)
    setEditingModel(null)
    if (selectedProvider) {
      await reloadModels(selectedProvider.id)
    }
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

      {error && (
        <div style={errStyle}>
          {t('common.error')}: {error}
        </div>
      )}
      {!canEdit && <ReadOnlyNote />}

      <AIModelSharingPanel />

      <section className={adminAiCardClass}>
        <div className={adminAiCardHeaderClass}>
          <strong className={adminAiCardTitleClass}>
            {t('admin.ai_providers.active_models_title')}
          </strong>
          <p className={adminAiCardDescClass}>{t('admin.ai_providers.active_models_hint')}</p>
        </div>
        <div className={adminAiCardBodyClass}>
          {activeModels.length === 0 ? (
            <p className="text-foreground-muted text-caption m-0">
              {t('admin.ai_providers.active_models_empty')}
            </p>
          ) : (
            <div className={adminAiPurposeGridClass}>
              {PURPOSES.map((purpose) => {
                const m = activeByPurpose(purpose)
                const providerLabel = m?.provider_name.trim()
                const modelLabel = m?.display_name.trim()
                const modelHint = m?.model_id.trim()
                return (
                  <div key={purpose} className={adminAiPurposeCardClass}>
                    <div className="flex items-start justify-between gap-2.5">
                      <div className="text-foreground min-w-0 text-[0.82rem] leading-tight font-bold">
                        {t(`admin.ai_providers.purposes.${purpose}`)}
                      </div>
                      <div className="flex shrink-0 items-center justify-end gap-1.5">
                        {providerLabel ? (
                          <span
                            className={`${adminAiPurposePillBase} bg-success/12 text-success border-success/16 border`}
                          >
                            {providerLabel}
                          </span>
                        ) : (
                          <span
                            className={`${adminAiPurposePillBase} bg-foreground-muted/10 text-foreground-muted border-border border`}
                          >
                            —
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="flex min-w-0 flex-col gap-1.5">
                      {m ? (
                        <>
                          <div
                            className="text-foreground text-[0.92rem] leading-tight font-semibold wrap-break-word"
                            title={modelHint ?? modelLabel}
                          >
                            {modelLabel ?? modelHint ?? '—'}
                          </div>
                          {modelHint && modelHint !== modelLabel && (
                            <div className="text-foreground-muted text-xs">
                              <code
                                className={`border-border bg-canvas-subtle rounded-lg border px-1.5 py-px text-[0.72rem]`}
                              >
                                {modelHint}
                              </code>
                            </div>
                          )}
                        </>
                      ) : (
                        <div className="text-foreground-muted text-[0.9rem]">
                          {t('common.em_dash')}
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </section>

      {/* Providers grid */}
      <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <h3 style={{ margin: 0, fontSize: 15 }}>{t('admin.ai_providers.providers_title')}</h3>
          <button
            style={primaryBtn}
            disabled={!canEdit}
            onClick={() => {
              setEditingProvider(null)
              setProviderModalOpen(true)
            }}
          >
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
                onEdit={() => {
                  setEditingProvider(p)
                  setProviderModalOpen(true)
                }}
                onDelete={() => {
                  void handleDeleteProvider(p)
                }}
                canEdit={canEdit}
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
          <div
            style={{
              ...cardHeaderStyle,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <span>{t('admin.ai_providers.models_for', { name: selectedProvider.name })}</span>
            <button
              style={secondaryBtn}
              disabled={!canEdit}
              onClick={() => {
                setEditingModel(null)
                setModelModalOpen(true)
              }}
            >
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
                      {m.is_default && (
                        <span style={defaultBadge}>{t('admin.ai_providers.default_badge')}</span>
                      )}
                      {!m.is_active && (
                        <span style={inactiveBadge}>{t('admin.ai_providers.inactive')}</span>
                      )}
                    </td>
                    <td
                      style={{
                        ...tdStyle,
                        fontFamily: 'var(--font-mono, monospace)',
                        fontSize: 12,
                      }}
                    >
                      {m.model_id}
                    </td>
                    <td style={tdStyle}>{t(`admin.ai_providers.purposes.${m.purpose}`)}</td>
                    <td style={tdStyle}>{m.max_tokens}</td>
                    <td style={{ ...tdStyle, textAlign: 'right', whiteSpace: 'nowrap' }}>
                      {!m.is_default && (
                        <button
                          style={linkBtn}
                          disabled={!canEdit}
                          onClick={() => {
                            void handleSetDefault(m)
                          }}
                        >
                          {t('admin.ai_providers.set_default')}
                        </button>
                      )}
                      <button
                        style={linkBtn}
                        disabled={!canEdit}
                        onClick={() => {
                          setEditingModel(m)
                          setModelModalOpen(true)
                        }}
                      >
                        {t('common.edit')}
                      </button>
                      <button
                        style={{ ...linkBtn, color: 'var(--error, #ef4444)' }}
                        disabled={!canEdit}
                        onClick={() => {
                          void handleDeleteModel(m)
                        }}
                      >
                        {t('common.delete')}
                      </button>
                    </td>
                  </tr>
                ))}
                {!modelsLoading && models.length === 0 && (
                  <tr>
                    <td
                      colSpan={5}
                      style={{
                        padding: 20,
                        textAlign: 'center',
                        color: 'var(--text-secondary, #a1a1aa)',
                      }}
                    >
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
          onClose={() => {
            setProviderModalOpen(false)
            setEditingProvider(null)
          }}
          onSaved={() => {
            void onProviderSaved()
          }}
        />
      )}

      {modelModalOpen && selectedProvider && (
        <ModelModal
          provider={selectedProvider}
          model={editingModel}
          onClose={() => {
            setModelModalOpen(false)
            setEditingModel(null)
          }}
          onSaved={() => {
            void onModelSaved()
          }}
        />
      )}
    </div>
  )
}

function ProviderCard({
  provider,
  selected,
  onSelect,
  onEdit,
  onDelete,
  canEdit,
}: {
  provider: AIProvider
  selected: boolean
  onSelect: () => void
  onEdit: () => void
  onDelete: () => void
  canEdit: boolean
}) {
  const t = useT()
  return (
    <div
      style={{
        ...providerCardStyle,
        borderColor: selected ? 'var(--accent)' : 'var(--border, rgba(255,255,255,0.06))',
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
      <div
        style={{
          fontSize: 12,
          color: 'var(--text-secondary, #a1a1aa)',
          fontFamily: 'var(--font-mono, monospace)',
          wordBreak: 'break-all',
        }}
      >
        {provider.base_url || '—'}
      </div>
      <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>
        {t('admin.ai_providers.model_count', { count: provider.model_count })}
        {provider.has_api_key && <span> · {provider.api_key_masked}</span>}
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 4 }} onClick={(e) => e.stopPropagation()}>
        <button style={linkBtn} disabled={!canEdit} onClick={onEdit}>
          {t('common.edit')}
        </button>
        <button
          style={{ ...linkBtn, color: 'var(--error, #ef4444)' }}
          disabled={!canEdit}
          onClick={onDelete}
        >
          {t('common.delete')}
        </button>
      </div>
    </div>
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
const thStyle: React.CSSProperties = {
  padding: '10px 16px',
  fontWeight: 600,
  color: 'var(--text-secondary, #a1a1aa)',
}
const trRow: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255,255,255,0.06))',
}
const tdStyle: React.CSSProperties = { padding: '10px 16px', color: 'var(--text-primary, #f4f4f5)' }
const primaryBtn: React.CSSProperties = {
  padding: '8px 14px',
  background: 'var(--accent)',
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
  color: 'var(--accent)',
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
const activeDot: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--success, #10b981)',
  fontWeight: 600,
}
const inactiveDot: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-secondary, #a1a1aa)',
  fontWeight: 600,
}
const errStyle: React.CSSProperties = {
  color: 'var(--error, crimson)',
  padding: 12,
  fontWeight: 600,
}
