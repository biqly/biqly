import { type CSSProperties, useCallback, useEffect, useId, useState } from 'react'

import { getPlatformSettings, updatePlatformSettings } from '../../api/admin'
import type { AIAdminRuntimeConfig, RuntimeConfigSource } from '../../api/aiAdmin'
import { clearPasswordPolicyCache } from '../../api/auth'
import {
  buildRuntimeConfigUpdate,
  draftFromConfig,
  type RuntimeConfigDraft,
  useRuntimeConfig,
} from '../../hooks/useRuntimeConfig'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import { ReadOnlyNote } from './ReadOnlyNote'

const toggleCardStyle: CSSProperties = {
  display: 'flex',
  gap: 12,
  alignItems: 'flex-start',
  padding: 16,
  borderRadius: 8,
  border: '1px solid var(--border, rgba(0, 0, 0, 0.12))',
  background: 'var(--bg-card-raised, rgba(0, 0, 0, 0.02))',
  cursor: 'pointer',
}

// SourceBadge marks where a knob's effective value comes from: the
// ai_runtime_config DB row ("DB") or environment defaults ("Env").
function SourceBadge({ source }: { source?: RuntimeConfigSource }) {
  const t = useT()
  if (!source) {
    return null
  }
  const fromDB = source === 'database'
  return (
    <span
      className={fromDB ? 'admin-badge-active' : 'admin-badge-pending'}
      aria-label={
        fromDB
          ? t('admin.platform_settings.source_db_aria')
          : t('admin.platform_settings.source_env_aria')
      }
      style={{ marginLeft: 8, verticalAlign: 'middle' }}
    >
      {fromDB ? t('admin.platform_settings.source_db') : t('admin.platform_settings.source_env')}
    </span>
  )
}

function ToggleField({
  label,
  hint,
  checked,
  disabled,
  source,
  onChange,
}: {
  label: string
  hint: string
  checked: boolean
  disabled: boolean
  source?: RuntimeConfigSource
  onChange: (checked: boolean) => void
}) {
  return (
    <label style={toggleCardStyle}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        disabled={disabled}
        style={{ marginTop: 3 }}
      />
      <span>
        <strong style={{ display: 'block', marginBottom: 4 }}>
          {label}
          <SourceBadge source={source} />
        </strong>
        <span className="form-hint" style={{ margin: 0 }}>
          {hint}
        </span>
      </span>
    </label>
  )
}

function NumberField({
  label,
  hint,
  value,
  min,
  max,
  step = 1,
  disabled,
  source,
  onChange,
}: {
  label: string
  hint: string
  value: number
  min: number
  max: number
  step?: number
  disabled: boolean
  source?: RuntimeConfigSource
  onChange: (value: number) => void
}) {
  const inputId = useId()
  return (
    <label className="admin-form-label" style={{ gap: 4, maxWidth: 360 }} htmlFor={inputId}>
      <span className="admin-label-text">
        {label}
        <SourceBadge source={source} />
      </span>
      <input
        id={inputId}
        type="number"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        disabled={disabled}
      />
      <span className="form-hint" style={{ margin: 0 }}>
        {hint}
      </span>
    </label>
  )
}

function SectionHeader({ title, description }: { title: string; description: string }) {
  return (
    <div style={{ marginTop: 24 }}>
      <h2 style={{ margin: 0 }}>{title}</h2>
      <p className="form-hint" style={{ marginTop: 8 }}>
        {description}
      </p>
    </div>
  )
}

// AIRuntimeForm renders the editable ambiguity / PII / memory knobs once the
// runtime config and its editable draft are loaded (props are non-null here).
function AIRuntimeForm({
  config,
  draft,
  setDraft,
  disabled,
  saving,
  onSave,
}: {
  config: AIAdminRuntimeConfig
  draft: RuntimeConfigDraft
  setDraft: (updater: (d: RuntimeConfigDraft | null) => RuntimeConfigDraft | null) => void
  disabled: boolean
  saving: boolean
  onSave: () => void
}) {
  const t = useT()
  const setAmbiguity = (patch: Partial<RuntimeConfigDraft['ambiguity']>) =>
    setDraft((d) => (d ? { ...d, ambiguity: { ...d.ambiguity, ...patch } } : d))
  const setPII = (patch: Partial<RuntimeConfigDraft['pii']>) =>
    setDraft((d) => (d ? { ...d, pii: { ...d.pii, ...patch } } : d))
  const setMemory = (patch: Partial<RuntimeConfigDraft['memory']>) =>
    setDraft((d) => (d ? { ...d, memory: { ...d.memory, ...patch } } : d))

  const ambiguitySources = config.ambiguity.sources ?? {}
  const piiSources = config.pii.sources ?? {}
  const memorySources = config.memory.sources ?? {}
  const anyDBOverride =
    config.ambiguity.db_override || config.pii.db_override || config.memory.db_override

  return (
    <>
      <ToggleField
        label={t('admin.platform_settings.ambiguity_check_label')}
        hint={t('admin.platform_settings.ambiguity_check_hint')}
        checked={draft.ambiguity.check_enabled}
        disabled={disabled}
        source={ambiguitySources.check_enabled}
        onChange={(v) => setAmbiguity({ check_enabled: v })}
      />

      <ToggleField
        label={t('admin.platform_settings.ambiguity_tiered_label')}
        hint={t('admin.platform_settings.ambiguity_tiered_hint')}
        checked={draft.ambiguity.tiered_enabled}
        disabled={disabled}
        source={ambiguitySources.tiered_enabled}
        onChange={(v) => setAmbiguity({ tiered_enabled: v })}
      />

      <NumberField
        label={t('admin.platform_settings.ambiguity_max_llm_label')}
        hint={t('admin.platform_settings.ambiguity_max_llm_hint')}
        value={draft.ambiguity.max_llm_tier_per_question}
        min={0}
        max={10}
        disabled={disabled}
        source={ambiguitySources.max_llm_tier_per_question}
        onChange={(v) => setAmbiguity({ max_llm_tier_per_question: v })}
      />

      <NumberField
        label={t('admin.platform_settings.ambiguity_confidence_label')}
        hint={t('admin.platform_settings.ambiguity_confidence_hint')}
        value={draft.ambiguity.confidence_threshold}
        min={0}
        max={1}
        step={0.05}
        disabled={disabled}
        source={ambiguitySources.confidence_threshold}
        onChange={(v) => setAmbiguity({ confidence_threshold: v })}
      />

      <NumberField
        label={t('admin.platform_settings.ambiguity_max_options_label')}
        hint={t('admin.platform_settings.ambiguity_max_options_hint')}
        value={draft.ambiguity.max_options}
        min={1}
        max={10}
        disabled={disabled}
        source={ambiguitySources.max_options}
        onChange={(v) => setAmbiguity({ max_options: v })}
      />

      <SectionHeader
        title={t('admin.platform_settings.pii_title')}
        description={t('admin.platform_settings.pii_description')}
      />

      <p className="form-hint" style={{ margin: 0 }}>
        {config.pii.enabled
          ? t('admin.platform_settings.pii_enabled_on')
          : t('admin.platform_settings.pii_enabled_off')}{' '}
        {t('admin.platform_settings.pii_enabled_env_note')}
      </p>

      <NumberField
        label={t('admin.platform_settings.pii_threshold_label')}
        hint={t('admin.platform_settings.pii_threshold_hint')}
        value={draft.pii.detection_threshold}
        min={0.05}
        max={1}
        step={0.05}
        disabled={disabled}
        source={piiSources.detection_threshold}
        onChange={(v) => setPII({ detection_threshold: v })}
      />

      <SectionHeader
        title={t('admin.platform_settings.memory_title')}
        description={t('admin.platform_settings.memory_description')}
      />

      <ToggleField
        label={t('admin.platform_settings.memory_recall_label')}
        hint={t('admin.platform_settings.memory_recall_hint')}
        checked={draft.memory.recall_enabled}
        disabled={disabled}
        source={memorySources.recall_enabled}
        onChange={(v) => setMemory({ recall_enabled: v })}
      />

      <NumberField
        label={t('admin.platform_settings.memory_limit_label')}
        hint={t('admin.platform_settings.memory_limit_hint')}
        value={draft.memory.recall_limit}
        min={1}
        max={10}
        disabled={disabled}
        source={memorySources.recall_limit}
        onChange={(v) => setMemory({ recall_limit: v })}
      />

      <p className="form-hint" style={{ margin: 0 }}>
        {anyDBOverride
          ? t('admin.platform_settings.ambiguity_db_override_note')
          : t('admin.platform_settings.ambiguity_env_default_note')}
      </p>

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className="btn btn-primary"
          disabled={saving || disabled}
          onClick={onSave}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </>
  )
}

// AIRuntimeSection owns the runtime-config lifecycle (load → draft → save) so
// the parent panel stays simple.
function AIRuntimeSection({ canEdit }: { canEdit: boolean }) {
  const t = useT()
  const toast = useToast()
  const { config, error, saving, save } = useRuntimeConfig()
  const [draft, setDraft] = useState<RuntimeConfigDraft | null>(null)

  useEffect(() => {
    if (config) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDraft(draftFromConfig(config))
    }
  }, [config])

  const handleSave = async () => {
    if (!draft) {
      return
    }
    try {
      await save(buildRuntimeConfigUpdate(draft))
      toast.success(t('admin.platform_settings.runtime_saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <>
      <SectionHeader
        title={t('admin.platform_settings.ambiguity_title')}
        description={t('admin.platform_settings.ambiguity_description')}
      />
      {error && (
        <p
          className="form-hint"
          style={{ margin: 0, color: 'var(--danger, #c0392b)' }}
          role="alert"
        >
          {error}
        </p>
      )}
      {config && draft && (
        <AIRuntimeForm
          config={config}
          draft={draft}
          setDraft={setDraft}
          disabled={!canEdit}
          saving={saving}
          onSave={() => void handleSave()}
        />
      )}
    </>
  )
}

export function PlatformSettingsPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { isSuperAdmin, hasPermission } = useAuth()
  // Platform (auth) settings are super-admin only; the AI runtime section is
  // also editable with the ai:settings RBAC permission (enforced server-side).
  const canEdit = isSuperAdmin
  const canEditRuntime = hasPermission('ai:settings')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [selfSignupEnabled, setSelfSignupEnabled] = useState(true)
  const [updatedAt, setUpdatedAt] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await getPlatformSettings(token)
      setSelfSignupEnabled(data.self_signup_enabled)
      setUpdatedAt(data.updated_at ?? null)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [token, toast])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const save = async () => {
    setSaving(true)
    try {
      const data = await updatePlatformSettings(token, selfSignupEnabled)
      setSelfSignupEnabled(data.self_signup_enabled)
      setUpdatedAt(data.updated_at ?? null)
      clearPasswordPolicyCache()
      toast.success(t('admin.platform_settings.saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <LoadingScreen minHeight="200px" />
  }

  return (
    <div className="page-stack" style={{ maxWidth: 640 }}>
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.platform_settings.title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.platform_settings.description')}{' '}
          <a
            href="https://github.com/biqly/biqly/blob/main/docs/configuration.md"
            target="_blank"
            rel="noopener noreferrer"
            style={{ color: 'var(--accent, #0066cc)', textDecoration: 'underline' }}
          >
            {t('admin.platform_settings.all_keys_reference')}
          </a>
        </p>
      </div>

      {!canEdit && <ReadOnlyNote />}

      <label style={toggleCardStyle}>
        <input
          type="checkbox"
          checked={selfSignupEnabled}
          onChange={(e) => setSelfSignupEnabled(e.target.checked)}
          disabled={!canEdit}
          style={{ marginTop: 3 }}
        />
        <span>
          <strong style={{ display: 'block', marginBottom: 4 }}>
            {t('admin.platform_settings.self_signup_label')}
          </strong>
          <span className="form-hint" style={{ margin: 0 }}>
            {selfSignupEnabled
              ? t('admin.platform_settings.self_signup_on_hint')
              : t('admin.platform_settings.self_signup_off_hint')}
          </span>
        </span>
      </label>

      {updatedAt && (
        <p className="form-hint" style={{ margin: 0 }}>
          {t('admin.platform_settings.last_updated', {
            date: new Date(updatedAt).toLocaleString(),
          })}
        </p>
      )}

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className="btn btn-primary"
          disabled={saving || !canEdit}
          onClick={() => void save()}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>

      <AIRuntimeSection canEdit={canEditRuntime} />
    </div>
  )
}
