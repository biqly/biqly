import { useCallback, useEffect, useId, useState } from 'react'

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
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { formHintClass } from '../../lib/formClasses'
import { errorMessage } from '../../utils/error'
import { formatDateTime } from '../../utils/formatters'
import { useAuth } from '../auth/AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'
import {
  adminActiveBadgeClass,
  adminBadgePendingClass,
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminFormLabelClass,
  adminInputClass,
  adminLabelTextClass,
  adminRangeSliderClass,
} from './adminClasses'
import { AdminFormSection } from './AdminFormSection'
import { AdminPanelShell } from './AdminPanelShell'

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
      className={cn(
        fromDB ? adminActiveBadgeClass(true) : adminBadgePendingClass,
        'ml-2 align-middle',
      )}
      aria-label={
        fromDB
          ? t('admin.platform_settings.source_db_aria')
          : t('admin.platform_settings.source_env_aria')
      }
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
    <label className="border-border bg-card-raised flex cursor-pointer items-start gap-3 rounded-lg border p-4">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        disabled={disabled}
        className="mt-0.75"
      />
      <span>
        <strong className="mb-1 block">
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
    <label className={cn(adminFormLabelClass, 'max-w-90 gap-1')} htmlFor={inputId}>
      <span className={adminLabelTextClass}>
        {label}
        <SourceBadge source={source} />
      </span>
      <div className="mt-1 flex items-center gap-1">
        <button
          type="button"
          className={cn(
            adminBtnSecondaryClass,
            'm-0 flex h-8.5 min-h-0 items-center justify-center px-3 py-1 text-[1.1rem] font-bold',
          )}
          onClick={decrement}
          disabled={disabled || value <= min}
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
          className={cn(adminInputClass, 'm-0 h-8.5 max-w-18 text-center')}
        />
        <button
          type="button"
          className={cn(
            adminBtnSecondaryClass,
            'm-0 flex h-8.5 min-h-0 items-center justify-center px-3 py-1 text-[1.1rem] font-bold',
          )}
          onClick={increment}
          disabled={disabled || value >= max}
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
    <label className={cn(adminFormLabelClass, 'max-w-90 gap-1')} htmlFor={inputId}>
      <span className={adminLabelTextClass}>
        {label}
        <SourceBadge source={source} />
      </span>
      <div className="mt-1 flex items-center gap-3">
        <input
          id={inputId}
          type="range"
          min={0}
          max={100}
          step={5}
          value={pctValue}
          onChange={(e) => onChange(Number(e.target.value) / 100)}
          disabled={disabled}
          className={cn(adminRangeSliderClass, 'flex-1 cursor-pointer')}
        />
        <div className="bg-card-raised border-border text-foreground min-w-14 rounded-md border px-2 py-1 text-center text-[0.82rem] font-bold">
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
}

function GeneralSettingsCard({
  selfSignupEnabled,
  setSelfSignupEnabled,
  canEdit,
  saving,
  updatedAt,
  onSave,
}: GeneralSettingsCardProps) {
  const t = useT()
  const [locale] = useLocale()

  return (
    <AdminFormSection title={t('admin.platform_settings.title')}>
      <label className="border-border bg-card-raised flex cursor-pointer items-start gap-3 rounded-lg border p-4">
        <input
          type="checkbox"
          checked={selfSignupEnabled}
          onChange={(e) => setSelfSignupEnabled(e.target.checked)}
          disabled={!canEdit}
          className="mt-0.75"
        />
        <span>
          <strong className="mb-1 block">{t('admin.platform_settings.self_signup_label')}</strong>
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
            date: formatDateTime(updatedAt, localeLanguageTag(locale)),
          })}
        </p>
      )}

      <div className="flex gap-2">
        <button
          type="button"
          className={adminBtnPrimaryClass}
          disabled={saving || !canEdit}
          onClick={onSave}
        >
          {saving ? t('common.saving') : t('common.save')}
        </button>
      </div>
    </AdminFormSection>
  )
}

interface AmbiguitySettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setAmbiguity: (patch: Partial<RuntimeConfigDraft['ambiguity']>) => void
}

function AmbiguitySettingsCard({
  draft,
  canEditRuntime,
  sources,
  setAmbiguity,
}: AmbiguitySettingsCardProps) {
  const t = useT()
  const ambiguitySources = sources ?? {}
  return (
    <AdminFormSection title={t('admin.platform_settings.ambiguity_title')}>
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
    </AdminFormSection>
  )
}

interface PIISettingsCardProps {
  config: AIAdminRuntimeConfig
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setPII: (patch: Partial<RuntimeConfigDraft['pii']>) => void
}

function PIISettingsCard({ config, draft, canEditRuntime, sources, setPII }: PIISettingsCardProps) {
  const t = useT()
  const piiSources = sources ?? {}
  return (
    <AdminFormSection title={t('admin.platform_settings.pii_title')}>
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
    </AdminFormSection>
  )
}

interface MemorySettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setMemory: (patch: Partial<RuntimeConfigDraft['memory']>) => void
}

function MemorySettingsCard({
  draft,
  canEditRuntime,
  sources,
  setMemory,
}: MemorySettingsCardProps) {
  const t = useT()
  const memorySources = sources ?? {}
  return (
    <AdminFormSection title={t('admin.platform_settings.memory_title')}>
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
    </AdminFormSection>
  )
}

interface QueueSettingsCardProps {
  draft: RuntimeConfigDraft
  canEditRuntime: boolean
  sources?: Record<string, RuntimeConfigSource>
  setQueue: (patch: Partial<RuntimeConfigDraft['queue']>) => void
}

function QueueSettingsCard({ draft, canEditRuntime, sources, setQueue }: QueueSettingsCardProps) {
  const t = useT()
  const queueSources = sources ?? {}
  return (
    <AdminFormSection title={t('admin.platform_settings.queue_title')}>
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
    </AdminFormSection>
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
}: RuntimeSettingsSectionProps) {
  const t = useT()
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
      />
      <MemorySettingsCard
        draft={draft}
        canEditRuntime={canEditRuntime}
        sources={memorySources}
        setMemory={setMemory}
      />
      <QueueSettingsCard
        draft={draft}
        canEditRuntime={canEditRuntime}
        sources={queueSources}
        setQueue={setQueue}
      />
      <div className="flex flex-col gap-4">
        <p className={cn(formHintClass, 'm-0')}>
          {anyDBOverride
            ? t('admin.platform_settings.ambiguity_db_override_note')
            : t('admin.platform_settings.ambiguity_env_default_note')}
        </p>
        <div className="flex gap-2">
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
      toast.error(errorMessage(e))
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
      toast.error(errorMessage(e))
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
      toast.error(errorMessage(e))
    }
  }

  if (loading) {
    return <LoadingScreen minHeight="200px" />
  }

  const ambiguitySources = config?.ambiguity.sources ?? {}

  return (
    <AdminPanelShell
      title={t('admin.platform_settings.title')}
      description={
        <>
          {t('admin.platform_settings.description')}{' '}
          <a
            href="https://github.com/biqly/biqly/blob/main/docs/configuration.md"
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent underline"
          >
            {t('admin.platform_settings.all_keys_reference')}
          </a>
        </>
      }
      readOnly={!canEdit}
      error={runtimeError}
      maxWidth={1000}
    >
      <div className="mt-6 grid w-full grid-cols-[repeat(auto-fit,minmax(400px,1fr))] items-start gap-6">
        {/* LEFT COLUMN */}
        <div className="flex flex-col gap-6">
          <GeneralSettingsCard
            selfSignupEnabled={selfSignupEnabled}
            setSelfSignupEnabled={setSelfSignupEnabled}
            canEdit={canEdit}
            saving={saving}
            updatedAt={updatedAt}
            onSave={() => void save()}
          />
          {config && draft && (
            <AmbiguitySettingsCard
              draft={draft}
              canEditRuntime={canEditRuntime}
              sources={ambiguitySources}
              setAmbiguity={setAmbiguity}
            />
          )}
        </div>

        {/* RIGHT COLUMN */}
        <div className="flex flex-col gap-6">
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
            />
          )}
        </div>
      </div>
    </AdminPanelShell>
  )
}
