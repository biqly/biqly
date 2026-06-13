export const aiJobFabClass =
  'fixed right-5 bottom-5 z-[calc(var(--z-nav,50)+5)] inline-flex cursor-pointer items-center gap-[0.55rem] rounded-full border border-border-strong bg-card px-[0.9rem] py-[0.55rem] text-foreground shadow-[0_10px_28px_rgba(0,0,0,0.35)] transition-[border-color,box-shadow,transform] duration-120 hover:-translate-y-px hover:border-accent'

export const aiJobFabAlertClass = 'border-red-400/55'

export const aiJobFabPulseClass =
  'size-[0.55rem] rounded-full bg-emerald-400 shadow-[0_0_0_0_rgba(52,211,153,0.5)] motion-safe:animate-[ai-job-pulse_1.6s_ease_infinite] motion-reduce:animate-none motion-reduce:shadow-none'

export const aiJobFabPulseIdleClass = 'motion-safe:animate-none shadow-none'

export const aiJobFabPulseAlertClass = 'bg-red-400 motion-safe:animate-none shadow-none'

export const aiJobFabPctClass = 'text-[0.85rem] text-foreground-muted tabular-nums'

export const aiJobPanelClass =
  'fixed right-5 bottom-5 z-[calc(var(--z-nav,50)+5)] max-h-[min(70vh,32rem)] w-[min(24rem,calc(100vw-2rem))] overflow-auto rounded-xl border border-border-strong bg-card shadow-[0_16px_48px_rgba(0,0,0,0.45)] motion-safe:animate-[ai-job-panel-in_180ms_cubic-bezier(0.16,1,0.3,1)] motion-reduce:animate-none'

export const aiJobPanelHeadClass =
  'sticky top-0 z-[1] flex items-start justify-between gap-3 border-b border-border bg-card px-4 py-[0.85rem]'

export const aiJobPanelTitleClass = 'm-0 text-[0.95rem]'

export const aiJobPanelSubClass = 'mt-[0.2rem] mb-0 text-[0.78rem] text-foreground-muted'

export const aiJobPanelActionsClass = 'flex shrink-0 gap-1'

export const aiJobPanelManageClass = 'flex flex-wrap items-start gap-[0.35rem] px-3 pb-2'

export const aiJobPanelStaleClass =
  'mt-[0.35rem] w-full rounded-[0.45rem] border border-dashed border-border p-[0.5rem_0.65rem] text-[0.8rem]'

export const aiJobPanelStaleHeadClass = 'mb-[0.35rem] text-foreground-muted'

export const aiJobPanelStaleListClass = 'm-0 mb-2 grid list-none gap-[0.35rem] p-0'

export const aiJobPanelStaleItemClass = 'flex items-center justify-between gap-2'

export const aiJobPanelStaleEmptyClass = 'm-0 text-[0.8rem] text-foreground-muted'

export const aiJobPanelListClass = 'grid gap-[0.65rem] p-3'

export const aiJobCardBaseClass =
  'rounded-[0.55rem] border border-border bg-canvas px-[0.65rem] py-[0.55rem] transition-[border-color] duration-120'

export const aiJobCardActiveClass = 'border-blue-400/40'

export const aiJobCardFailedClass = 'border-red-400/40'

export const aiJobCardDoneClass = 'border-emerald-400/30'

export function aiJobCardClass(modifier: '' | 'active' | 'failed' | 'done'): string {
  const extra =
    modifier === 'active'
      ? aiJobCardActiveClass
      : modifier === 'failed'
        ? aiJobCardFailedClass
        : modifier === 'done'
          ? aiJobCardDoneClass
          : ''
  return [aiJobCardBaseClass, extra].filter(Boolean).join(' ')
}

export const aiJobCardHeadClass = 'flex items-start justify-between gap-2'

export const aiJobCardToggleClass =
  'group flex min-w-0 flex-1 cursor-pointer items-start gap-[0.45rem] rounded-[0.35rem] border-0 bg-transparent p-[0.1rem_0] text-left font-[inherit] text-inherit focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent'

export const aiJobCardChevronClass =
  'mt-[0.1rem] w-[0.9rem] shrink-0 text-foreground-muted before:inline-block before:text-[0.8rem] before:transition-transform before:duration-140 before:content-["▸"] group-aria-expanded:before:rotate-90'

export const aiJobCardProgressClass = 'mt-2 h-1 overflow-hidden rounded-full bg-border'

export const aiJobCardProgressFillClass =
  'block h-full rounded-[inherit] bg-gradient-to-r from-accent to-emerald-400 transition-[width] duration-[600ms] ease-out'

export const aiJobCardActionsClass = 'flex shrink-0 items-center gap-1'

export const aiJobCardCancelClass = 'text-danger'

export const aiJobCardCancelledClass = 'm-0 text-[0.8125rem] text-foreground-muted'

export const aiJobCardHintClass = 'mt-[0.35rem] mb-0 text-[0.75rem] text-foreground-muted'

export const aiJobCardTitlesClass = 'grid min-w-0 gap-[0.15rem]'

export const aiJobCardTitleClass = 'overflow-hidden text-[0.86rem] text-ellipsis whitespace-nowrap'

export const aiJobCardMetaClass = 'text-[0.72rem] text-foreground-muted'

export const aiJobCardBodyClass = 'mt-[0.65rem] grid gap-[0.55rem]'

export const aiJobCardTotalClass =
  'm-0 flex items-center justify-between gap-2 border-t border-dashed border-border pt-[0.45rem] text-[0.75rem] text-foreground-muted'

export const aiJobCardTotalTimeClass = 'font-semibold text-foreground tabular-nums'

export const aiJobCardErrorClass = 'm-0 text-[0.78rem] text-red-200'

export const aiJobPipelineClass = 'm-0 grid list-none gap-[0.35rem] p-0'

const aiJobPipelineStepBase = 'flex items-center gap-[0.45rem] text-[0.75rem] text-foreground-muted'

export function aiJobPipelineStepClass(state: 'done' | 'current' | 'pending' | 'failed'): string {
  const stateClass =
    state === 'current'
      ? 'font-semibold text-foreground'
      : state === 'done'
        ? 'text-emerald-200'
        : state === 'failed'
          ? 'text-red-200'
          : ''
  return [aiJobPipelineStepBase, stateClass].filter(Boolean).join(' ')
}

export function aiJobPipelineDotClass(state: 'done' | 'current' | 'pending' | 'failed'): string {
  const base = 'size-[0.45rem] shrink-0 rounded-full bg-border-strong'
  if (state === 'current') {
    return `${base} bg-blue-400 shadow-[0_0_0_0_rgba(96,165,250,0.5)] motion-safe:animate-[ai-job-step-pulse_1.4s_ease_infinite] motion-reduce:animate-none`
  }
  if (state === 'done') {
    return `${base} bg-emerald-400`
  }
  if (state === 'failed') {
    return `${base} bg-red-400`
  }
  return base
}

export const aiJobPipelineLabelClass =
  'min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap'

export const aiJobPipelineTimeClass = 'shrink-0 text-[0.72rem] text-foreground-muted tabular-nums'

export function aiJobPipelineTimeClassForState(
  state: 'done' | 'current' | 'pending' | 'failed',
): string {
  return state === 'current' ? `${aiJobPipelineTimeClass} text-foreground` : aiJobPipelineTimeClass
}

export const aiHistoryClass = 'flex flex-col gap-4'

export const aiHistoryHeaderClass = 'flex items-center justify-between gap-4'

export const aiHistoryHeaderTitleClass = '[&_h2]:m-0'

export const aiHistoryToggleClass =
  'flex cursor-pointer items-center gap-1.5 text-[13px] text-foreground-muted [&_input]:cursor-pointer'

export const aiHistoryErrorClass =
  'rounded-md border border-red-500/25 bg-red-500/8 px-3.5 py-2.5 text-[13px] text-red-600'

export const aiHistoryLoadingClass = 'text-[13px] text-foreground-muted'

export const aiHistoryEmptyClass = 'text-[13px] text-foreground-muted'

export const aiHistoryTableWrapClass = 'max-w-full overflow-x-auto'

export const aiHistoryTableClass =
  'w-full table-fixed border-collapse text-[13px] [&_td]:border-b [&_td]:border-border [&_td]:px-2.5 [&_td]:py-2 [&_td]:text-left [&_th]:border-b [&_th]:border-border [&_th]:px-2.5 [&_th]:py-2 [&_th]:text-left [&_th]:text-[11px] [&_th]:font-semibold [&_th]:tracking-wide [&_th]:text-foreground-muted [&_th]:uppercase [&_th]:whitespace-nowrap'

export const aiHistoryQuestionClass =
  'max-w-[280px] overflow-hidden text-ellipsis whitespace-nowrap'

export type AiHistoryStatusVariant = 'success' | 'error' | 'clarification' | 'active'

export const aiHistoryStatusBaseClass =
  'inline-block rounded-[10px] px-2 py-0.5 text-[11px] font-semibold'

export function aiHistoryStatusClass(variant: AiHistoryStatusVariant): string {
  const variantClass =
    variant === 'success'
      ? 'bg-emerald-500/12 text-success'
      : variant === 'error'
        ? 'bg-red-500/10 text-error'
        : variant === 'clarification'
          ? 'bg-amber-500/12 text-warning'
          : 'bg-indigo-500/12 text-accent'
  return `${aiHistoryStatusBaseClass} ${variantClass}`
}

export const aiHistoryMonoClass = 'font-mono text-xs'

export const aiHistoryDetailBtnClass =
  'cursor-pointer rounded border border-border px-2 py-0.5 text-[11px] text-foreground-muted hover:bg-indigo-500/6'

export const aiHistoryActionsClass =
  'inline-flex items-center gap-1.5 [&_.share-btn]:px-2 [&_.share-btn]:py-0.5'

export const aiHistoryRowExpandedClass = 'bg-indigo-500/[0.03]'

export const aiHistoryDetailRowClass = '[&_td]:!border-b-2 [&_td]:!border-accent [&_td]:!p-0'

export const aiHistoryDetailCellClass = 'max-w-0 w-full align-top'

export const aiHistoryDetailContentClass = 'flex min-w-0 max-w-full flex-col gap-3 px-4 py-3'

export const aiHistoryDetailHeaderClass =
  'sticky top-0 z-[1] -mx-4 -mt-3 mb-1 flex items-center justify-between gap-3 border-b border-border bg-card px-4 py-2'

export const aiHistoryDetailCloseBtnClass =
  'shrink-0 cursor-pointer rounded border border-border px-2.5 py-1 text-[11px] font-medium text-foreground-muted hover:bg-indigo-500/6 hover:text-foreground'

export const aiHistoryDetailBlockClass =
  'min-w-0 max-w-full [&_h4]:m-0 [&_h4]:mb-1.5 [&_h4]:text-[12px] [&_h4]:font-semibold [&_h4]:tracking-wide [&_h4]:text-foreground-muted [&_h4]:uppercase [&_pre]:m-0 [&_pre]:max-h-[200px] [&_pre]:max-w-full [&_pre]:overflow-x-auto [&_pre]:overflow-y-auto [&_pre]:rounded-md [&_pre]:bg-canvas-subtle [&_pre]:p-2.5 [&_pre]:text-[12px] [&_pre]:break-all [&_pre]:whitespace-pre-wrap [&_pre]:text-foreground [[data-theme=dark]_&_pre]:bg-white/5'

export const aiHistoryLoadMoreClass =
  'cursor-pointer self-center rounded-md border border-accent px-5 py-2 text-[13px] text-accent hover:bg-indigo-500/6'

export const aiJobsTableRowClass =
  'cursor-pointer transition-[background-color] duration-150 hover:bg-[var(--bg-hover,rgba(255,255,255,0.04))]'

export const jobDetailGridClass =
  'grid items-start gap-6 md:max-[768px]:gap-4 max-[768px]:grid-cols-1 grid-cols-[1.2fr_1.8fr]'

export const jobDetailSectionClass =
  'flex flex-col gap-4 rounded-lg border border-border bg-canvas p-4'

export const jobDetailItemClass = 'flex flex-col gap-1'

export const jobDetailLabelClass =
  'text-[0.75rem] font-semibold tracking-wide text-foreground-muted uppercase'

export const jobDetailValueClass = 'break-all text-[0.875rem] text-foreground'

export const jobDetailValueMonoClass = 'font-mono text-[0.8125rem]'

export const jobProgressBarClass = 'mt-1 h-2 overflow-hidden rounded bg-border'

export const jobProgressBarFillClass =
  'h-full rounded-[inherit] bg-gradient-to-r from-accent to-emerald-400 transition-[width] duration-[400ms] ease-out'

export const jobJsonBlockClass =
  'm-0 max-h-[28rem] overflow-auto wrap-break-word rounded-md border border-border bg-canvas p-[0.85rem] font-mono text-[0.78rem] whitespace-pre-wrap text-foreground'

export const jobErrorBlockClass =
  'wrap-break-word rounded-md border border-red-500/20 bg-red-500/8 p-3 text-[0.875rem] whitespace-pre-wrap text-danger'
