/* eslint-disable react-refresh/only-export-components */
import type { AIProviderType, AIPurpose } from '../../api/aiProviders'

export const PROVIDER_TYPES: AIProviderType[] = ['openai', 'openai-compatible', 'anthropic']
export const PURPOSES: AIPurpose[] = ['query', 'describe', 'embedding', 'translation', 'judge']

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
  style,
}: {
  label: string
  children: React.ReactNode
  style?: React.CSSProperties
}) {
  return (
    <label style={{ display: 'flex', flexDirection: 'column', gap: 4, ...style }}>
      <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-secondary, #a1a1aa)' }}>
        {label}
      </span>
      {children}
    </label>
  )
}

export const aiModalFormStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 14,
}
export const aiModalInputStyle: React.CSSProperties = {
  padding: '8px 10px',
  background: 'var(--bg-input, rgba(255,255,255,0.04))',
  border: '1px solid var(--border, rgba(255,255,255,0.1))',
  borderRadius: 6,
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 13,
  width: '100%',
}
export const aiModalCheckboxRow: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  fontSize: 13,
}
export const aiModalActions: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'flex-end',
  gap: 8,
  marginTop: 8,
}
export const aiModalPrimaryBtn: React.CSSProperties = {
  padding: '8px 14px',
  background: 'var(--accent)',
  color: '#fff',
  border: 'none',
  borderRadius: 6,
  fontSize: 13,
  fontWeight: 600,
  cursor: 'pointer',
}
export const aiModalSecondaryBtn: React.CSSProperties = {
  padding: '8px 14px',
  background: 'transparent',
  color: 'var(--text-primary, #f4f4f5)',
  border: '1px solid var(--border-strong, rgba(255,255,255,0.12))',
  borderRadius: 6,
  fontSize: 13,
  cursor: 'pointer',
}
export const aiModalLinkBtn: React.CSSProperties = {
  padding: '4px 8px',
  background: 'transparent',
  border: 'none',
  color: 'var(--accent)',
  fontSize: 12,
  cursor: 'pointer',
}
