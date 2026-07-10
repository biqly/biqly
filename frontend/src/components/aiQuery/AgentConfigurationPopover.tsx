import { useEffect, useId, useRef, useState } from 'react'

import type { TranslationKey } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'

interface AgentConfigurationPopoverProps {
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  contextAvailable: boolean
  contextEnabled: boolean
  onContextEnabledChange: (enabled: boolean) => void
  autoFindEnabled: boolean
  onAutoFindEnabledChange: (enabled: boolean) => void
  agentModeEnabled: boolean
  onAgentModeEnabledChange: (enabled: boolean) => void
}

interface ConfigurationToggleProps {
  checked: boolean
  description: string
  label: string
  onChange: (enabled: boolean) => void
}

function ConfigurationToggle({ checked, description, label, onChange }: ConfigurationToggleProps) {
  return (
    <label className="flex cursor-pointer items-start justify-between gap-4 py-2.5">
      <span className="min-w-0">
        <span className="text-foreground block text-sm font-medium">{label}</span>
        <span className="text-foreground-muted mt-0.5 block text-xs leading-relaxed">
          {description}
        </span>
      </span>
      <input
        type="checkbox"
        className="accent-accent mt-0.5 h-4 w-4 shrink-0"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
    </label>
  )
}

export function AgentConfigurationPopover({
  t,
  contextAvailable,
  contextEnabled,
  onContextEnabledChange,
  autoFindEnabled,
  onAutoFindEnabledChange,
  agentModeEnabled,
  onAgentModeEnabledChange,
}: AgentConfigurationPopoverProps) {
  const [open, setOpen] = useState(false)
  const panelId = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const activeCount =
    Number(contextAvailable && contextEnabled) + Number(autoFindEnabled) + Number(agentModeEnabled)

  useEffect(() => {
    if (!open) {
      return
    }
    const close = (returnFocus: boolean) => {
      setOpen(false)
      if (returnFocus) {
        triggerRef.current?.focus()
      }
    }
    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        close(false)
      }
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        close(true)
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  return (
    <div ref={rootRef} className="relative">
      <button
        ref={triggerRef}
        type="button"
        className={cn(
          buttonClass('ghost'),
          'mt-0! h-8! w-auto! gap-1.5 rounded-full! px-2.5! text-xs',
          open && 'border-accent! bg-accent/10! text-accent!',
        )}
        aria-expanded={open}
        aria-controls={panelId}
        aria-haspopup="dialog"
        onClick={() => setOpen((current) => !current)}
      >
        <span aria-hidden="true">⚙</span>
        {t('ai_query.agent_config')}
        <span className="bg-accent/15 text-accent rounded-full px-1.5 py-0.5 text-[0.65rem] font-semibold">
          {t('ai_query.agent_config_status', { count: activeCount })}
        </span>
      </button>

      {open && (
        <div
          id={panelId}
          role="dialog"
          aria-label={t('ai_query.agent_config_title')}
          className="border-border bg-card-raised shadow-card absolute bottom-full left-0 z-40 mb-2 w-[min(22rem,calc(100vw-2rem))] rounded-xl border p-3"
        >
          <div className="border-border border-b pb-2">
            <p className="text-foreground m-0 text-sm font-semibold">
              {t('ai_query.agent_config_title')}
            </p>
            <p className="text-foreground-muted mt-1 mb-0 text-xs leading-relaxed">
              {t('ai_query.agent_config_desc')}
            </p>
          </div>
          <div className="divide-border divide-y">
            {contextAvailable && (
              <ConfigurationToggle
                checked={contextEnabled}
                label={t('ai_query.context_toggle')}
                description={t('ai_query.context_toggle_desc')}
                onChange={onContextEnabledChange}
              />
            )}
            <ConfigurationToggle
              checked={autoFindEnabled}
              label={t('ai_query.auto_find_toggle')}
              description={t('ai_query.auto_find_toggle_title')}
              onChange={onAutoFindEnabledChange}
            />
            <ConfigurationToggle
              checked={agentModeEnabled}
              label={t('ai_query.agent_mode_toggle')}
              description={t('ai_query.agent_mode_toggle_title')}
              onChange={onAgentModeEnabledChange}
            />
          </div>
          <div className="bg-canvas-subtle text-foreground-muted mt-2 flex items-start gap-2 rounded-lg px-2.5 py-2 text-xs">
            <span aria-hidden="true">⌁</span>
            <span>
              <strong className="text-foreground font-medium">
                {t('ai_query.agent_config_caps_title')}
              </strong>{' '}
              {t('ai_query.agent_config_caps_managed')}
            </span>
          </div>
        </div>
      )}
    </div>
  )
}
