/**
 * Compact, single-line badge row that names the LLM models a feature uses.
 * Designed to live under a modal/page title so users immediately see "this
 * panel calls model X for generation, model Y for translation, model Z for
 * embeddings" without scrolling into env-var diagnostics.
 *
 * Layout: a horizontal `inline-flex` strip with `overflow-x: auto`, so even
 * long custom model identifiers stay on one line; on narrower viewports the
 * strip scrolls horizontally instead of wrapping.
 */
import type { CSSProperties } from 'react'
import { useT } from '../../i18n'

export interface ModelBadgeRowProps {
  primaryLabel: string
  primaryModel?: string
  primaryNote?: string
  translationModel?: string
  translationNote?: string
  embeddingModel?: string
  embeddingNote?: string
  className?: string
  style?: CSSProperties
}

interface Badge {
  label: string
  model?: string
  note?: string
}

export function ModelBadgeRow(props: ModelBadgeRowProps) {
  const t = useT()
  const badges: Badge[] = [{ label: props.primaryLabel, model: props.primaryModel, note: props.primaryNote }]
  if (props.translationModel) badges.push({ label: t('common.model_badge_translate'), model: props.translationModel, note: props.translationNote })
  if (props.embeddingModel) badges.push({ label: t('common.model_badge_embedding'), model: props.embeddingModel, note: props.embeddingNote })

  const containerStyle: CSSProperties = {
    display: 'flex',
    flexWrap: 'nowrap',
    gap: '0.5rem',
    alignItems: 'center',
    overflowX: 'auto',
    maxWidth: '100%',
    padding: '0.25rem 0',
    fontSize: '0.78rem',
    color: 'var(--text-secondary)',
    ...(props.style ?? {}),
  }

  const pillStyle: CSSProperties = {
    display: 'inline-flex',
    flex: '0 0 auto',
    alignItems: 'center',
    gap: '0.35rem',
    padding: '0.15rem 0.5rem',
    borderRadius: '999px',
    background: 'var(--surface-2, rgba(255,255,255,0.05))',
    border: '1px solid var(--border, rgba(255,255,255,0.08))',
    whiteSpace: 'nowrap',
  }

  return (
    <div
      className={'model-badge-row' + (props.className ? ' ' + props.className : '')}
      style={containerStyle}
      aria-label={t('common.model_badge_models_aria')}
    >
      {badges.map((b) => (
        <span key={b.label} style={pillStyle}>
          <strong style={{ fontWeight: 600 }}>{b.label}:</strong>
          <code translate="no" style={{ background: 'transparent', padding: 0 }}>{b.model && b.model.trim() !== '' ? b.model : t('common.em_dash')}</code>
          {b.note ? <span style={{ opacity: 0.7 }}>· {b.note}</span> : null}
        </span>
      ))}
    </div>
  )
}
