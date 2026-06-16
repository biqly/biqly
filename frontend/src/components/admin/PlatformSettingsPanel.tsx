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
import { type TFunction, useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formHintClass } from '../../lib/formClasses'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import {
  adminActiveBadgeClass,
  adminBadgePendingClass,
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminCardClass,
  adminFormLabelClass,
  adminInputClass,
  adminLabelTextClass,
  adminRangeSliderClass,
} from './adminClasses'
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
      className={fromDB ? adminActiveBadgeClass(true) : adminBadgePendingClass}
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
        <span className={cn(formHintClass, 'm-0')}>{hint}</span>
      </span>
    </label>
  )
}

function StepperField({
  label,
  hint,
  value,
  min,
  max,
  disabled,
  source,
  onChange,
}: {
  label: string
  hint: string
  value: number
  min: number
  max: number
  disabled: boolean
  source?: RuntimeConfigSource
  onChange: (value: number) => void
}) {
  const inputId = useId()
  const decrement = () => {
    if (value > min) {
      onChange(value - 1)
    }
  }
  const increment = () => {
    if (value < max) {
      onChange(value + 1)
    }
  }

  return (
    <label className={adminFormLabelClass} style={{ gap: 4, maxWidth: 360 }} htmlFor={inputId}>
      <span className={adminLabelTextClass}>
        {label}
        <SourceBadge source={source} />
      </span>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.25rem', marginTop: 4 }}>
        <button
          type="button"
          className={adminBtnSecondaryClass}
          onClick={decrement}
          disabled={disabled || value <= min}
          style={{
            padding: '4px 12px',
            height: '34px',
            minHeight: 'auto',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '1.1rem',
            fontWeight: 700,
            margin: 0,
          }}
        >
          -
        </button>
        <input
          id={inputId}
          type="number"
          min={min}
          max={max}
          value={value}
          onChange={(e) => {
            const val = Number(e.target.value)
            if (!isNaN(val) && val >= min && val <= max) {
              onChange(val)
            }
          }}
          disabled={disabled}
          className={adminInputClass}
          style={{
            textAlign: 'center',
            height: '34px',
            maxWidth: '4.5rem',
            margin: 0,
          }}
        />
        <button
          type="button"
          className={adminBtnSecondaryClass}
          onClick={increment}
          disabled={disabled || value >= max}
          style={{
            padding: '4px 12px',
            height: '34px',
            minHeight: 'auto',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '1.1rem',
            fontWeight: 700,
            margin: 0,
          }}
        >
          +
        </button>
      </div>
      <span className={cn(formHintClass, 'm-0')}>{hint}</span>
    </label>
  )
}

function PercentageField({
  label,
  hint,
  value,
  disabled,
  source,
  onChange,
}: {
  label: string
  hint: string
  value: number
  disabled: boolean
  source?: RuntimeConfigSource
  onChange: (value: number) => void
}) {
  const inputId = useId()
  const pctValue = Math.round(value * 100)

  return (
    <label className={adminFormLabelClass} style={{ gap: 4, maxWidth: 360 }} htmlFor={inputId}>
      <span className={adminLabelTextClass}>
        {label}
        <SourceBadge source={source} />
      </span>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginTop: 4 }}>
        <input
          id={inputId}
          type="range"
          min={0}
          max={100}
          step={5}
          value={pctValue}
          onChange={(e) => onChange(Number(e.target.value) / 100)}
          disabled={disabled}
          className={adminRangeSliderClass}
          style={{ flex: 1, cursor: 'pointer' }}
        />
        <div
          style={{
            minWidth: '3.5rem',
            textAlign: 'center',
            padding: '4px 8px',
            background: 'var(--bg-card-raised)',
            border: '1px solid var(--border)',
            borderRadius: '6px',
            fontSize: '0.82rem',
            fontWeight: 700,
            color: 'var(--text-primary)',
          }}
        >
          {pctValue}%
        </div>
      </div>
      <span className={cn(formHintClass, 'm-0')}>{hint}</span>
    </label>
  )
}

interface GeneralSettingsCardProps {
  selfSignupEnabled: boolean
  setSelfSignupEnabled: (v: boolean) => void
  canEdit: boolean
  saving: boolean
  updatedAt: string | null
  onSave: () => void
  t: TFunction
}

function GeneralSettingsCard({
  selfSignupEnabled,
  setSelfSignupEnabled,
  canEdit,
  saving,
  updatedAt,
  onSave,
  t,
}: GeneralSettingsCardProps) {
  return (
    <div
      className={adminCardClass}
      style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}
    >
      <h3 style={{ margin: 0 }}>{t('admin.platform_settings.title')}</h3>

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
          <span className={cn(formHintClass, 'm-0')}>
            {selfSignupEnabled
              ? t('admin.platform_settings.self_signup_on_hint')
              : t('admin.platform_settings.self_signup_off_hint')}
          </span>
        </span>
      </label>

      {updatedAt && (
        <p className={cn(formHintClass, 'm-0')}>
          {t('admin.platform_settings.last_updated', {
            date: new Date(updatedAt).toLocaleString(),
          })}
        </p>
      )}

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          className={adminBtnPrimaryClass}
          disabled={saving || !canEdit}
          onClick={onSave}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </div>
  )
}

interface AmbiguitySettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setAmbiguity: (patch: Partial<RuntimeConfigDraft['ambiguity']>) => void
  t: TFunction
}

function AmbiguitySettingsCard({
  draft,
  canEditRuntime,
  sources,
  setAmbiguity,
  t,
}: AmbiguitySettingsCardProps) {
  const ambiguitySources = sources ?? {}
  return (
    <div
      className={adminCardClass}
      style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}
    >
      <h3 style={{ margin: 0 }}>{t('admin.platform_settings.ambiguity_title')}</h3>

      <ToggleField
        label={t('admin.platform_settings.ambiguity_check_label')}
        hint={t('admin.platform_settings.ambiguity_check_hint')}
        checked={draft.ambiguity.check_enabled}
        disabled={!canEditRuntime}
        source={ambiguitySources.check_enabled}
        onChange={(v) => setAmbiguity({ check_enabled: v })}
      />

      <ToggleField
        label={t('admin.platform_settings.ambiguity_tiered_label')}
        hint={t('admin.platform_settings.ambiguity_tiered_hint')}
        checked={draft.ambiguity.tiered_enabled}
        disabled={!canEditRuntime}
        source={ambiguitySources.tiered_enabled}
        onChange={(v) => setAmbiguity({ tiered_enabled: v })}
      />

      <StepperField
        label={t('admin.platform_settings.ambiguity_max_llm_label')}
        hint={t('admin.platform_settings.ambiguity_max_llm_hint')}
        value={draft.ambiguity.max_llm_tier_per_question}
        min={0}
        max={10}
        disabled={!canEditRuntime}
        source={ambiguitySources.max_llm_tier_per_question}
        onChange={(v) => setAmbiguity({ max_llm_tier_per_question: v })}
      />

      <PercentageField
        label={t('admin.platform_settings.ambiguity_confidence_label')}
        hint={t('admin.platform_settings.ambiguity_confidence_hint')}
        value={draft.ambiguity.confidence_threshold}
        disabled={!canEditRuntime}
        source={ambiguitySources.confidence_threshold}
        onChange={(v) => setAmbiguity({ confidence_threshold: v })}
      />

      <StepperField
        label={t('admin.platform_settings.ambiguity_max_options_label')}
        hint={t('admin.platform_settings.ambiguity_max_options_hint')}
        value={draft.ambiguity.max_options}
        min={1}
        max={10}
        disabled={!canEditRuntime}
        source={ambiguitySources.max_options}
        onChange={(v) => setAmbiguity({ max_options: v })}
      />
    </div>
  )
}

interface PIISettingsCardProps {
  config: AIAdminRuntimeConfig
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setPII: (patch: Partial<RuntimeConfigDraft['pii']>) => void
  t: TFunction
}

function PIISettingsCard({
  config,
  draft,
  canEditRuntime,
  sources,
  setPII,
  t,
}: PIISettingsCardProps) {
  const piiSources = sources ?? {}
  return (
    <div
      className={adminCardClass}
      style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}
    >
      <h3 style={{ margin: 0 }}>{t('admin.platform_settings.pii_title')}</h3>
      <p className={cn(formHintClass, 'm-0')}>
        {config.pii.enabled
          ? t('admin.platform_settings.pii_enabled_on')
          : t('admin.platform_settings.pii_enabled_off')}{' '}
        {t('admin.platform_settings.pii_enabled_env_note')}
      </p>
      <PercentageField
        label={t('admin.platform_settings.pii_threshold_label')}
        hint={t('admin.platform_settings.pii_threshold_hint')}
        value={draft.pii.detection_threshold}
        disabled={!canEditRuntime}
        source={piiSources.detection_threshold}
        onChange={(v) => setPII({ detection_threshold: v })}
      />
    </div>
  )
}

interface MemorySettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setMemory: (patch: Partial<RuntimeConfigDraft['memory']>) => void
  t: TFunction
}

function MemorySettingsCard({
  draft,
  canEditRuntime,
  sources,
  setMemory,
  t,
}: MemorySettingsCardProps) {
  const memorySources = sources ?? {}
  return (
    <div
      className={adminCardClass}
      style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}
    >
      <h3 style={{ margin: 0 }}>{t('admin.platform_settings.memory_title')}</h3>

      <ToggleField
        label={t('admin.platform_settings.memory_recall_label')}
        hint={t('admin.platform_settings.memory_recall_hint')}
        checked={draft.memory.recall_enabled}
        disabled={!canEditRuntime}
        source={memorySources.recall_enabled}
        onChange={(v) => setMemory({ recall_enabled: v })}
      />

      <StepperField
        label={t('admin.platform_settings.memory_limit_label')}
        hint={t('admin.platform_settings.memory_limit_hint')}
        value={draft.memory.recall_limit}
        min={1}
        max={10}
        disabled={!canEditRuntime}
        source={memorySources.recall_limit}
        onChange={(v) => setMemory({ recall_limit: v })}
      />
    </div>
  )
}

interface QueueSettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setQueue: (patch: Partial<RuntimeConfigDraft['queue']>) => void
  t: TFunction
}

function QueueSettingsCard({
  draft,
  canEditRuntime,
  sources,
  setQueue,
  t,
}: QueueSettingsCardProps) {
  const queueSources = sources ?? {}
  return (
    <div
      className={adminCardClass}
      style={{ display: 'flex', flexDirection: 'column', gap: '1.25rem' }}
    >
      <h3 style={{ margin: 0 }}>{t('admin.platform_settings.queue_title')}</h3>

      <StepperField
        label={t('admin.platform_settings.queue_concurrency_label')}
        hint={t('admin.platform_settings.queue_concurrency_hint')}
        value={draft.queue.concurrency}
        min={1}
        max={10}
        disabled={!canEditRuntime}
        source={queueSources.concurrency}
        onChange={(v) => setQueue({ concurrency: v })}
      />
    </div>
  )
}

interface RuntimeSettingsSectionProps {
  config: AIAdminRuntimeConfig
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  savingRuntime: boolean
  onSaveRuntime: () => void
  setPII: (patch: Partial<RuntimeConfigDraft['pii']>) => void
  setMemory: (patch: Partial<RuntimeConfigDraft['memory']>) => void
  setQueue: (patch: Partial<RuntimeConfigDraft['queue']>) => void
  t: TFunction
}

function RuntimeSettingsSection({
  config,
  draft,
  canEditRuntime,
  savingRuntime,
  onSaveRuntime,
  setPII,
  setMemory,
  setQueue,
  t,
}: RuntimeSettingsSectionProps) {
  const piiSources = config.pii.sources ?? {}
  const memorySources = config.memory.sources ?? {}
  const queueSources = config.queue.sources ?? {}
  const anyDBOverride =
    config.ambiguity.db_override ||
    config.pii.db_override ||
    config.memory.db_override ||
    config.queue.db_override

  return (
    <>
      <PIISettingsCard
        config={config}
        draft={draft}
        canEditRuntime={canEditRuntime}
        sources={piiSources}
        setPII={setPII}
        t={t}
      />
      <MemorySettingsCard
        draft={draft}
        canEditRuntime={canEditRuntime}
        sources={memorySources}
        setMemory={setMemory}
        t={t}
      />
      <QueueSettingsCard
        draft={draft}
        canEditRuntime={canEditRuntime}
        sources={queueSources}
        setQueue={setQueue}
        t={t}
      />
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
        <p className={cn(formHintClass, 'm-0')}>
          {anyDBOverride
            ? t('admin.platform_settings.ambiguity_db_override_note')
            : t('admin.platform_settings.ambiguity_env_default_note')}
        </p>
        <div style={{ display: 'flex', gap: 8 }}>
          <button
            type="button"
            className={adminBtnPrimaryClass}
            disabled={savingRuntime || !canEditRuntime}
            onClick={onSaveRuntime}
          >
            {savingRuntime ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </div>
    </>
  )
}

export function PlatformSettingsPanel({ token }: { token: string }) {
  const t = useT()
  const toast = useToast()
  const { isSuperAdmin, hasPermission } = useAuth()

  const canEdit = isSuperAdmin
  const canEditRuntime = hasPermission('ai:settings')

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [selfSignupEnabled, setSelfSignupEnabled] = useState(true)
  const [updatedAt, setUpdatedAt] = useState<string | null>(null)

  const {
    config,
    error: runtimeError,
    saving: savingRuntime,
    save: saveRuntime,
  } = useRuntimeConfig()
  const [draft, setDraft] = useState<RuntimeConfigDraft | null>(null)

  useEffect(() => {
    if (config) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDraft(draftFromConfig(config))
    }
  }, [config])

  const setAmbiguity = (patch: Partial<RuntimeConfigDraft['ambiguity']>) =>
    setDraft((d) => (d ? { ...d, ambiguity: { ...d.ambiguity, ...patch } } : d))
  const setPII = (patch: Partial<RuntimeConfigDraft['pii']>) =>
    setDraft((d) => (d ? { ...d, pii: { ...d.pii, ...patch } } : d))
  const setMemory = (patch: Partial<RuntimeConfigDraft['memory']>) =>
    setDraft((d) => (d ? { ...d, memory: { ...d.memory, ...patch } } : d))
  const setQueue = (patch: Partial<RuntimeConfigDraft['queue']>) =>
    setDraft((d) => (d ? { ...d, queue: { ...d.queue, ...patch } } : d))

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

  const handleSaveRuntime = async () => {
    if (!draft) {
      return
    }
    try {
      await saveRuntime(buildRuntimeConfigUpdate(draft))
      toast.success(t('admin.platform_settings.runtime_saved'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  if (loading) {
    return <LoadingScreen minHeight="200px" />
  }

  const ambiguitySources = config?.ambiguity.sources ?? {}

  return (
    <div className={legacyLayoutClass('page-stack')} style={{ maxWidth: 1000, width: '100%' }}>
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.platform_settings.title')}</h2>
        <p className={formHintClass}>
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

      {runtimeError && (
        <p className={cn(formHintClass, 'm-0 text-(--danger,#c0392b)')} role="alert">
          {runtimeError}
        </p>
      )}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))',
          gap: '1.5rem',
          alignItems: 'start',
          width: '100%',
          marginTop: '1.5rem',
        }}
      >
        {/* LEFT COLUMN */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          <GeneralSettingsCard
            selfSignupEnabled={selfSignupEnabled}
            setSelfSignupEnabled={setSelfSignupEnabled}
            canEdit={canEdit}
            saving={saving}
            updatedAt={updatedAt}
            onSave={() => void save()}
            t={t}
          />
          {config && draft && (
            <AmbiguitySettingsCard
              draft={draft}
              canEditRuntime={canEditRuntime}
              sources={ambiguitySources}
              setAmbiguity={setAmbiguity}
              t={t}
            />
          )}
        </div>

        {/* RIGHT COLUMN */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          {config && draft && (
            <RuntimeSettingsSection
              config={config}
              draft={draft}
              canEditRuntime={canEditRuntime}
              savingRuntime={savingRuntime}
              onSaveRuntime={() => void handleSaveRuntime()}
              setPII={setPII}
              setMemory={setMemory}
              setQueue={setQueue}
              t={t}
            />
          )}
        </div>
      </div>
    </div>
  )
}
