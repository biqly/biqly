import { cn } from './cn'

export const modelingPageClass = 'flex flex-col gap-4 min-h-0'

export const modelingToolbarClass = cn(
  'flex flex-wrap items-end gap-x-5 gap-y-4 rounded-[10px] border border-border bg-card p-4 px-[1.15rem] shadow-[var(--shadow)]',
)

export const modelingToolbarGroupDatasourceClass =
  'min-w-[min(13rem,100%)] max-w-[18rem] flex-[1_1_13rem]'

export const modelingToolbarGroupModelClass =
  'min-w-[min(15rem,100%)] max-w-[28rem] flex-[2_1_15rem]'

export const modelingToolbarGroupActionsClass = 'ml-auto min-w-0 flex-[0_1_auto] max-[680px]:ml-0'

export const modelingToolbarModelRowClass = cn(
  'flex min-w-0 flex-wrap items-center gap-2',
  '[&>:first-child]:min-w-[10rem] [&>:first-child]:flex-1',
)

export const modelingToolbarActionsClass = cn(
  'flex w-full min-w-0 flex-wrap items-center justify-end gap-2 self-end max-[680px]:justify-start',
  '[&_button]:mt-0 [&_button]:min-h-[2.1rem] [&_button]:py-[0.35rem] [&_button]:px-4 [&_button]:whitespace-nowrap',
  '[&_button]:inline-flex [&_button]:items-center [&_button]:justify-center [&_button]:leading-none',
)

export function modelingShellClass(opts: { paletteOpen: boolean; editorOpen: boolean }): string {
  const { paletteOpen, editorOpen } = opts
  let cols = 'grid-cols-[minmax(18.5rem,20rem)_minmax(0,1fr)_minmax(21rem,22rem)]'
  if (!paletteOpen && !editorOpen) {
    cols = 'grid-cols-[2rem_minmax(0,1fr)_2rem]'
  } else if (!paletteOpen) {
    cols = 'grid-cols-[2rem_minmax(0,1fr)_minmax(21rem,22rem)]'
  } else if (!editorOpen) {
    cols = 'grid-cols-[minmax(18.5rem,20rem)_minmax(0,1fr)_2rem]'
  }
  return cn(
    'relative grid h-[calc(100vh-22rem)] min-h-[32rem] grid-rows-1 overflow-hidden',
    'rounded-lg border border-border bg-card shadow-[var(--shadow)]',
    'ease transition-[grid-template-columns] duration-180',
    cols,
    'max-[1180px]:h-[min(72vh,40rem)] max-[1180px]:min-h-[20rem] max-[1180px]:grid-cols-1',
  )
}

export const modelingMobileScrimClass = cn(
  'pointer-events-none fixed inset-0 z-30 opacity-0 transition-opacity duration-200',
  'max-[1180px]:pointer-events-auto max-[1180px]:bg-[var(--mobile-nav-scrim,rgba(15,23,42,0.58))]',
  'max-[1180px]:backdrop-blur-[var(--mobile-nav-scrim-blur,8px)]',
)

export function modelingMobileScrimVisibleClass(visible: boolean): string {
  return visible ? 'max-[1180px]:opacity-100' : ''
}

export function modelingMobileFabClass(side: 'left' | 'right', panelOpen: boolean): string {
  return cn(
    'fixed z-50 hidden items-center justify-center gap-1.5 max-[1180px]:inline-flex',
    'h-10 min-w-10 rounded-lg border border-border bg-card px-3 shadow-lg',
    'cursor-pointer text-sm font-semibold text-foreground',
    'transition-[transform,background] duration-150 hover:bg-card-raised active:scale-[0.98]',
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
    side === 'left' ? 'bottom-4 left-4' : 'right-4 bottom-4',
    panelOpen && 'max-[1180px]:!hidden',
  )
}

const modelingSidePanelMobileBase = cn(
  'max-[1180px]:fixed max-[1180px]:top-0 max-[1180px]:bottom-0 max-[1180px]:z-40',
  'max-[1180px]:w-[min(20rem,88vw)] max-[1180px]:shadow-[var(--mobile-nav-panel-shadow)]',
  'max-[1180px]:transition-transform max-[1180px]:duration-200 motion-reduce:max-[1180px]:transition-none',
)

const modelingSidePanelBase = cn(
  'relative z-2 flex min-h-0 min-w-0 flex-col gap-0 border-r border-border bg-card p-0',
  modelingSidePanelMobileBase,
)

export const modelingSideCollapsedClass = 'cursor-pointer [&_.modeling-side-body]:hidden'

export function modelingPaletteClass(open?: boolean): string {
  return cn(
    modelingSidePanelBase,
    '[&_h2]:mb-[0.35rem] [&_h2]:text-[1.05rem] [&_h2]:leading-[1.25]',
    '[&_p]:text-[0.86rem] [&_p]:text-foreground-muted',
    'max-[1180px]:left-0 max-[1180px]:border-r max-[1180px]:border-border',
    open
      ? 'max-[1180px]:translate-x-0'
      : 'max-[1180px]:pointer-events-none max-[1180px]:-translate-x-full',
    !open && modelingSideCollapsedClass,
  )
}

export function modelingEditorClass(open?: boolean): string {
  return cn(
    modelingSidePanelBase,
    'border-r-0 border-l border-border',
    '[&_h2]:mb-[0.35rem] [&_h2]:text-[1.05rem] [&_h2]:leading-[1.25]',
    '[&_p]:text-[0.86rem] [&_p]:text-foreground-muted',
    '[&_select]:w-full [&_select]:rounded-lg [&_select]:border [&_select]:border-border',
    '[&_select]:min-h-[2.55rem] [&_select]:bg-card-raised [&_select]:px-3 [&_select]:text-foreground',
    'max-[1180px]:right-0 max-[1180px]:left-auto max-[1180px]:w-[min(22rem,92vw)]',
    'max-[1180px]:border-r-0 max-[1180px]:border-l',
    open
      ? 'max-[1180px]:translate-x-0'
      : 'max-[1180px]:pointer-events-none max-[1180px]:translate-x-full',
    !open && modelingSideCollapsedClass,
  )
}

export const modelingSideBodyClass = 'flex flex-col gap-4 min-h-0 p-4 overflow-hidden'

export const modelingSideBodyMarkerClass = 'modeling-side-body'

export const modelingPaletteSideBodyClass = cn(
  modelingSideBodyClass,
  modelingSideBodyMarkerClass,
  'flex-1 max-[1180px]:overflow-y-auto',
)

export const modelingEditorSideBodyClass = cn(
  modelingSideBodyClass,
  modelingSideBodyMarkerClass,
  'flex-1 gap-3 overflow-y-auto [&_.form-group]:mb-0',
)

export function modelingSideToggleClass(side: 'left' | 'right'): string {
  return cn(
    'absolute top-[0.65rem] h-6 w-6 rounded-full border border-border bg-card-raised',
    'z-5 cursor-pointer text-[0.85rem] leading-none font-bold text-foreground-muted',
    'ease shadow-[0_2px_6px_rgba(0,0,0,0.12)] transition-[color,background] duration-120',
    'hover:bg-[color-mix(in_srgb,var(--accent)_12%,var(--bg-card-raised))] hover:text-accent',
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
    side === 'left' ? 'right-[-0.75rem]' : 'left-[-0.75rem]',
    'max-[1180px]:top-3',
    side === 'left' ? 'max-[1180px]:right-3 max-[1180px]:left-auto' : 'max-[1180px]:left-3',
  )
}

export const modelingKickerClass =
  'block mb-1 text-accent text-[0.72rem] font-extrabold tracking-[0.04em] uppercase'

// Vertical label shown on the 2rem collapsed side rail so the hidden panel
// stays discoverable; clicking it re-opens the panel.
export const modelingRailLabelClass = cn(
  'absolute inset-x-0 top-12 mx-auto flex cursor-pointer items-center justify-center',
  'border-0 bg-transparent p-0 text-[0.68rem] font-bold tracking-[0.08em] uppercase',
  'text-foreground-muted [writing-mode:vertical-rl] hover:text-accent',
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
  'max-[1180px]:hidden',
)

export function modelingStatusPillClass(published?: boolean): string {
  return cn(
    'inline-flex shrink-0 items-center gap-[0.3rem] min-h-[2.1rem] rounded-full px-[0.55rem_0.7rem]',
    'border border-border bg-card-raised text-[0.72rem] font-semibold whitespace-nowrap text-foreground-muted',
    'before:h-[0.45rem] before:w-[0.45rem] before:rounded-full before:bg-warning before:content-[""]',
    published &&
      'border-[color-mix(in_srgb,var(--success)_35%,transparent)] text-success before:bg-success',
  )
}

export const modelingMenuModelNameClass =
  'block overflow-hidden text-ellipsis whitespace-nowrap text-foreground'

export const modelingJoinListClass =
  'flex min-h-0 flex-1 flex-col gap-3 overflow-auto pr-1 [&_h3]:text-[0.85rem] [&_h3]:mb-1'

const modelingJoinPillBase = cn(
  'join-pill flex flex-col gap-1 rounded-lg border border-border bg-card-raised p-3',
  '[&_strong]:text-[0.78rem] [&_strong]:leading-snug [&_strong]:break-all',
  '[&_span]:text-[0.72rem] [&_span]:leading-snug [&_span]:break-all [&_span]:text-foreground-muted',
)

export function modelingJoinPillClass(opts?: { active?: boolean; suggested?: boolean }): string {
  return cn(
    modelingJoinPillBase,
    opts?.active &&
      'border-accent bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] shadow-[0_0_0_1px_var(--accent)]',
    opts?.suggested &&
      'border-[color-mix(in_srgb,var(--accent)_40%,var(--border))] bg-[color-mix(in_srgb,var(--accent)_4%,var(--bg-card-raised))]',
  )
}

export function modelingTabCountClass(zero?: boolean, active?: boolean): string {
  return cn(
    'modeling-tab__count inline-flex min-w-[1.15rem] shrink-0 items-center justify-center',
    'rounded-full px-[0.35rem] py-[0.05rem] text-[0.68rem] leading-none font-bold tabular-nums',
    active
      ? 'bg-accent text-white'
      : zero
        ? 'bg-card-raised text-foreground-muted'
        : 'bg-[color-mix(in_srgb,var(--accent)_14%,transparent)] text-accent',
  )
}

export const modelingTabsClass = 'flex border border-border rounded-lg overflow-hidden'

export function modelingTabClass(active?: boolean): string {
  return cn(
    'flex min-w-0 flex-1 items-center justify-center gap-1 border-0 bg-card-raised text-foreground-muted',
    'cursor-pointer px-1.5 py-[0.55rem] text-[0.78rem] font-bold overflow-hidden',
    'transition-[background,color] duration-150 not-last:border-r not-last:border-border',
    'hover:bg-[color-mix(in_srgb,var(--accent)_12%,var(--bg-card-raised))]',
    active
      ? 'bg-[color-mix(in_srgb,var(--accent)_18%,var(--bg-card-raised))] text-accent'
      : 'bg-card-raised text-foreground-muted',
  )
}

export const modelingTabContentClass = 'flex flex-col flex-1 min-h-0 overflow-hidden'

export const modelingEmptyClass = 'text-foreground-muted text-[0.82rem] my-2 mx-0'

export const modelingJoinPillHeaderClass = 'flex items-center justify-between gap-[0.4rem]'

export const modelingJoinMetaClass = 'text-foreground-muted text-[0.68rem]'

export const modelingSubgroupTitleClass = 'flex items-baseline gap-[0.35rem] my-[0.8rem_0_0.35rem]'

export const modelingSubgroupMetaClass = 'text-foreground-muted text-[0.7rem] font-medium'

export function modelingGroupClass(open?: boolean): string {
  return cn(
    'mb-[0.4rem] flex-[0_0_auto] overflow-hidden rounded-lg border border-border bg-card',
    open && '[&_.modeling-group-header]:bg-[var(--surface-hover)]',
  )
}

export const modelingGroupHeaderClass = cn(
  'font-inherit flex w-full cursor-pointer items-center gap-[0.4rem] border-0 bg-transparent px-[0.6rem] py-[0.45rem] text-left text-inherit',
  'hover:bg-[var(--surface-hover)]',
)

export const modelingGroupChevronClass = 'shrink-0 w-[0.9rem] text-foreground-muted text-[0.7rem]'

export const modelingGroupTitleClass =
  'flex-[1_1_auto] font-semibold text-[0.82rem] whitespace-nowrap overflow-hidden text-ellipsis'

export const modelingGroupMetaClass =
  'flex-[0_1_auto] text-foreground-muted text-[0.66rem] whitespace-nowrap overflow-hidden text-ellipsis'

export const modelingGroupCountClass =
  'shrink-0 min-w-[1.4rem] py-[0.05rem] px-[0.4rem] rounded-full bg-[var(--accent-soft)] text-accent text-[0.68rem] font-semibold text-center'

export const modelingGroupBodyClass = cn(
  'flex flex-col gap-[0.3rem] px-2 py-[0.3rem] pb-2',
  '[&_.join-pill]:m-0 [&_.join-pill]:p-[0.4rem_0.55rem]',
  '[&_.join-pill_span]:text-[0.68rem] [&_.join-pill_strong]:text-[0.8rem]',
)

export const modelingDeleteBtnClass = cn(
  'm-0 inline-flex h-[1.6rem] w-[1.6rem] min-w-[1.6rem] shrink-0 items-center justify-center p-0',
  'rounded-md border border-[color-mix(in_srgb,#e53e3e_40%,var(--border))]',
  'bg-[color-mix(in_srgb,#e53e3e_8%,var(--bg-card-raised))] text-[#e53e3e]',
  'cursor-pointer text-[0.82rem] leading-none font-bold transition-[background,border-color] duration-150',
  'hover:border-[#e53e3e] hover:bg-[color-mix(in_srgb,#e53e3e_18%,var(--bg-card-raised))]',
)

export const modelingAddBtnClass = cn(
  'm-0 inline-flex h-[1.6rem] w-[1.6rem] min-w-[1.6rem] shrink-0 items-center justify-center p-0',
  'rounded-md border border-[color-mix(in_srgb,var(--accent)_40%,var(--border))]',
  'bg-[color-mix(in_srgb,var(--accent)_8%,var(--bg-card-raised))] text-accent',
  'cursor-pointer text-[0.82rem] leading-none font-bold transition-[background,border-color] duration-150',
  'hover:border-accent hover:bg-[color-mix(in_srgb,var(--accent)_18%,var(--bg-card-raised))]',
)

export const modelingRenameBtnClass = cn(
  'm-0 inline-flex h-[1.6rem] w-[1.6rem] min-w-[1.6rem] shrink-0 items-center justify-center p-0',
  'cursor-pointer rounded-md border border-border bg-card-raised text-[0.72rem] leading-none text-foreground-muted',
  'transition-[background,border-color,color] duration-150',
  'hover:border-accent hover:bg-[color-mix(in_srgb,var(--accent)_10%,var(--bg-card-raised))] hover:text-accent',
)

export const modelingPillActionsClass = 'inline-flex items-center gap-1 shrink-0'

export const modelingSectionHeaderClass =
  'flex items-center justify-between gap-2 my-3 mx-0 mb-2 [&_h3]:m-0'

export const modelingSectionAddBtnClass = 'w-auto m-0'

export const modelingBaseBadgeClass = 'text-accent ml-1'

export const modelingCanvasWrapClass = 'modeling-canvas-wrap'

export function modelingJoinLineClass(hi?: boolean): string {
  return cn('modeling-join-line', hi && 'modeling-join-line--hi')
}

// Invisible wide stroke laid over each join line so the thin visible path is
// easy to hover; it opts back into pointer events while the container <svg>
// stays pointer-events-none so empty areas pass through to canvas pan/drag.
export const modelingJoinHitClass = 'pointer-events-[stroke] fill-none stroke-transparent'

export const modelingJoinTooltipClass = cn(
  'pointer-events-none fixed z-50 flex max-w-[18rem] flex-col gap-1 rounded-lg border border-border',
  'bg-card-raised px-3 py-2 text-[0.72rem] text-foreground shadow-[var(--shadow)]',
)

export const modelingJoinTooltipTitleClass = 'font-semibold text-foreground'

export const modelingJoinTooltipRowClass = 'flex gap-1.5'

export const modelingJoinTooltipLabelClass = 'shrink-0 text-foreground-muted'

export const modelingJoinTooltipValueClass = 'min-w-0 wrap-anywhere text-foreground'

export const modelingZoomControlsClass = cn(
  'absolute top-3 right-3 z-5 flex items-center gap-1 p-[0.3rem]',
  'rounded-lg border border-border bg-card-raised shadow-[0_4px_14px_rgba(0,0,0,0.12)]',
  '[&_button]:h-[1.85rem] [&_button]:w-[1.85rem] [&_button]:rounded-md [&_button]:border-0',
  '[&_button]:cursor-pointer [&_button]:bg-transparent [&_button]:text-[0.9rem] [&_button]:font-bold [&_button]:text-foreground',
  '[&_button:hover]:bg-[color-mix(in_srgb,var(--accent)_14%,transparent)] [&_button:hover]:text-accent',
)

export const modelingZoomReadoutClass =
  'px-2 text-foreground-muted text-[0.72rem] tabular-nums min-w-[2.5rem] text-center'

export const modelingCanvasClass = 'absolute top-0 left-0'

export const modelingLinesClass = 'absolute inset-0 overflow-visible pointer-events-none'

export function modelingTableCardClass(opts?: { base?: boolean; hi?: boolean }): string {
  return cn(
    'modeling-table-card absolute overflow-hidden rounded-[10px] border border-border-strong bg-card-raised',
    'ease shadow-[0_12px_30px_rgba(0,0,0,0.18)] transition-[box-shadow,transform,border-color] duration-120',
    'hover:border-[color-mix(in_srgb,var(--accent)_50%,var(--border-strong))] hover:shadow-[0_18px_38px_rgba(0,0,0,0.22)]',
    opts?.base &&
      'border-[color-mix(in_srgb,var(--accent)_68%,var(--border-strong))] shadow-[0_16px_34px_color-mix(in_srgb,var(--accent)_18%,transparent)]',
    opts?.hi &&
      'border-accent shadow-[0_18px_38px_color-mix(in_srgb,var(--accent)_30%,transparent)]',
    '[&_header]:flex [&_header]:flex-col [&_header]:gap-[0.1rem] [&_header]:border-b [&_header]:border-border',
    '[&_header]:cursor-grab [&_header]:bg-accent [&_header]:p-[0.65rem_0.8rem] [&_header]:text-white [&_header]:select-none',
    '[&_header:active]:cursor-grabbing',
    '[&_header:focus-visible]:outline-2 [&_header:focus-visible]:outline-offset-2 [&_header:focus-visible]:outline-[var(--control-focus-border,#5b8eff)]',
    '[&_header_span]:text-[0.68rem] [&_header_span]:font-bold [&_header_span]:opacity-[0.78]',
    '[&_header_strong]:overflow-hidden [&_header_strong]:text-[0.88rem] [&_header_strong]:text-ellipsis [&_header_strong]:whitespace-nowrap',
    '[&_ul]:list-none [&_ul]:px-0 [&_ul]:py-[0.35rem]',
    '[&_li]:flex [&_li]:min-h-[1.35rem] [&_li]:items-center [&_li]:justify-between [&_li]:gap-[0.6rem]',
    '[&_li]:px-3 [&_li]:py-[0.22rem] [&_li]:text-[0.74rem] [&_li]:text-foreground-muted',
    '[&_li+li]:border-t [&_li+li]:border-[color-mix(in_srgb,var(--border)_70%,transparent)]',
    '[&_small]:shrink-0 [&_small]:text-[0.68rem] [&_small]:text-foreground-muted',
  )
}

export function modelingTableRowClass(opts?: {
  joined?: boolean
  active?: boolean
  more?: boolean
}): string {
  return cn(
    opts?.joined && 'bg-[color-mix(in_srgb,var(--accent)_10%,transparent)] text-foreground',
    opts?.active && 'bg-[color-mix(in_srgb,var(--accent)_22%,transparent)] font-bold text-accent',
    opts?.more && 'justify-center text-[0.7rem] text-foreground-muted italic',
  )
}

export const modelingColumnNameClass = cn(
  'flex min-w-0 items-center gap-[0.28rem] overflow-hidden text-ellipsis whitespace-nowrap',
  '[&_b]:shrink-0 [&_b]:rounded [&_b]:bg-[color-mix(in_srgb,var(--accent)_16%,transparent)]',
  '[&_b]:px-[0.2rem] [&_b]:py-[0.05rem] [&_b]:text-[0.58rem] [&_b]:text-accent',
)

export const modelingTypeIconClass = cn(
  'inline-flex h-[1.05rem] min-w-[1.5rem] shrink-0 items-center justify-center rounded',
  'bg-[color-mix(in_srgb,var(--foreground)_8%,transparent)] px-[0.25rem]',
  'text-[0.58rem] font-bold text-foreground-muted tabular-nums',
)

export const modelingCardSectionClass = cn(
  'flex items-center justify-between border-t border-border px-3 py-[0.28rem]',
  'text-[0.66rem] font-semibold tracking-wide text-foreground-muted uppercase',
)

export const modelingCardSectionAddClass = cn(
  'flex h-[1.15rem] w-[1.15rem] cursor-pointer items-center justify-center rounded border-0',
  'bg-transparent text-[0.85rem] leading-none text-foreground-muted',
  'hover:bg-[color-mix(in_srgb,var(--accent)_14%,transparent)] hover:text-accent',
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
)

export const modelingRelRowClass =
  'flex items-center gap-[0.4rem] px-3 py-[0.18rem] text-[0.72rem] text-foreground-muted'

export const modelingKebabClass = cn(
  'absolute top-2 right-2 flex h-[1.3rem] w-[1.3rem] cursor-pointer items-center justify-center rounded border-0',
  'bg-transparent text-[0.95rem] leading-none text-white/80',
  'hover:bg-white/15 hover:text-white',
  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white',
)

export const modelingCardHeaderRowClass = 'flex items-center gap-[0.4rem] pr-6'

// Fixed-position column-visibility dropdown, anchored to a card's kebab button.
// Rendered outside the transformed/overflow-hidden canvas so it isn't clipped
// or scaled.
export const modelingColumnsMenuClass = cn(
  'fixed z-50 flex max-h-[18rem] w-[15rem] flex-col overflow-hidden rounded-lg border border-border',
  'bg-card-raised text-foreground shadow-[var(--shadow)]',
)

export const modelingColumnsMenuTitleClass = cn(
  'shrink-0 border-b border-border px-3 py-2 text-[0.72rem] font-semibold',
  'tracking-wide text-foreground-muted uppercase',
)

export const modelingColumnsMenuListClass = 'flex min-h-0 flex-col overflow-y-auto py-1'

export const modelingColumnsMenuItemClass = cn(
  'flex cursor-pointer items-center gap-2 px-3 py-[0.3rem] text-[0.78rem] text-foreground',
  'hover:bg-[color-mix(in_srgb,var(--accent)_10%,transparent)]',
  '[&_input]:h-[0.85rem] [&_input]:w-[0.85rem] [&_input]:shrink-0 [&_input]:accent-[var(--accent)]',
  '[&_span]:min-w-0 [&_span]:overflow-hidden [&_span]:text-ellipsis [&_span]:whitespace-nowrap',
)

export const modelingDetailRootClass = 'flex min-w-0 flex-col gap-4 text-sm'
export const modelingDetailHeaderClass = 'flex items-start justify-between gap-3'
export const modelingDetailAliasClass = 'flex flex-col gap-1'
export const modelingDetailMutedClass = 'text-xs text-foreground-muted'
export const modelingDetailDescriptionClass = 'text-foreground-muted'
export const modelingDetailTitleClass = 'mb-2 font-semibold'
export const modelingDetailFieldNameClass = 'font-medium'
export const modelingDetailPreviewHeaderClass = 'mb-2 flex items-center gap-2'
export const modelingDetailTableWrapClass =
  'max-h-[24rem] max-w-full overflow-auto rounded-md border border-border'
export const modelingDetailTableClass = 'min-w-full text-xs'
export const modelingDetailTableHeaderClass =
  'sticky top-0 border-b border-border bg-card px-2 py-1 text-left font-semibold whitespace-nowrap'
export const modelingDetailTableCellClass = 'border-b border-border/50 px-2 py-1 whitespace-nowrap'

// Info tables (columns / relationships) in the table-detail modal. Unlike the
// data-preview table, description cells must WRAP rather than force horizontal
// scroll, so cells use align-top + wrap-anywhere.
export const modelingDetailInfoTableWrapClass =
  'max-h-[22rem] overflow-y-auto rounded-md border border-border'
export const modelingDetailInfoTableClass = 'w-full text-xs'
export const modelingDetailInfoCellClass =
  'border-b border-border/50 px-2.5 py-1.5 align-top wrap-anywhere'
export const modelingDetailInfoNameCellClass =
  'border-b border-border/50 px-2.5 py-1.5 align-top font-medium whitespace-nowrap'
export const modelingDetailInfoTypeCellClass =
  'border-b border-border/50 px-2.5 py-1.5 align-top whitespace-nowrap text-foreground-muted'

export const modelingTypeHintClass = 'block text-foreground-muted text-[0.72rem] leading-[1.35]'

export const modelingEditorGridClass = cn('grid grid-cols-2 gap-[0.7rem] max-[760px]:grid-cols-1')

export const modelingSchemaTagListClass = 'flex flex-wrap gap-[0.4rem] mt-[0.35rem] mb-4'

export function modelingSchemaTagClass(active?: boolean): string {
  return cn(
    'inline-flex items-center gap-[0.35rem] rounded-full px-[0.6rem] py-1 text-[0.74rem] font-semibold',
    'ease border border-border bg-card-raised text-foreground-muted transition-all duration-120 select-none',
    active &&
      'border-accent bg-[color-mix(in_srgb,var(--accent)_10%,var(--bg-card-raised))] text-foreground shadow-[0_0_0_1px_var(--accent)]',
    !active &&
      '[&_.modeling-schema-tag__name]:text-foreground-muted [&_.modeling-schema-tag__name]:line-through [&_.modeling-schema-tag__name]:opacity-60',
  )
}

export const modelingSchemaTagNameClass = cn(
  'modeling-schema-tag__name max-w-[8rem] overflow-hidden text-ellipsis whitespace-nowrap',
)

export const modelingSchemaTagToggleClass = cn(
  'cursor-pointer border-0 bg-transparent p-0 text-[1.1rem] leading-none text-inherit',
  'inline-flex h-[1.05rem] w-[1.05rem] items-center justify-center rounded-full',
  'ease transition-[background-color,color] duration-100',
  'hover:bg-white/10 hover:text-foreground',
)

const LEGACY_MODELING_CLASS_MAP: Record<string, string> = {
  'modeling-page': modelingPageClass,
  'modeling-toolbar': modelingToolbarClass,
  'modeling-toolbar__model-row': modelingToolbarModelRowClass,
  'modeling-toolbar-actions': modelingToolbarActionsClass,
  'modeling-side-body': modelingSideBodyClass,
  'modeling-kicker': modelingKickerClass,
  'modeling-menu-model-name': modelingMenuModelNameClass,
  'modeling-join-list': modelingJoinListClass,
  'modeling-tabs': modelingTabsClass,
  'modeling-tab-content': modelingTabContentClass,
  'modeling-empty': modelingEmptyClass,
  'modeling-join-pill-header': modelingJoinPillHeaderClass,
  'modeling-join-meta': modelingJoinMetaClass,
  'modeling-subgroup-title': modelingSubgroupTitleClass,
  'modeling-subgroup-meta': modelingSubgroupMetaClass,
  'modeling-group-header': modelingGroupHeaderClass,
  'modeling-group-chevron': modelingGroupChevronClass,
  'modeling-group-title': modelingGroupTitleClass,
  'modeling-group-meta': modelingGroupMetaClass,
  'modeling-group-count': modelingGroupCountClass,
  'modeling-group-body': modelingGroupBodyClass,
  'modeling-delete-btn': modelingDeleteBtnClass,
  'modeling-add-btn': modelingAddBtnClass,
  'modeling-rename-btn': modelingRenameBtnClass,
  'modeling-pill-actions': modelingPillActionsClass,
  'modeling-section-header': modelingSectionHeaderClass,
  'modeling-section-add-btn': modelingSectionAddBtnClass,
  'modeling-base-badge': modelingBaseBadgeClass,
  'modeling-zoom-controls': modelingZoomControlsClass,
  'modeling-zoom-readout': modelingZoomReadoutClass,
  'modeling-canvas': modelingCanvasClass,
  'modeling-lines': modelingLinesClass,
  'modeling-column-name': modelingColumnNameClass,
  'modeling-type-hint': modelingTypeHintClass,
  'modeling-editor-grid': modelingEditorGridClass,
  'modeling-schema-tag-list': modelingSchemaTagListClass,
  'modeling-schema-tag__name': modelingSchemaTagNameClass,
  'modeling-schema-tag__toggle': modelingSchemaTagToggleClass,
  'modeling-side-toggle--left': modelingSideToggleClass('left'),
  'modeling-side-toggle--right': modelingSideToggleClass('right'),
  'modeling-tab__count': modelingTabCountClass(),
  'modeling-tab__count--zero': modelingTabCountClass(true),
  'modeling-join-pill': modelingJoinPillClass(),
  'modeling-join-pill--active': modelingJoinPillClass({ active: true }),
  'modeling-join-pill--suggested': modelingJoinPillClass({ suggested: true }),
  'modeling-tab': modelingTabClass(),
  'modeling-tab--active': modelingTabClass(true),
  'modeling-group': modelingGroupClass(),
  'modeling-group--open': modelingGroupClass(true),
  'modeling-schema-tag': modelingSchemaTagClass(),
  'modeling-schema-tag--active': modelingSchemaTagClass(true),
  'modeling-table-card': modelingTableCardClass(),
  'modeling-table-card--base': modelingTableCardClass({ base: true }),
  'modeling-table-card--hi': modelingTableCardClass({ hi: true }),
  'modeling-row--joined': modelingTableRowClass({ joined: true }),
  'modeling-row--active': modelingTableRowClass({ active: true }),
  'modeling-row--more': modelingTableRowClass({ more: true }),
  'modeling-status-pill': modelingStatusPillClass(),
  'modeling-status-pill--published': modelingStatusPillClass(true),
  'modeling-palette': modelingPaletteClass(true),
  'modeling-editor': modelingEditorClass(true),
  'modeling-side-toggle': modelingSideToggleClass('left'),
}

const LEGACY_MODELING_CLASS_SKIP = new Set([
  'modeling-canvas-wrap',
  'modeling-join-line',
  'modeling-join-line--hi',
  'modeling-side--collapsed',
])

function resolveLegacyModelingShell(set: Set<string>): { classes: string; consumed: Set<string> } {
  if (set.has('modeling-table-card')) {
    return {
      classes: modelingTableCardClass({
        base: set.has('modeling-table-card--base'),
        hi: set.has('modeling-table-card--hi'),
      }),
      consumed: new Set(
        ['modeling-table-card', 'modeling-table-card--base', 'modeling-table-card--hi'].filter(
          (p) => p === 'modeling-table-card' || set.has(p),
        ),
      ),
    }
  }
  if (set.has('modeling-join-pill')) {
    return {
      classes: modelingJoinPillClass({
        active: set.has('modeling-join-pill--active'),
        suggested: set.has('modeling-join-pill--suggested'),
      }),
      consumed: new Set(
        [
          'modeling-join-pill',
          'modeling-join-pill--active',
          'modeling-join-pill--suggested',
        ].filter((p) => p === 'modeling-join-pill' || set.has(p)),
      ),
    }
  }
  if (set.has('modeling-schema-tag')) {
    return {
      classes: modelingSchemaTagClass(set.has('modeling-schema-tag--active')),
      consumed: new Set(
        ['modeling-schema-tag', 'modeling-schema-tag--active'].filter(
          (p) => p === 'modeling-schema-tag' || set.has(p),
        ),
      ),
    }
  }
  if (set.has('modeling-group')) {
    return {
      classes: modelingGroupClass(set.has('modeling-group--open')),
      consumed: new Set(
        ['modeling-group', 'modeling-group--open'].filter(
          (p) => p === 'modeling-group' || set.has(p),
        ),
      ),
    }
  }
  if (set.has('modeling-tab')) {
    return {
      classes: modelingTabClass(set.has('modeling-tab--active')),
      consumed: new Set(
        ['modeling-tab', 'modeling-tab--active'].filter((p) => p === 'modeling-tab' || set.has(p)),
      ),
    }
  }
  if (set.has('modeling-status-pill')) {
    return {
      classes: modelingStatusPillClass(set.has('modeling-status-pill--published')),
      consumed: new Set(
        ['modeling-status-pill', 'modeling-status-pill--published'].filter(
          (p) => p === 'modeling-status-pill' || set.has(p),
        ),
      ),
    }
  }
  return { classes: '', consumed: new Set<string>() }
}

/** Maps legacy modeling class strings to Tailwind for gradual migration. */
export function legacyModelingClass(className: string): string {
  const parts = className.split(/\s+/).filter(Boolean)
  const set = new Set(parts)
  const shell = resolveLegacyModelingShell(set)
  const skipped = parts.filter((part) => LEGACY_MODELING_CLASS_SKIP.has(part))
  const mapped = parts
    .filter((part) => !shell.consumed.has(part) && !LEGACY_MODELING_CLASS_SKIP.has(part))
    .map((part) => LEGACY_MODELING_CLASS_MAP[part] ?? part)

  return cn(shell.classes, ...mapped, ...skipped)
}
