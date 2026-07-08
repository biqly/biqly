/* eslint-disable react-refresh/only-export-components */
import type { AIProviderType, AIPurpose } from '../../api/aiProviders'
import { cn } from '../../lib/cn'

export const PROVIDER_TYPES: AIProviderType[] = ['openai', 'openai-compatible', 'anthropic']
export const PURPOSES: AIPurpose[] = [
  'query',
  'describe',
  'embedding',
  'translation',
  'judge',
  'agent',
]

export function defaultBaseURL(type: AIProviderType): string {
  switch (type) {
    case 'openai':
      return 'https://api.openai.com/v1'
    case 'anthropic':
      return 'https://api.anthropic.com/v1'
    default:
      return ''
  }
}

export function ModalField({
  label,
  children,
  className,
}: {
  label: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <label className={cn('flex flex-col gap-1', className)}>
      <span className="text-caption text-foreground-muted font-semibold">{label}</span>
      {children}
    </label>
  )
}

export const aiModalFormClass = 'flex flex-col gap-3.5'
export const aiModalInputClass =
  'px-2.5 py-2 bg-input border border-border rounded-md text-foreground text-sm w-full focus:outline-none focus:border-accent'
export const aiModalCheckboxRowClass = 'flex items-center gap-2 text-sm'
export const aiModalActionsClass = 'flex justify-end gap-2 mt-2'
export const aiModalPrimaryBtnClass =
  'px-3.5 py-2 bg-accent hover:bg-accent-strong text-white border-0 rounded-md text-sm font-semibold cursor-pointer disabled:opacity-50'
export const aiModalSecondaryBtnClass =
  'px-3.5 py-2 bg-transparent text-foreground border border-border rounded-md text-sm cursor-pointer hover:bg-canvas-subtle'
export const aiModalLinkBtnClass =
  'px-2 py-1 bg-transparent border-0 text-accent text-caption cursor-pointer hover:underline'
