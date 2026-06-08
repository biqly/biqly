import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  type AIModel,
  type AIProvider,
  type AIPurpose,
  createModel,
  listProviderRemoteModels,
  type RemoteModelOption,
  updateModel,
} from '../../api/aiProviders'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import {
  aiModalActions,
  aiModalCheckboxRow,
  aiModalFormStyle,
  aiModalInputStyle,
  aiModalLinkBtn,
  aiModalPrimaryBtn,
  aiModalSecondaryBtn,
  ModalField,
  PURPOSES,
} from './aiProviderModalShared'

function RemoteModelIdPicker({
  t,
  providerId,
  modelID,
  displayName,
  remoteModels,
  loadingRemote,
  remoteError,
  useManualModelID,
  onModelIDChange,
  onFetchRemote,
  onToggleManual,
}: {
  t: ReturnType<typeof useT>
  providerId: string
  modelID: string
  displayName: string
  remoteModels: RemoteModelOption[]
  loadingRemote: boolean
  remoteError: string | null
  useManualModelID: boolean
  onModelIDChange: (next: string) => void
  onFetchRemote: () => void
  onToggleManual: () => void
}) {
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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ display: 'flex', gap: 8, alignItems: 'stretch' }}>
        {remoteModels.length > 0 && !useManualModelID ? (
          <div style={{ flex: 1, minWidth: 0 }}>
            <Select
              value={modelID}
              onChange={onModelIDChange}
              options={remoteModelOptions}
              placeholder={t('admin.ai_providers.fields.model_id')}
            />
          </div>
        ) : (
          <input
            style={{ ...aiModalInputStyle, flex: 1 }}
            value={modelID}
            onChange={(e) => onModelIDChange(e.target.value)}
            autoFocus
            placeholder="gpt-4o"
            list={remoteModels.length > 0 ? `remote-models-${providerId}` : undefined}
          />
        )}
        <button
          type="button"
          style={{ ...aiModalSecondaryBtn, flexShrink: 0, alignSelf: 'stretch' }}
          disabled={loadingRemote}
          onClick={onFetchRemote}
        >
          {loadingRemote
            ? t('admin.ai_providers.fields.fetching_remote_models')
            : t('admin.ai_providers.fields.fetch_remote_models')}
        </button>
      </div>
      {remoteModels.length > 0 && !remoteError && (
        <small className="form-hint" style={{ margin: 0 }}>
          {t('admin.ai_providers.fields.remote_models_count', { count: remoteModels.length })}
          {' · '}
          <button
            type="button"
            style={{ ...aiModalLinkBtn, padding: 0, fontSize: 'inherit' }}
            onClick={onToggleManual}
          >
            {useManualModelID
              ? t('admin.ai_providers.fields.pick_from_list')
              : t('admin.ai_providers.fields.enter_model_manually')}
          </button>
        </small>
      )}
      {remoteModels.length > 0 && useManualModelID && (
        <datalist id={`remote-models-${providerId}`}>
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
  )
}

export function ModelModal({
  provider,
  model,
  onClose,
  onSaved,
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
      setRemoteModels(rows)
      if (rows.length === 0) {
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

  const pickModelID = (next: string) => {
    setModelID(next)
    if (!displayName.trim() && next.trim()) {
      setDisplayName(next.trim())
    }
  }

  const save = async () => {
    if (!modelID.trim()) {
      toast.error(t('admin.ai_providers.fields.model_id'))
      return
    }
    setSaving(true)
    try {
      if (editing) {
        await updateModel(model.id, {
          model_id: modelID,
          display_name: displayName || modelID,
          purpose,
          max_tokens: maxTokens,
          temperature,
          max_prompt_input_runes: maxPromptRunes,
        })
      } else {
        await createModel({
          provider_id: provider.id,
          model_id: modelID,
          display_name: displayName || modelID,
          purpose,
          max_tokens: maxTokens,
          temperature,
          max_prompt_input_runes: maxPromptRunes,
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
    <Modal
      open
      title={editing ? t('admin.ai_providers.edit_model') : t('admin.ai_providers.add_model')}
      onClose={onClose}
    >
      <div style={aiModalFormStyle}>
        <ModalField label={t('admin.ai_providers.fields.model_id')}>
          <RemoteModelIdPicker
            t={t}
            providerId={provider.id}
            modelID={modelID}
            displayName={displayName}
            remoteModels={remoteModels}
            loadingRemote={loadingRemote}
            remoteError={remoteError}
            useManualModelID={useManualModelID}
            onModelIDChange={pickModelID}
            onFetchRemote={() => void fetchRemoteModels()}
            onToggleManual={() => setUseManualModelID((v) => !v)}
          />
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.display_name')}>
          <input
            style={aiModalInputStyle}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.purpose')}>
          <Select<AIPurpose>
            value={purpose}
            onChange={setPurpose}
            options={PURPOSES.map((p) => ({
              value: p,
              label: t(`admin.ai_providers.purposes.${p}`),
            }))}
          />
        </ModalField>
        <div style={{ display: 'flex', gap: 12 }}>
          <ModalField label={t('admin.ai_providers.fields.max_tokens')} style={{ flex: 1 }}>
            <input
              style={aiModalInputStyle}
              type="number"
              value={maxTokens}
              onChange={(e) => setMaxTokens(Number(e.target.value))}
            />
          </ModalField>
          <ModalField label={t('admin.ai_providers.fields.temperature')} style={{ flex: 1 }}>
            <input
              style={aiModalInputStyle}
              type="number"
              step="0.1"
              value={temperature}
              onChange={(e) => setTemperature(Number(e.target.value))}
            />
          </ModalField>
        </div>
        <ModalField label={t('admin.ai_providers.fields.max_prompt_runes')}>
          <input
            style={aiModalInputStyle}
            type="number"
            value={maxPromptRunes}
            onChange={(e) => setMaxPromptRunes(Number(e.target.value))}
          />
        </ModalField>
        {!editing && (
          <label style={aiModalCheckboxRow}>
            <input
              type="checkbox"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
            />
            {t('admin.ai_providers.fields.set_default')}
          </label>
        )}
        <div style={aiModalActions}>
          <button style={aiModalSecondaryBtn} onClick={onClose}>
            {t('common.cancel')}
          </button>
          <button style={aiModalPrimaryBtn} disabled={saving} onClick={() => void save()}>
            {t('common.save')}
          </button>
        </div>
      </div>
    </Modal>
  )
}
