import { cn } from './cn'

export const emptyStateClass = 'grid justify-items-start gap-3 text-foreground-muted'

export const uiEmptyStateClass =
  'grid gap-[0.35rem] justify-items-center text-center py-5 px-2 text-foreground-muted'

export const uiEmptyStateInlineClass = cn(
  uiEmptyStateClass,
  'justify-items-start text-left py-[0.35rem] px-0',
)

export const uiEmptyStateTitleClass =
  'm-0 text-[0.94rem] font-semibold tracking-[-0.02em] text-foreground'

export const uiEmptyStateDescClass = 'm-0 max-w-[28rem] text-[0.82rem] leading-[1.48]'

export const uiEmptyStateSlotClass = 'mt-[0.4rem]'

export const uiEmptyStateIconClass = cn(
  'inline-grid place-items-center w-12 h-12 mb-[0.35rem] rounded-full',
  'bg-[var(--accent-glow)] text-accent',
  '[&_svg]:w-6 [&_svg]:h-6',
)

export const uiEmptyStateActionClass = 'w-auto mt-[0.65rem]'

export const errorAlertClass = cn(
  'border border-[color-mix(in_srgb,var(--error)_22%,transparent)] rounded-lg',
  'bg-[color-mix(in_srgb,var(--error)_8%,transparent)] text-[#fecdd3]',
  'px-[0.85rem] py-[0.7rem] text-[0.85rem] mb-4',
)

export const errorAlertTopGapClass = 'mt-4'

export const successTextClass = 'text-success'

export const loadingTextClass = 'text-foreground-faint text-[0.85rem] my-2'

export const loadingOverlayWrapClass = 'relative'

export const loadingOverlayClass = cn(
  'absolute inset-0 flex items-center justify-center gap-[0.6rem]',
  'bg-[rgba(9,9,11,0.75)] backdrop-blur-[6px] [-webkit-backdrop-filter:blur(6px)]',
  'rounded-[inherit] z-[5] text-foreground font-[Plus_Jakarta_Sans,sans-serif]',
  'text-[0.85rem] font-semibold [data-theme=light]:bg-[rgba(248,250,252,0.75)]',
)

export const loadingOverlaySpinnerClass = cn(
  'w-[1.15rem] h-[1.15rem] border-2 border-border-strong border-t-accent rounded-full',
  'animate-loading-spin shadow-[0_0_8px_var(--accent-glow)]',
)

export const warningPanelClass = cn(
  'grid gap-[0.6rem] border border-[color-mix(in_srgb,var(--warning)_30%,transparent)]',
  'border-l-[3px] border-l-warning rounded-lg',
  'bg-[color-mix(in_srgb,var(--warning)_7%,transparent)] text-foreground-muted',
  'px-4 py-[0.85rem] mb-4',
)

export const warningPanelStrongClass = 'block text-warning text-[0.92rem] mb-1'

export const warningPanelPClass = 'm-0 text-foreground-muted text-[0.8rem] leading-[1.45]'

export const warningPanelListClass = 'grid gap-[0.45rem] m-0 p-0 list-none'

export const warningPanelLiClass = cn(
  'relative min-w-0 rounded-[0.4rem]',
  'bg-[color-mix(in_srgb,var(--warning)_10%,transparent)] text-foreground',
  'text-[0.82rem] leading-[1.45] [overflow-wrap:anywhere]',
  'py-[0.55rem] pr-[0.65rem] pl-[1.65rem]',
  'before:content-[""] before:absolute before:top-[0.72rem] before:left-3',
  'before:w-[0.38rem] before:h-[0.38rem] before:rounded-full before:bg-warning',
)

export const chartContainerClass = 'min-w-0 mt-4'

export const evalStatusBadgeBaseClass =
  'inline-block text-[0.72rem] font-bold uppercase tracking-normal px-2 py-0.5 rounded-full'

export const evalStatusPassBadgeClass = cn(
  evalStatusBadgeBaseClass,
  'bg-[rgba(52,211,153,0.12)] text-success',
)

export const evalStatusFailBadgeClass = cn(
  evalStatusBadgeBaseClass,
  'bg-[rgba(251,113,133,0.12)] text-error',
)

export function evalStatusBadgeClass(pass: boolean): string {
  return pass ? evalStatusPassBadgeClass : evalStatusFailBadgeClass
}

export const abRecommendationBannerBaseClass = 'flex flex-col gap-2 border rounded-lg p-4 mb-6'

export function abRecommendationBannerClass(isWorse: boolean): string {
  return cn(
    abRecommendationBannerBaseClass,
    isWorse
      ? 'bg-[color-mix(in_srgb,var(--error)_8%,var(--bg-card-raised))] border-[color-mix(in_srgb,var(--error)_30%,var(--border))]'
      : 'bg-[color-mix(in_srgb,var(--success)_8%,var(--bg-card-raised))] border-[color-mix(in_srgb,var(--success)_30%,var(--border))]',
  )
}

export function abRecommendationTitleClass(isWorse: boolean): string {
  return cn('flex items-center gap-2 font-semibold', isWorse ? 'text-error' : 'text-success')
}

export const semanticModelSetupClass = cn(
  'flex items-center justify-between gap-3 mt-[0.55rem] border border-border rounded-lg bg-card-alt p-3',
  '[&_strong]:block [&_strong]:mb-[0.2rem] [&_strong]:text-[0.86rem]',
  '[&_p]:m-0 [&_p]:text-foreground-muted [&_p]:text-[0.8rem] [&_p]:leading-[1.4]',
  '[&_ul]:grid [&_ul]:gap-1 [&_ul]:mt-2 [&_ul]:mb-0 [&_ul]:pl-4 [&_ul]:text-error [&_ul]:text-[0.78rem]',
)

export function semanticModelSetupStatusClass(variant?: 'success' | 'error'): string {
  return cn(
    semanticModelSetupClass,
    variant === 'success' &&
      'border-[color-mix(in_srgb,var(--success)_38%,var(--border))] bg-[color-mix(in_srgb,var(--success)_8%,var(--bg-card-alt))]',
    variant === 'error' &&
      'border-[color-mix(in_srgb,var(--error)_45%,var(--border))] bg-[color-mix(in_srgb,var(--error)_8%,var(--bg-card-alt))]',
  )
}

export const sqlPreviewClass = cn(
  'overflow-auto border border-border rounded-lg bg-[#0a0b0e] text-[#e1e7f3]',
  "font-['SFMono-Regular','Cascadia_Code','Monaco','Consolas',monospace]",
  'text-[0.82rem] leading-[1.6] mt-4 p-[0.9rem] whitespace-pre-wrap break-words',
)

export const suggestionBlockClass =
  'rounded-lg border border-border bg-background-muted p-[0.75rem_0.9rem] text-[0.9rem] leading-[1.5] text-foreground'

const LEGACY_FEEDBACK_CLASS_MAP: Record<string, string> = {
  error: errorAlertClass,
  'error--top-gap': errorAlertTopGapClass,
  success: successTextClass,
  'loading-text': loadingTextClass,
  'loading-overlay-wrap': loadingOverlayWrapClass,
  'loading-overlay': loadingOverlayClass,
  'loading-overlay-spinner': loadingOverlaySpinnerClass,
  'warning-panel': warningPanelClass,
  'chart-container': chartContainerClass,
  'sql-preview': sqlPreviewClass,
  'empty-state': emptyStateClass,
  'ui-empty-state': uiEmptyStateClass,
  'ui-empty-state--inline': uiEmptyStateInlineClass,
}

function resolveLegacyFeedbackShell(raw: string): string {
  const tokens = raw.trim().split(/\s+/).filter(Boolean)
  const mapped = tokens.map((token) => LEGACY_FEEDBACK_CLASS_MAP[token] ?? token)
  return cn(...mapped)
}

export function legacyFeedbackClass(raw: string): string {
  return resolveLegacyFeedbackShell(raw)
}
