import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'

const qbStepLabelTheme: Record<string, string> = {
  data: 'text-indigo-600 dark:text-indigo-400',
  join: 'text-sky-600 dark:text-sky-400',
  filter: 'text-purple-600 dark:text-purple-400',
  fields: 'text-pink-600 dark:text-pink-400',
  summarize: 'text-emerald-600 dark:text-emerald-400',
  sort: 'text-rose-600 dark:text-rose-400',
  limit: 'text-slate-600 dark:text-slate-400',
  advanced: 'text-amber-600 dark:text-amber-400',
}

const qbStepCardTheme: Record<string, string> = {
  data: 'border-l-[3px] border-l-indigo-500',
  join: 'border-l-[3px] border-l-sky-600 flex-col! items-stretch! gap-[0.65rem] py-3 px-4 w-full',
  filter: 'border-l-[3px] border-l-purple-500',
  fields: 'border-l-[3px] border-l-pink-500',
  summarize: 'border-l-[3px] border-l-emerald-500',
  sort: 'border-l-[3px] border-l-rose-500',
  limit: 'border-l-[3px] border-l-slate-500',
  advanced: 'border-l-[3px] border-l-amber-600',
}

export const qbCardClass = 'rounded-2xl border border-border bg-card p-5 pb-[1.6rem] shadow-card'

// Header is now a vertical stack of two labeled-group rows (data/saved, then
// options). Each row groups controls with a shared visual rhythm.
export const qbHeaderClass = 'mb-6 flex flex-col gap-[1.1rem] border-b border-border pb-5'

// A row that lays several labeled groups side by side, wrapping on small screens.
export const qbHeaderRowClass =
  'flex min-w-0 flex-wrap items-end gap-x-6 gap-y-3 max-[720px]:flex-col max-[720px]:items-stretch'

// A labeled group shell: uppercase caption label above the control(s).
export const qbHeaderGroupClass =
  'flex min-w-0 flex-col gap-[0.4rem] [&_.ui-select]:min-w-36 [&_.ui-select]:max-w-64 [&_.ui-select]:flex-[0_1_auto] max-[720px]:[&_.ui-select]:max-w-none max-[720px]:[&_.ui-select]:flex-[1_1_12rem]'

export const qbHeaderLabelClass =
  'text-foreground-muted font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.66rem] font-bold tracking-[0.08em] uppercase'

// Inline cluster of controls inside a group (datasource+model, save actions, ...).
export const qbHeaderControlsClass =
  'flex min-w-0 flex-wrap items-center gap-[0.55rem] max-[720px]:w-full'

export const qbSavedDraftActionsClass = 'flex min-w-0 flex-wrap items-center gap-2'

export const qbModeToggleClass = 'shrink-0'

export const qbNotebookClass = 'relative mb-6 flex flex-col gap-3'

export const qbStepRootClass = 'flex items-start gap-4 max-md:flex-col max-md:gap-1'

export const qbStepCollapsedClass = ''

export const qbStepLabelBase =
  'w-28 shrink-0 pt-2 text-right text-[0.78rem] font-bold uppercase leading-snug tracking-wide max-md:pl-2 max-md:text-left'

export const qbStepLabelToggleClass =
  'flex cursor-pointer items-center justify-end gap-[0.3rem] border-0 bg-transparent font-[inherit] max-md:justify-start'

export const qbStepChevronClass = 'text-[0.65rem] opacity-70'

export function qbStepLabelClass(themeClass: string, toggle = false): string {
  const theme = qbStepLabelTheme[themeClass] ?? 'text-foreground-muted'
  return [qbStepLabelBase, theme, toggle ? qbStepLabelToggleClass : ''].filter(Boolean).join(' ')
}

export const qbStepSummaryCardClass =
  'flex-1 cursor-pointer rounded-lg border border-dashed border-border bg-transparent px-3 py-2 text-left text-[0.82rem] text-foreground-muted hover:border-accent hover:text-foreground'

export const qbStepCardBase =
  'relative flex min-h-[2.6rem] min-w-0 flex-1 flex-wrap items-center gap-[0.6rem] rounded-[0.6rem] border border-border-strong bg-card-raised px-[0.85rem] py-[0.6rem] shadow-card-sm transition-[border-color,box-shadow] duration-150 hover:border-[var(--control-active-border)] hover:shadow-[inset_0_0_0_1px_rgba(255,255,255,0.01),var(--shadow-sm)] [&_.ui-select]:min-h-[1.7rem] [&_.ui-select]:text-[0.76rem] [&_input]:min-h-[1.7rem] [&_input]:rounded-[0.35rem] [&_input]:border [&_input]:border-border-strong [&_input]:bg-canvas [&_input]:px-2 [&_input]:py-[0.2rem] [&_input]:text-[0.76rem] [&_input]:text-foreground [&_input]:outline-none [&_input:focus-visible]:border-accent/55 [&_select]:min-h-[1.7rem] [&_select]:text-[0.76rem]'

export function qbStepCardClass(themeClass: string): string {
  const theme = qbStepCardTheme[themeClass] ?? ''
  return [qbStepCardBase, theme].filter(Boolean).join(' ')
}

export const qbStepCloseClass =
  'absolute top-2 right-2 flex cursor-pointer items-center justify-center rounded border-0 bg-transparent p-[0.15rem] text-[0.85rem] leading-none text-foreground-faint transition-[color,background-color] duration-150 hover:bg-border-strong hover:text-foreground'

export const qbTagBase =
  'inline-flex select-none items-center gap-[0.4rem] rounded-[0.4rem] px-[0.6rem] py-1 text-[0.78rem] font-semibold leading-snug [&_.ui-select]:min-w-[8.5rem] [&_.ui-select]:max-w-64 [&_.ui-select]:flex-[1_1_auto]'

export const qbTagBlueClass =
  'border border-indigo-500/22 bg-indigo-500/8 text-indigo-600 dark:text-indigo-300'

export const qbTagGreenClass =
  'border border-emerald-500/22 bg-emerald-500/8 text-emerald-600 dark:text-emerald-300'

export const qbTagPurpleClass =
  'border border-purple-500/22 bg-purple-500/8 text-purple-600 dark:text-purple-300'

export const qbTagTableClass =
  '!rounded-[0.35rem] !border-border-strong !bg-white/[0.04] !px-2 !py-[0.2rem] !font-mono !text-[0.76rem] !text-foreground'

export const qbTagCloseClass =
  'inline-flex cursor-pointer items-center justify-center border-0 bg-transparent px-[0.1rem] text-[0.85rem] font-bold leading-none opacity-60 transition-opacity duration-100 hover:opacity-100'

export const qbAddBtnClass =
  'inline-flex h-[1.55rem] w-[1.55rem] shrink-0 cursor-pointer items-center justify-center rounded-full border border-dashed border-border-strong bg-transparent p-0 text-[0.95rem] leading-none text-foreground-faint transition-all duration-120 hover:border-foreground-muted hover:bg-white/[0.02] hover:text-foreground'

export const qbSummarizeSplitClass =
  'grid w-full gap-3 lg:grid-cols-[minmax(0,1fr)_2rem_minmax(0,1fr)] lg:items-stretch'

export const qbSummarizeSectionClass =
  'flex min-w-0 flex-col gap-3 rounded-lg border border-border bg-card-raised/50 p-3'

export const qbSummarizeDividerClass =
  'hidden select-none items-center justify-center text-lg text-foreground-faint lg:flex'

export const qbSummarizeItemsClass = 'flex min-h-8 flex-wrap items-center gap-2'

export const qbSummarizeHeadingClass = 'text-sm font-semibold text-foreground'

export const qbSummarizeHintClass = 'mt-1 text-xs leading-relaxed text-foreground-muted'

export const qbSummarizeAddClass =
  'inline-flex min-h-8 w-fit cursor-pointer items-center gap-1.5 rounded-md border border-dashed border-border-strong bg-transparent px-2.5 py-1 text-xs font-semibold text-foreground-muted transition-colors hover:border-accent hover:bg-accent/5 hover:text-foreground'

export const qbToolbarClass =
  'mt-4 flex flex-wrap items-center gap-2 border-t border-border px-2 pt-[0.85rem]'

const qbToolbarBtnBase =
  'inline-flex cursor-pointer items-center gap-[0.35rem] rounded-[0.45rem] border border-border-strong bg-card-raised px-3 py-[0.35rem] text-[0.78rem] font-semibold text-foreground-muted transition-all duration-150 hover:border-[var(--control-active-border)] hover:bg-[var(--control-hover-bg)] hover:text-foreground'

export const qbToolbarBtnClass = qbToolbarBtnBase

export const qbToolbarBtnActiveClass =
  'border-accent bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] text-foreground shadow-[0_0_8px_var(--accent-glow)]'

export const qbToolbarBtnFilterActiveClass = 'border-purple-500 bg-purple-500/6 text-foreground'

export const qbToolbarBtnSummarizeActiveClass =
  'border-emerald-500 bg-emerald-500/6 text-foreground'

export const qbToolbarBtnSortActiveClass = 'border-rose-500 bg-rose-500/6 text-foreground'

export const qbToolbarBtnLimitActiveClass = 'border-slate-500 bg-slate-500/6 text-foreground'

export const qbToolbarBtnAdvancedActiveClass = 'border-amber-600 bg-amber-600/6 text-foreground'

export function qbToolbarBtnVariantClass(
  variant: 'filter' | 'summarize' | 'sort' | 'limit' | 'advanced',
  active: boolean,
): string {
  if (!active) {
    return qbToolbarBtnBase
  }
  const activeMap = {
    filter: qbToolbarBtnFilterActiveClass,
    summarize: qbToolbarBtnSummarizeActiveClass,
    sort: qbToolbarBtnSortActiveClass,
    limit: qbToolbarBtnLimitActiveClass,
    advanced: qbToolbarBtnAdvancedActiveClass,
  }
  return [qbToolbarBtnBase, activeMap[variant]].join(' ')
}

export const qbVisualizeContainerClass = 'mt-5 flex items-center gap-[0.6rem]'

// Delegate to shared ui/Button primary styling so the Visualize CTA matches
// <Button variant="primary">. autoWidth keeps it inline (buttonBase is
// full-width). gap-[0.4rem] preserves the original icon+label spacing.
export const qbVisualizeBtnClass = cn(buttonClass('primary', { autoWidth: true }), 'gap-[0.4rem]')

export const qbSqlToggleClass = 'font-mono'

export const qbSqlCardClass = ''

export const qbSqlCardHeadClass = 'flex items-center justify-between gap-3 [&_h2]:m-0'

export const qbJoinFlowClass =
  'flex w-full flex-wrap items-center gap-3 border-b border-white/[0.03] py-[0.4rem] last:border-b-0 last:pb-0 first:pt-0'

export const qbJoinTypeClass =
  'inline-flex items-center gap-[0.3rem] rounded border border-sky-400/30 bg-sky-400/15 px-[0.45rem] py-[0.15rem] font-sans text-[0.68rem] font-bold tracking-wide text-sky-600 dark:text-sky-400 uppercase'

export const qbJoinConnectorClass =
  'inline-flex select-none items-center gap-[0.3rem] text-foreground-faint'

export const qbJoinLineClass = 'h-px w-[0.8rem] bg-border-strong'

export const qbJoinCardinalityClass =
  'rounded border border-border bg-canvas px-[0.3rem] py-[0.05rem] text-[0.65rem] font-semibold whitespace-nowrap text-foreground-muted'

export const qbJoinOnClauseClass =
  'ml-auto inline-flex items-center gap-[0.4rem] text-[0.75rem] max-[900px]:mt-1 max-[900px]:ml-0 max-[900px]:w-full max-[900px]:pl-1'

export const qbJoinOnLabelClass = 'text-[0.68rem] font-bold tracking-wide text-foreground-faint'

export const qbJoinExpressionClass =
  'rounded-[0.35rem] border border-border bg-black/5 dark:bg-black/25 px-2 py-[0.2rem] font-mono text-[0.74rem] text-foreground-muted'

export const qbJoinTablePrefixClass = 'text-indigo-600 dark:text-indigo-300'

export const qbJoinTypeIconClass = 'inline-block shrink-0 align-middle'
