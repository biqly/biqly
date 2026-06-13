import { cn } from './cn'

export const tagPillClass = 'bg-card px-2 py-0.5 rounded text-xs text-foreground-muted'

export const tagPillMetaClass = cn(tagPillClass, 'inline-flex items-center gap-[0.35rem]')

export const savedQuestionTagsClass = 'flex gap-1 mt-2'

export const datasourceAccessNoteClass =
  '-mt-[0.4rem] mb-[0.8rem] text-foreground-muted text-[0.82rem]'

export const datasourceAccessBadgeClass = cn(
  'inline-flex items-center gap-[0.3rem] px-[0.55rem] py-[0.12rem] rounded-full',
  'text-[0.68rem] font-semibold tracking-wide whitespace-nowrap',
  'border border-[color-mix(in_srgb,var(--success)_35%,transparent)]',
  'bg-[color-mix(in_srgb,var(--success)_14%,transparent)]',
  'text-[color-mix(in_srgb,var(--success)_90%,var(--text-primary))]',
)

export const datasourceAccessBadgeIconClass = cn(
  'inline-flex items-center justify-center w-[0.95rem] h-[0.95rem] rounded-full',
  'text-[0.62rem] leading-none bg-success text-white',
)

export const datasourceIdCopyButtonClass = cn(
  'inline-flex items-center px-[0.45rem] py-[0.1rem] border border-border rounded-[0.35rem]',
  'bg-transparent font-[Geist_Mono,ui-monospace,monospace] text-[0.68rem] text-foreground-faint',
  'cursor-copy tracking-wide hover:border-border-strong hover:text-foreground-muted',
  'focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
)

export const promptWarningClass =
  'text-[0.75rem] text-warning bg-[rgba(251,191,36,0.06)] border border-[rgba(251,191,36,0.2)] rounded-[0.35rem] px-[0.6rem] py-[0.3rem] mt-[0.4rem]'

export const wfBadgeClass =
  'inline-block mr-[0.3rem] text-[0.72rem] text-foreground-muted bg-card-raised border border-border rounded-[0.3rem] px-2 py-[0.2rem]'

export const lqMetaBadgesClass = 'flex flex-wrap gap-[0.35rem] mb-2'

export const promptStatsPanelClass = 'flex flex-wrap gap-[0.35rem] mb-2'
