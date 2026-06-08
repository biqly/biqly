import { useState } from 'react'

import {
  type AIProvider,
  type AIProviderType,
  type ConnectionTestResult,
  createProvider,
  testProvider,
  updateProvider,
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
  aiModalPrimaryBtn,
  aiModalSecondaryBtn,
  defaultBaseURL,
  ModalField,
  PROVIDER_TYPES,
} from './aiProviderModalShared'

function ProviderConnectionTest({
  t,
  testing,
  testResult,
  onTest,
}: {
  t: ReturnType<typeof useT>
  testing: boolean
  testResult: ConnectionTestResult | null
  onTest: () => void
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      <button type="button" style={aiModalSecondaryBtn} disabled={testing} onClick={onTest}>
        {testing ? t('admin.ai_providers.testing') : t('admin.ai_providers.test_connection')}
      </button>
      {testResult && (
        <span
          style={{
            fontSize: 13,
            color:
              testResult.status === 'connected'
                ? 'var(--success, #10b981)'
                : 'var(--error, #ef4444)',
          }}
        >
          {testResult.status === 'connected'
            ? t('admin.ai_providers.test_connected', { ms: testResult.latency_ms ?? 0 })
            : `${t('admin.ai_providers.test_failed')}: ${testResult.message ?? ''}`}
        </span>
      )}
    </div>
  )
}

export function ProviderModal({
  provider,
  onClose,
  onSaved,
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
    if (!baseURL || baseURL === defaultBaseURL(type)) {
      setBaseURL(defaultBaseURL(next))
    }
  }

  const save = async () => {
    if (!name.trim()) {
      toast.error(t('admin.ai_providers.fields.name'))
      return
    }
    setSaving(true)
    try {
      if (editing) {
        await updateProvider(provider.id, {
          name,
          provider_type: type,
          base_url: baseURL,
          api_key: apiKey ? apiKey : null,
          is_active: isActive,
          http_timeout_seconds: timeout,
        })
      } else {
        await createProvider({
          name,
          provider_type: type,
          base_url: baseURL,
          api_key: apiKey || undefined,
          is_active: isActive,
          http_timeout_seconds: timeout,
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
    <Modal
      open
      title={editing ? t('admin.ai_providers.edit_provider') : t('admin.ai_providers.add_provider')}
      onClose={onClose}
    >
      <div style={aiModalFormStyle}>
        <ModalField label={t('admin.ai_providers.fields.name')}>
          <input
            style={aiModalInputStyle}
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.type')}>
          <Select<AIProviderType>
            value={type}
            onChange={onTypeChange}
            options={PROVIDER_TYPES.map((pt) => ({
              value: pt,
              label: t(`admin.ai_providers.types.${pt}`),
            }))}
          />
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.base_url')}>
          <input
            style={aiModalInputStyle}
            value={baseURL}
            onChange={(e) => setBaseURL(e.target.value)}
            placeholder="https://…/v1"
          />
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.api_key')}>
          <div style={{ display: 'flex', gap: 8 }}>
            <input
              style={{ ...aiModalInputStyle, flex: 1 }}
              type={showKey ? 'text' : 'password'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={editing ? t('admin.ai_providers.fields.api_key_keep') : ''}
            />
            <button type="button" style={aiModalSecondaryBtn} onClick={() => setShowKey((s) => !s)}>
              {showKey ? t('admin.ai_providers.hide_key') : t('admin.ai_providers.show_key')}
            </button>
          </div>
        </ModalField>
        <ModalField label={t('admin.ai_providers.fields.http_timeout')}>
          <input
            style={aiModalInputStyle}
            type="number"
            value={timeout}
            onChange={(e) => setTimeoutVal(Number(e.target.value))}
          />
        </ModalField>
        <label style={aiModalCheckboxRow}>
          <input
            type="checkbox"
            checked={isActive}
            onChange={(e) => setIsActive(e.target.checked)}
          />
          {t('admin.ai_providers.fields.is_active')}
        </label>

        {editing && (
          <ProviderConnectionTest
            t={t}
            testing={testing}
            testResult={testResult}
            onTest={() => void runTest()}
          />
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
