export const compositesPageClass = 'flex min-h-0 flex-col gap-0'

export const compositesControlsRowClass = 'mb-5 flex flex-wrap items-center gap-[0.65rem]'

export const compositesDsSelectClass = 'min-w-56 max-w-[22rem] flex-1'

export const compositesLayoutClass =
  'grid min-h-[28rem] grid-cols-[15rem_1fr] items-start gap-4 max-[900px]:grid-cols-1'

export const compositesDetailClass = 'flex min-w-0 flex-col gap-4'

export const compositesSidebarClass =
  'sticky top-4 overflow-hidden rounded-xl border border-border bg-card'

export const compositesSidebarHeaderClass = 'border-b border-border px-4 pt-3 pb-[0.6rem]'

export const compositesSidebarHeaderTitleClass =
  'm-0 font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.78rem] font-bold tracking-wide text-foreground-muted uppercase'

export const compositesSidebarListClass = 'm-0 max-h-[60vh] list-none overflow-y-auto py-[0.35rem]'

export const compositesSidebarItemClass =
  'flex items-center gap-0 border-b border-border last:border-b-0'

export const compositesSidebarEmptyClass = 'p-4'

export const compositesListBtnBaseClass =
  'flex min-w-0 flex-1 cursor-pointer flex-col items-start gap-[0.2rem] border-0 bg-transparent px-4 py-[0.6rem] text-left transition-[background-color] duration-[140ms] hover:bg-[var(--control-hover-bg)]'

export function compositesListBtnClass(active: boolean): string {
  return active
    ? `${compositesListBtnBaseClass} bg-[var(--accent-glow)]`
    : compositesListBtnBaseClass
}

export const compositeNameBaseClass =
  'max-w-full overflow-hidden text-[0.84rem] font-semibold text-ellipsis whitespace-nowrap text-foreground'

export function compositeNameClass(active: boolean): string {
  return active ? `${compositeNameBaseClass} text-accent` : compositeNameBaseClass
}

export const compositesDeleteBtnClass =
  'mr-2 inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[0.35rem] border border-transparent bg-transparent text-[1rem] text-foreground-muted transition-all duration-[140ms] hover:border-rose-400/40 hover:bg-rose-400/12 hover:text-rose-200'

const compositeStatusBaseClass =
  'inline-flex items-center gap-[0.3rem] rounded-full border border-transparent px-2 py-[0.15rem] text-[0.7rem] font-semibold tracking-wide uppercase before:inline-block before:size-[0.45rem] before:rounded-full before:bg-current before:opacity-80 before:content-[""]'

export function compositeStatusClass(status: string): string {
  if (status === 'draft') {
    return `${compositeStatusBaseClass} bg-[color-mix(in_srgb,var(--warning)_12%,transparent)] text-warning border-[color-mix(in_srgb,var(--warning)_30%,transparent)]`
  }
  if (status === 'published') {
    return `${compositeStatusBaseClass} bg-[color-mix(in_srgb,var(--success)_12%,transparent)] text-success border-[color-mix(in_srgb,var(--success)_30%,transparent)]`
  }
  if (status === 'error') {
    return `${compositeStatusBaseClass} bg-[color-mix(in_srgb,var(--error)_12%,transparent)] text-error border-[color-mix(in_srgb,var(--error)_30%,transparent)]`
  }
  return `${compositeStatusBaseClass} bg-[color-mix(in_srgb,var(--accent)_12%,transparent)] text-accent border-[color-mix(in_srgb,var(--accent)_30%,transparent)]`
}

export const compositeDetailHeadClass =
  'flex flex-wrap items-start justify-between gap-4 rounded-xl border border-border bg-card px-5 pt-[1.1rem] pb-4'

export const compositeDetailHeadTitleClass =
  'm-0 mb-[0.3rem] font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[1.15rem] font-bold tracking-tight text-foreground'

export const compositeDetailHeadDescClass =
  'm-0 mb-[0.4rem] text-[0.84rem] leading-snug text-foreground-muted'

export const compositeActionsClass = 'flex shrink-0 flex-wrap items-center gap-2'

const compositeValidationBaseClass =
  'rounded-[0.6rem] border border-border px-4 py-[0.85rem] text-[0.84rem] [&_strong]:mb-[0.4rem] [&_strong]:block [&_strong]:text-[0.84rem] [&_strong]:font-bold'

export function compositeValidationClass(valid: boolean): string {
  return valid
    ? `${compositeValidationBaseClass} border-[color-mix(in_srgb,var(--success)_35%,transparent)] bg-[color-mix(in_srgb,var(--success)_8%,transparent)]`
    : `${compositeValidationBaseClass} border-[color-mix(in_srgb,var(--error)_35%,transparent)] bg-[color-mix(in_srgb,var(--error)_8%,transparent)]`
}

export const validationErrorClass = 'py-[0.2rem] text-[0.8rem] text-error'

export const validationWarningClass = 'py-[0.2rem] text-[0.8rem] text-warning'

export const compositeCanvasWrapClass =
  'overflow-x-auto rounded-xl border border-border bg-card p-3'

export const compositeCanvasClass = 'block rounded-lg'

export const compositeCanvasEmptyClass =
  'px-4 py-8 text-center text-[0.84rem] text-foreground-muted'

export const compositeSectionClass = 'rounded-xl border border-border bg-card px-5 py-4'

export const compositeSectionTitleClass =
  'm-0 mb-3 font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.9rem] font-bold tracking-tight text-foreground'

export const sectionHintClass =
  '-mt-[0.35rem] mb-3 text-[0.8rem] leading-snug text-foreground-muted'

export const sectionHeadRowClass =
  'mb-3 flex flex-wrap items-center justify-between gap-2 [&_h3]:m-0 [&>div]:flex [&>div]:gap-[0.45rem]'

export const componentListClass = 'm-0 mb-3 flex list-none flex-col gap-[0.35rem] p-0'

export const componentListItemClass =
  'flex items-center gap-[0.6rem] rounded-lg border border-border bg-card-raised px-3 py-2 text-[0.83rem] transition-[border-color] duration-[140ms] hover:border-border-strong'

export const componentAliasClass = 'min-w-20 font-mono text-[0.78rem] font-bold text-foreground'

export const componentModelClass =
  'min-w-0 flex-1 overflow-hidden text-[0.82rem] text-ellipsis whitespace-nowrap text-foreground-muted'

const componentRoleBaseClass =
  'rounded-full border border-transparent px-[0.45rem] py-[0.15rem] text-[0.72rem] font-bold tracking-wide uppercase'

export function componentRoleClass(role: string): string {
  return role === 'primary'
    ? `${componentRoleBaseClass} bg-[var(--accent-glow)] text-accent border-[color-mix(in_srgb,var(--accent)_30%,transparent)]`
    : `${componentRoleBaseClass} bg-[color-mix(in_srgb,var(--text-muted)_8%,transparent)] text-foreground-muted border-border`
}

export const componentAddRowClass =
  'grid grid-cols-[1fr_8rem_7rem_auto] items-end gap-2 max-[700px]:grid-cols-2'

export const crossJoinListClass = 'm-0 mb-2 flex list-none flex-col gap-[0.35rem] p-0'

export const crossJoinListItemClass =
  'flex flex-wrap items-center gap-[0.65rem] rounded-lg border border-border bg-card-raised px-3 py-2 text-[0.82rem] [&>span:first-child]:min-w-0 [&>span:first-child]:flex-1 [&>span:first-child]:overflow-hidden [&>span:first-child]:font-mono [&>span:first-child]:text-[0.78rem] [&>span:first-child]:text-ellipsis [&>span:first-child]:whitespace-nowrap [&>span:first-child]:text-foreground'

export const joinMetaClass = 'text-[0.74rem] whitespace-nowrap text-foreground-muted'

export const crossJoinSuggestionsClass = 'mt-3 border-t border-border pt-3'

export const crossJoinSuggestionsTitleClass =
  'm-0 mb-2 text-[0.78rem] font-bold tracking-wide text-foreground-muted uppercase'

export const suggestionRowClass =
  'flex flex-wrap items-center gap-[0.65rem] rounded-[0.4rem] px-[0.65rem] py-[0.4rem] text-[0.82rem] transition-[background-color] duration-[120ms] hover:bg-[var(--control-hover-bg)] [&>span:first-child]:min-w-0 [&>span:first-child]:flex-1 [&>span:first-child]:font-mono [&>span:first-child]:text-[0.78rem] [&>span:first-child]:text-foreground'

export const suggestionReasonClass = 'text-[0.75rem] text-foreground-muted italic'

export const canonicalDateGridClass = 'flex flex-col gap-3'

export const canonicalDateModelClass =
  '[&_strong]:mb-[0.4rem] [&_strong]:block [&_strong]:font-mono [&_strong]:text-[0.78rem] [&_strong]:font-bold [&_strong]:tracking-wide [&_strong]:text-foreground-muted [&_strong]:uppercase'

export const canonicalDateDimsClass = 'flex flex-wrap gap-[0.4rem]'

const dateDimChipBaseClass =
  'cursor-pointer rounded-full border border-border bg-card-raised px-[0.65rem] py-1 text-[0.77rem] font-medium text-foreground-muted transition-all duration-[140ms] hover:border-border-strong hover:text-foreground'

export function dateDimChipClass(active: boolean): string {
  return active
    ? `${dateDimChipBaseClass} border-accent bg-[var(--accent-glow)] font-bold text-accent`
    : dateDimChipBaseClass
}

export const resolutionListClass = 'm-0 flex list-none flex-col gap-[0.45rem] p-0'

export const resolutionListItemClass =
  'flex items-center gap-3 rounded-lg border border-border bg-card-raised px-[0.65rem] py-[0.45rem] text-[0.83rem]'

export const resolutionNameClass = 'min-w-0 flex-1 font-mono text-[0.78rem] text-foreground'

export const resolutionEmptyClass =
  '!border-0 !bg-transparent px-1 py-2 text-[0.83rem] text-foreground-muted italic'

export const compositesBtnSecondaryClass =
  'inline-flex min-h-[2.1rem] w-auto cursor-pointer items-center justify-center rounded-lg border border-border-strong bg-card-raised px-[0.85rem] py-[0.4rem] font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.8rem] font-semibold whitespace-nowrap text-foreground shadow-[0_1px_2px_rgba(0,0,0,0.05)] transition-all duration-[160ms] ease-[cubic-bezier(0.4,0,0.2,1)] hover:-translate-y-px hover:border-[var(--control-hover-border)] hover:bg-[var(--control-hover-bg)] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45'

export const compositesBtnPrimaryClass =
  'inline-flex min-h-[2.1rem] w-auto cursor-pointer items-center justify-center rounded-lg border border-accent-strong bg-gradient-to-br from-accent to-accent-strong px-[0.85rem] py-[0.4rem] font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.8rem] font-semibold whitespace-nowrap text-white shadow-[0_2px_8px_var(--accent-glow)] transition-all duration-[160ms] ease-[cubic-bezier(0.4,0,0.2,1)] hover:-translate-y-px hover:border-accent-hover hover:from-accent-hover hover:to-accent-strong hover:shadow-[0_4px_14px_var(--accent-glow)] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45 disabled:shadow-none'

export const compositesBtnLinkClass =
  'inline-flex min-h-0 w-auto cursor-pointer items-center border-0 bg-transparent px-1 py-[0.1rem] text-[0.8rem] font-semibold text-accent underline decoration-transparent transition-[color,text-decoration-color] duration-[120ms] hover:text-accent-hover hover:decoration-current'

export const compositesBtnIconDangerClass =
  'inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[0.35rem] border border-transparent bg-transparent text-[1.05rem] leading-none text-foreground-muted transition-all duration-[140ms] hover:border-rose-400/40 hover:bg-rose-400/12 hover:text-rose-200'

export const compositeCreateFormClass = 'flex flex-col gap-[0.85rem]'

export const compositeCreateFormRowClass = 'grid grid-cols-2 gap-3 max-[500px]:grid-cols-1'

export const compositeCreateFieldGroupClass =
  'flex flex-col gap-[0.3rem] [&_label]:font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] [&_label]:text-[0.78rem] [&_label]:font-semibold [&_label]:text-foreground-muted [&_input]:w-full [&_input]:rounded-lg [&_input]:border [&_input]:border-border [&_input]:bg-card-raised [&_input]:px-3 [&_input]:py-2 [&_input]:text-[0.85rem] [&_input]:leading-snug [&_input]:text-foreground [&_input]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)] [&_input]:transition-[border-color,box-shadow] [&_input]:duration-[160ms] [&_input:focus-visible]:border-[var(--control-focus-border)] [&_input:focus-visible]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08),0_0_0_3px_var(--control-focus-ring)] [&_input:focus-visible]:outline-none [&_textarea]:min-h-20 [&_textarea]:w-full [&_textarea]:resize-y [&_textarea]:rounded-lg [&_textarea]:border [&_textarea]:border-border [&_textarea]:bg-card-raised [&_textarea]:px-3 [&_textarea]:py-2 [&_textarea]:text-[0.85rem] [&_textarea]:leading-snug [&_textarea]:text-foreground [&_textarea]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)] [&_textarea]:transition-[border-color,box-shadow] [&_textarea]:duration-[160ms] [&_textarea:focus-visible]:border-[var(--control-focus-border)] [&_textarea:focus-visible]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08),0_0_0_3px_var(--control-focus-ring)] [&_textarea:focus-visible]:outline-none'

export const compositeCreateFormActionsClass =
  'mt-1 flex justify-end gap-2 border-t border-border pt-2'

export const crossJoinEditorClass = 'flex flex-col gap-4'

export const crossJoinEditorErrorClass =
  'rounded-md border border-red-500/25 bg-red-500/8 px-3 py-2 text-[0.84rem] text-red-600'

export const crossJoinGridClass =
  'grid grid-cols-2 gap-3 [&_label]:flex [&_label]:flex-col [&_label]:gap-1 [&_label]:text-[0.8rem] [&_label]:text-foreground-muted'

export const crossJoinActionsClass = 'flex justify-end gap-2 border-t border-border pt-2'
