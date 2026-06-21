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
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { useModal } from '../../hooks/useModal'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { errorMessage } from '../../utils/error'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { adminBtnPrimaryClass, adminBtnSecondaryClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
import { AIModelSharingPanel } from './AIModelSharingPanel'
import { PURPOSES } from './aiProviderModalShared'
import { ModelModal } from './ModelModal'
import { ProviderModal } from './ProviderModal'

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

  const providerModal = useModal<AIProvider>()
  const modelModal = useModal<AIModel>()
  const confirmMutation = useConfirmedMutation()

  const reloadTop = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [provs, active] = await Promise.all([listProviders(), listActiveModels()])
      setProviders(provs)
      setActiveModels(active)
    } catch (e) {
      setError(errorMessage(e))
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
        toast.error(errorMessage(e))
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
    const ok = await confirmMutation(() => deleteProvider(p.id), {
      title: t('admin.ai_providers.title'),
      message: t('admin.ai_providers.confirm_delete_provider', { name: p.name }),
      successMessage: t('admin.ai_providers.deleted'),
    })
    if (ok) {
      if (selectedProvider?.id === p.id) {
        setSelectedProvider(null)
      }
      await reloadTop()
    }
  }

  const handleDeleteModel = async (m: AIModel) => {
    const ok = await confirmMutation(() => deleteModel(m.id), {
      title: t('admin.ai_providers.title'),
      message: t('admin.ai_providers.confirm_delete_model', { name: m.display_name }),
      successMessage: t('admin.ai_providers.deleted'),
    })
    if (ok) {
      if (selectedProvider) {
        await reloadModels(selectedProvider.id)
      }
      await reloadTop()
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
      toast.error(errorMessage(e))
    }
  }

  const onProviderSaved = async () => {
    providerModal.closeModal()
    await reloadTop()
    if (selectedProvider) {
      const refreshed = providers.find((p) => p.id === selectedProvider.id)
      if (refreshed) {
        setSelectedProvider(refreshed)
      }
    }
  }

  const onModelSaved = async () => {
    modelModal.closeModal()
    if (selectedProvider) {
      await reloadModels(selectedProvider.id)
    }
    await reloadTop()
  }

  const activeByPurpose = (purpose: AIPurpose) => activeModels.find((m) => m.purpose === purpose)

  return (
    <AdminPanelShell
      title={t('admin.ai_providers.title')}
      description={t('admin.ai_providers.description')}
      readOnly={!canEdit}
      maxWidth="100%"
    >
      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}

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
      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h3 className="text-foreground m-0 text-base font-semibold">
            {t('admin.ai_providers.providers_title')}
          </h3>
          <button
            onClick={() => providerModal.openModal()}
            disabled={!canEdit}
            className={cn(adminBtnPrimaryClass, !canEdit && 'cursor-not-allowed opacity-50')}
          >
            + {t('admin.ai_providers.add_provider')}
          </button>
        </div>

        <LoadingOverlay loading={loading}>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
            {providers.map((p) => (
              <ProviderCard
                key={p.id}
                provider={p}
                selected={selectedProvider?.id === p.id}
                onSelect={() => setSelectedProvider((cur) => (cur?.id === p.id ? null : p))}
                onEdit={() => providerModal.openModal(p)}
                onDelete={() => {
                  void handleDeleteProvider(p)
                }}
                canEdit={canEdit}
              />
            ))}
            {!loading && providers.length === 0 && (
              <div className="text-foreground-muted p-4 text-sm">
                {t('admin.ai_providers.no_providers')}
              </div>
            )}
          </div>
        </LoadingOverlay>
      </section>

      {/* Models for selected provider */}
      {selectedProvider && (
        <section className="bg-card border-border overflow-hidden rounded-lg border shadow-sm">
          <div className="border-border flex items-center justify-between border-b p-[12px_16px] text-sm font-semibold">
            <span>{t('admin.ai_providers.models_for', { name: selectedProvider.name })}</span>
            <button
              onClick={() => modelModal.openModal()}
              disabled={!canEdit}
              className={cn(adminBtnSecondaryClass, !canEdit && 'cursor-not-allowed opacity-50')}
            >
              + {t('admin.ai_providers.add_model')}
            </button>
          </div>
          <LoadingOverlay loading={modelsLoading}>
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-left text-sm">
                <thead>
                  <tr className="border-border bg-card-raised border-b">
                    <th className="text-foreground p-[10px_16px] text-xs font-semibold tracking-wider uppercase">
                      {t('admin.ai_providers.fields.display_name')}
                    </th>
                    <th className="text-foreground p-[10px_16px] text-xs font-semibold tracking-wider uppercase">
                      {t('admin.ai_providers.fields.model_id')}
                    </th>
                    <th className="text-foreground p-[10px_16px] text-xs font-semibold tracking-wider uppercase">
                      {t('admin.ai_providers.fields.purpose')}
                    </th>
                    <th className="text-foreground p-[10px_16px] text-xs font-semibold tracking-wider uppercase">
                      {t('admin.ai_providers.fields.max_tokens')}
                    </th>
                    <th className="text-foreground p-[10px_16px] text-right text-xs font-semibold tracking-wider uppercase" />
                  </tr>
                </thead>
                <tbody>
                  {models.map((m) => (
                    <tr key={m.id} className="border-border border-b">
                      <td className="text-foreground p-[10px_16px]">
                        {m.display_name}
                        {m.is_default && (
                          <span className="text-success text-micro ml-2 inline-block rounded-full bg-emerald-500/12 px-1.5 py-0.5 font-semibold">
                            {t('admin.ai_providers.default_badge')}
                          </span>
                        )}
                        {!m.is_active && (
                          <span className="bg-foreground-muted/12 text-foreground-muted text-micro ml-2 inline-block rounded-full px-1.5 py-0.5 font-semibold">
                            {t('admin.ai_providers.inactive')}
                          </span>
                        )}
                      </td>
                      <td className="text-foreground p-[10px_16px] font-mono text-xs">
                        {m.model_id}
                      </td>
                      <td className="text-foreground p-[10px_16px]">
                        {t(`admin.ai_providers.purposes.${m.purpose}`)}
                      </td>
                      <td className="text-foreground p-[10px_16px]">{m.max_tokens}</td>
                      <td className="text-foreground p-[10px_16px] text-right whitespace-nowrap">
                        {!m.is_default && (
                          <button
                            disabled={!canEdit}
                            onClick={() => {
                              void handleSetDefault(m)
                            }}
                            className="text-accent cursor-pointer border-none bg-transparent px-2 py-1 text-xs font-semibold hover:underline disabled:opacity-50"
                          >
                            {t('admin.ai_providers.set_default')}
                          </button>
                        )}
                        <button
                          disabled={!canEdit}
                          onClick={() => modelModal.openModal(m)}
                          className="text-accent cursor-pointer border-none bg-transparent px-2 py-1 text-xs font-semibold hover:underline disabled:opacity-50"
                        >
                          {t('common.edit')}
                        </button>
                        <button
                          disabled={!canEdit}
                          onClick={() => {
                            void handleDeleteModel(m)
                          }}
                          className="text-error cursor-pointer border-none bg-transparent px-2 py-1 text-xs font-semibold hover:underline disabled:opacity-50"
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                  {!modelsLoading && models.length === 0 && (
                    <tr>
                      <td colSpan={5} className="text-foreground-muted p-5 text-center text-sm">
                        {t('admin.ai_providers.no_models')}
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </LoadingOverlay>
        </section>
      )}

      {providerModal.open && (
        <ProviderModal
          provider={providerModal.data}
          onClose={providerModal.closeModal}
          onSaved={() => {
            void onProviderSaved()
          }}
        />
      )}

      {modelModal.open && selectedProvider && (
        <ModelModal
          provider={selectedProvider}
          model={modelModal.data}
          onClose={modelModal.closeModal}
          onSaved={() => {
            void onModelSaved()
          }}
        />
      )}
    </AdminPanelShell>
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
      className={cn(
        'bg-card flex cursor-pointer flex-col gap-1.5 rounded-lg border p-3.5 transition-all duration-150',
        selected ? 'border-accent' : 'border-border',
      )}
      onClick={onSelect}
    >
      <div className="flex items-start justify-between">
        <strong className="text-foreground text-sm font-semibold">{provider.name}</strong>
        <span
          className={cn(
            'text-xs font-semibold',
            provider.is_active ? 'text-success' : 'text-foreground-muted',
          )}
        >
          {provider.is_active ? t('admin.ai_providers.active') : t('admin.ai_providers.inactive')}
        </span>
      </div>
      <div className="text-foreground-muted text-xs">
        {t(`admin.ai_providers.types.${provider.provider_type}`)}
      </div>
      <div className="text-foreground-muted font-mono text-xs break-all">
        {provider.base_url || '—'}
      </div>
      <div className="text-foreground-muted text-xs">
        {t('admin.ai_providers.model_count', { count: provider.model_count })}
        {provider.has_api_key && <span> · {provider.api_key_masked}</span>}
      </div>
      <div className="mt-1 flex gap-2" onClick={(e) => e.stopPropagation()}>
        <button
          disabled={!canEdit}
          onClick={onEdit}
          className="text-accent cursor-pointer border-none bg-transparent px-2 py-1 text-xs font-semibold hover:underline disabled:opacity-50"
        >
          {t('common.edit')}
        </button>
        <button
          disabled={!canEdit}
          onClick={onDelete}
          className="text-error cursor-pointer border-none bg-transparent px-2 py-1 text-xs font-semibold hover:underline disabled:opacity-50"
        >
          {t('common.delete')}
        </button>
      </div>
    </div>
  )
}
