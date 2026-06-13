import { cn } from './cn'

export const resultsTableScrollClass =
  'max-w-full overflow-x-auto [-webkit-overflow-scrolling:touch] mt-4 [&_.results-table]:mt-0'

export const resultsTableClass = 'w-full min-w-[42rem] mt-4 border-collapse text-[0.9rem]'

export const resultsTableDatasourcesListClass = cn(
  resultsTableClass,
  'results-table--datasources-list min-w-0 w-full table-fixed text-[0.875rem]',
  '[&_th]:px-4 [&_td]:px-4 [&_th]:py-3 [&_td]:py-[0.85rem]',
  '[&_thead_th]:bg-[var(--table-header-bg)] [&_thead_th]:text-[var(--table-header-fg)]',
  '[&_thead_th]:text-[0.68rem] [&_thead_th]:font-bold [&_thead_th]:uppercase [&_thead_th]:tracking-[0.08em]',
  '[&_thead_th]:border-b [&_thead_th]:border-border-strong [&_thead_th]:align-middle',
  '[&_tbody_td]:align-middle [&_tbody_td]:border-b [&_tbody_td]:border-border',
  '[&_tbody_tr:last-child_td]:border-b-0',
  '[&_thead_th.datasources-col-actions]:text-right',
  '[&_tbody_td.actions]:text-right',
  '[&_tbody_td.datasources-col-driver]:whitespace-nowrap',
  '[&_tbody_td.datasources-col-sync]:whitespace-nowrap text-[0.84rem] text-foreground-muted',
)

export const datasourceTableSectionLabelClass =
  'm-0 mb-3 text-[0.72rem] font-bold uppercase tracking-[0.08em] text-foreground-muted'

export const datasourceRowStatusClass = 'mt-1 text-[0.75rem] leading-snug text-foreground-muted'

export const datasourceConnectionHintClass = cn(
  'font-[family-name:var(--font-mono)] text-[0.75rem] leading-[1.4] text-foreground-muted',
  'break-all',
)

const resultsTableMetadataListBase = cn(
  resultsTableClass,
  'min-w-0 w-full mt-2 text-[0.8125rem] table-fixed',
  '[&_th]:p-[0.4rem_0.55rem] [&_td]:p-[0.4rem_0.55rem]',
  '[&_thead_th]:text-[0.86rem] [&_thead_th]:leading-[1.35] [&_thead_th]:pt-[0.6rem] [&_thead_th]:pb-[0.6rem]',
  '[&_thead_th]:text-left [&_thead_th]:align-middle',
  '[&_thead_th.metadata-col-name]:pl-[calc(0.55rem+0.7rem+0.35rem)]',
  '[&_thead_th.metadata-col-desc]:text-left',
  '[&_td.metadata-desc-cell]:text-left',
  '[&_thead_th.metadata-col-actions]:text-right',
  '[&_td.actions]:text-right',
  '[&_th.metadata-col-type]:align-middle [&_th.metadata-col-type]:text-foreground-muted',
  '[&_td.metadata-col-type]:align-middle [&_td.metadata-col-type]:text-foreground-muted [&_td.metadata-col-type]:text-[0.82rem]',
  '[&_td.metadata-col-type]:whitespace-normal [&_td.metadata-col-type]:break-words [&_td.metadata-col-type]:overflow-hidden',
  '[&_tbody_tr.metadata-table-row:hover>td]:bg-[color-mix(in_srgb,var(--accent)_4%,transparent)]',
  '[&_tbody_tr.metadata-table-row--expanded>td]:!bg-[color-mix(in_srgb,var(--accent)_8%,transparent)]',
  '[&_tbody_tr.metadata-table-row--expanded>td]:border-b-0',
  '[&_tbody_tr.metadata-table-row--expanded>td]:shadow-[inset_0_-1px_0_color-mix(in_srgb,var(--accent)_22%,transparent)]',
)

export const metadataTableColNameClass = 'w-[30%]'
export const metadataTableColTypeClass = 'w-[15%]'
export const metadataTableColDescClass = 'w-[41%]'
export const metadataTableColActionsClass = 'w-[14%]'

export const metadataNestedColNameClass = 'w-[30%]'
export const metadataNestedColTypeClass = 'w-[22%]'
export const metadataNestedColDescClass = 'w-[48%]'

export const metadataFilterEmptyRowClass = 'py-3 px-3 text-[0.85rem] text-foreground-muted'

export function resultsTableMetadataListClass(extra?: string): string {
  return cn(resultsTableMetadataListBase, extra)
}

export function resultsTableNestedClass(extra?: string): string {
  return cn(
    resultsTableMetadataListBase,
    'results-table--nested mt-0 mb-0 rounded-none',
    '[&_thead_th]:border-t-0 [&_thead_th]:align-middle',
    '[&_thead_th:first-child]:pl-[2.1rem]',
    '[&_tbody_td:first-child]:pl-[2.1rem] [&_tbody_td]:align-middle',
    '[&_thead_th]:text-[0.86rem] [&_thead_th]:leading-[1.35] [&_thead_th]:pt-[0.6rem] [&_thead_th]:pb-[0.6rem]',
    '[&_td.metadata-col-type]:text-[0.86rem] [&_td.metadata-col-type]:whitespace-normal [&_td.metadata-col-type]:break-words',
    '[&_tbody_td.metadata-desc-cell--editing]:align-top',
    '[&_tbody_tr:nth-child(odd)_td]:bg-[var(--metadata-nested-inner-stripe-odd)]',
    '[&_tbody_tr:nth-child(even)_td]:bg-[var(--metadata-nested-inner-stripe-even)]',
    '[&_tbody_tr:hover_td]:bg-[var(--table-stripe-hover)]',
    extra,
  )
}

export const metadataToolbarClass = 'flex flex-col gap-3 mb-4'

export const metadataToolbarTitleClass =
  'm-0 min-w-0 truncate text-[1.05rem] font-[650] leading-snug'

export const metadataToolbarTopRowClass =
  'grid grid-cols-1 gap-3 min-w-0 min-[981px]:grid-cols-[minmax(0,1fr)_auto] min-[981px]:items-center min-[981px]:gap-x-4'

export const metadataTableFiltersRowClass = cn(
  'flex flex-wrap gap-x-3 gap-y-2 items-center min-w-0',
  '[&_.metadata-filter-field]:flex-[0_1_11rem] [&_.metadata-filter-field]:min-w-[9rem] [&_.metadata-filter-field]:max-w-[11rem]',
)

export const metadataToolbarActionsClass = cn(
  'flex flex-wrap items-center justify-start gap-2 sm:gap-3 min-w-0 min-[981px]:justify-end',
  '[&_.metadata-toolbar-action-btn]:w-auto [&_.metadata-toolbar-action-btn]:mt-0',
  '[&_.metadata-toolbar-action-btn]:min-h-[1.85rem] [&_.metadata-toolbar-action-btn]:shrink-0',
)

export const metadataToolbarLangGroupClass = 'inline-flex items-center gap-2 shrink-0'

export const metadataLangTabsClass = cn(
  'inline-flex shrink-0 flex-none flex-row items-center border border-border-strong rounded-[0.4rem]',
  'bg-card-raised p-0.5 gap-0.5 h-[1.85rem] box-border',
)

export function metadataLangTabClass(active?: boolean): string {
  return cn(
    'appearance-none border-0 bg-transparent text-foreground-muted text-[0.76rem] font-semibold',
    'px-[0.65rem] rounded-[0.25rem] cursor-pointer h-full inline-flex items-center justify-center',
    'leading-none box-border transition-all duration-150',
    'hover:text-foreground',
    active && 'bg-card text-foreground shadow-[0_1px_2px_rgba(0,0,0,0.05)]',
  )
}

export const metadataHintBtnClass = cn(
  'appearance-none inline-flex shrink-0 flex-none items-center justify-center w-[1.85rem] h-[1.85rem] p-0',
  'border border-border rounded-full bg-card-raised text-foreground-muted',
  'text-[0.72rem] font-bold italic leading-none cursor-help',
  "font-[Georgia,'Times_New_Roman',serif]",
  'hover:text-foreground hover:border-border-strong',
)

export const metadataEmptyHintClass = 'text-foreground-muted text-[0.85rem] m-0 mb-2'

export const metadataTableFiltersClass = 'flex flex-wrap gap-x-[0.65rem] gap-y-2 items-center m-0'

export const metadataTableFiltersToolbarClass = cn(
  metadataTableFiltersClass,
  'flex-[1_1_16rem] justify-start items-center min-w-0',
  '[&_.metadata-filter-field]:flex-[0_1_11rem] [&_.metadata-filter-field]:min-w-[9rem] [&_.metadata-filter-field]:max-w-[11rem]',
)

export const metadataFilterFieldClass = cn('m-0 min-w-0', '[&_.ui-select]:w-full')

export function metadataTypeBadgeClass(isView?: boolean): string {
  return cn(
    'inline-block max-w-full box-border px-[0.35rem_0.55rem] py-[0.15rem] rounded-full border border-border',
    'bg-card-raised text-foreground-muted text-[0.66rem] font-semibold tracking-[0.04em]',
    'uppercase break-words text-center leading-snug',
    isView &&
      'border-[color-mix(in_srgb,var(--accent)_35%,transparent)] text-accent bg-[color-mix(in_srgb,var(--accent)_8%,transparent)]',
  )
}

export function metadataTableRowClass(expanded?: boolean): string {
  return cn('group metadata-table-row', expanded && 'metadata-table-row--expanded')
}

export const metadataRowActionClass = cn(
  'metadata-row-action inline-flex items-center gap-[0.3rem] px-[0.25rem_0.55rem] py-1',
  'border border-border rounded-[0.4rem] bg-transparent text-foreground-muted text-[0.74rem] cursor-pointer',
  'opacity-0 transition-[opacity,border-color,color] duration-[120ms]',
  'group-hover:opacity-100 group-focus-within:opacity-100',
  'group-[.metadata-table-row--expanded]:opacity-100',
  'hover:border-accent hover:text-foreground',
  'focus-visible:opacity-100 focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2',
  '[@media(hover:none)]:opacity-100',
)

export const metadataRowActionLabelClass = 'metadata-row-action__label'

export const metadataNestedCaptionClass = cn(
  'caption-top text-left py-2 px-[0.65rem] pt-2 pb-[0.4rem] m-0',
  'text-[0.86rem] font-[650] text-foreground tracking-[0.01em]',
  'border-b border-border-strong bg-[var(--metadata-nested-caption-bg)]',
)

export const metadataNestedRowClass = 'metadata-nested-row'

export const metadataNestedCellClass = cn(
  'metadata-nested-cell !p-[0.45rem_0.65rem_1rem] align-top',
  '!bg-[var(--metadata-nested-cell-bg)] border-b border-border-strong',
)

export const metadataNestedPanelClass = cn(
  'metadata-nested-panel rounded-lg border border-border-strong border-l-[3px] border-l-accent',
  'bg-[var(--metadata-nested-panel-bg)] shadow-[var(--metadata-nested-panel-shadow)] overflow-hidden',
  '[&_.results-table--nested]:mt-0 [&_.results-table--nested]:mb-0 [&_.results-table--nested]:rounded-none',
  '[&_.results-table--nested_thead_th]:border-t-0',
)

export const metadataColNameCellClass =
  'metadata-col-name-cell break-words [overflow-wrap:anywhere] text-foreground text-[0.86rem]'

export const metadataColNameBaseClass = 'metadata-col-name-base font-medium'

export function metadataColNameSuffixClass(multiline?: boolean): string {
  return cn(
    'metadata-col-name-suffix text-foreground-muted font-medium',
    multiline && 'block mt-[0.2rem] leading-[1.35]',
  )
}

export const metadataDescDisplayValueClass = 'text-foreground'

export const metadataDescDisplayPlaceholderClass = 'text-foreground-muted italic'

export function metadataDescCellClass(editing?: boolean): string {
  return cn(
    'metadata-desc-cell align-middle break-words [overflow-wrap:anywhere]',
    editing && 'metadata-desc-cell--editing align-top py-[0.35rem]',
  )
}

export const metadataInlineFieldClass = cn(
  'metadata-inline-field block box-border w-full min-w-0 min-h-[3.25rem] m-0 font-inherit',
  'px-[0.45rem_0.55rem] py-[0.45rem] border border-border-strong rounded-[0.4rem]',
  'bg-card-raised text-foreground text-[0.8125rem] leading-[1.45] resize-y',
  'shadow-[inset_0_1px_0_var(--control-surface-highlight)]',
  'transition-[border-color,box-shadow] duration-[120ms]',
  'hover:border-[var(--metadata-inline-hover-border)]',
  'focus:outline-none focus:border-[var(--control-focus-border)]',
  'focus:shadow-[inset_0_1px_0_var(--control-surface-highlight),0_0_0_2px_var(--control-focus-ring)]',
  'focus-visible:outline-none',
)

export const metadataInlineFieldFitRowsClass = cn(
  metadataInlineFieldClass,
  'metadata-inline-field--fit-rows min-h-0',
)

export const metadataDisplayExprClass = cn(
  'metadata-display-expr grid gap-[0.3rem] px-3 py-[0.65rem]',
  'border-b border-border-strong bg-[color-mix(in_srgb,var(--accent)_4%,transparent)]',
)

export const metadataDisplayExprLabelClass =
  'metadata-display-expr__label text-[0.74rem] font-bold text-foreground-muted'

export const metadataDisplayExprRowClass = 'metadata-display-expr__row flex items-center gap-2'

export const metadataDisplayExprInputClass = cn(
  'metadata-display-expr__input flex-1 min-w-[18rem]',
  'font-[family-name:var(--font-mono,monospace)] text-[0.78rem] px-[0.35rem_0.55rem] py-[0.35rem]',
  'border border-border rounded-[0.4rem] bg-card text-foreground',
  'focus:outline-none focus:border-accent',
)

export const metadataDisplayExprSavedClass =
  'metadata-display-expr__saved text-[0.74rem] text-success whitespace-nowrap'

export const metadataDisplayExprHintClass =
  'metadata-display-expr__hint text-[0.68rem] text-foreground-muted'

export const cellDrillableClass = cn(
  'cell-drillable cursor-pointer underline decoration-dotted decoration-foreground-muted underline-offset-2',
  'hover:text-foreground',
)

const LEGACY_TABLE_CLASS_MAP: Record<string, string> = {
  'metadata-toolbar': metadataToolbarClass,
  'metadata-toolbar__title': metadataToolbarTitleClass,
  'metadata-toolbar__actions': metadataToolbarActionsClass,
  'metadata-lang-tabs': metadataLangTabsClass,
  'metadata-lang-tab': metadataLangTabClass(),
  'metadata-hint-btn': metadataHintBtnClass,
  'metadata-empty-hint': metadataEmptyHintClass,
  'metadata-table-filters': metadataTableFiltersClass,
  'metadata-table-filters--toolbar': metadataTableFiltersToolbarClass,
  'metadata-filter-field': metadataFilterFieldClass,
  'metadata-type-badge': metadataTypeBadgeClass(),
  'metadata-type-badge--view': metadataTypeBadgeClass(true),
  'metadata-table-row': metadataTableRowClass(),
  'metadata-table-row--expanded': metadataTableRowClass(true),
  'metadata-row-action': metadataRowActionClass,
  'metadata-row-action__label': metadataRowActionLabelClass,
  'metadata-nested-caption': metadataNestedCaptionClass,
  'metadata-nested-row': metadataNestedRowClass,
  'metadata-nested-cell': metadataNestedCellClass,
  'metadata-nested-panel': metadataNestedPanelClass,
  'metadata-col-name-cell': metadataColNameCellClass,
  'metadata-col-name-base': metadataColNameBaseClass,
  'metadata-col-name-suffix': metadataColNameSuffixClass(),
  'metadata-col-name-suffix--multiline': metadataColNameSuffixClass(true),
  'metadata-desc-cell': metadataDescCellClass(),
  'metadata-desc-cell--editing': metadataDescCellClass(true),
  'metadata-inline-field': metadataInlineFieldClass,
  'metadata-inline-field--fit-rows': metadataInlineFieldFitRowsClass,
  'metadata-display-expr': metadataDisplayExprClass,
  'metadata-display-expr__label': metadataDisplayExprLabelClass,
  'metadata-display-expr__row': metadataDisplayExprRowClass,
  'metadata-display-expr__input': metadataDisplayExprInputClass,
  'metadata-display-expr__saved': metadataDisplayExprSavedClass,
  'metadata-display-expr__hint': metadataDisplayExprHintClass,
  'cell-drillable': cellDrillableClass,
}

const LEGACY_TABLE_CLASS_SKIP = new Set(['metadata-lang-tab--active'])

function resolveLegacyTableShell(set: Set<string>): { classes: string; consumed: Set<string> } {
  if (set.has('results-table--nested') && set.has('results-table--metadata-list')) {
    return {
      classes: resultsTableNestedClass(),
      consumed: new Set(['results-table', 'results-table--metadata-list', 'results-table--nested']),
    }
  }
  if (set.has('results-table--metadata-list')) {
    return {
      classes: resultsTableMetadataListClass(),
      consumed: new Set(['results-table', 'results-table--metadata-list']),
    }
  }
  if (set.has('results-table--datasources-list')) {
    return {
      classes: resultsTableDatasourcesListClass,
      consumed: new Set(['results-table', 'results-table--datasources-list']),
    }
  }
  if (set.has('results-table-scroll')) {
    return { classes: resultsTableScrollClass, consumed: new Set(['results-table-scroll']) }
  }
  if (set.has('results-table')) {
    return { classes: resultsTableClass, consumed: new Set(['results-table']) }
  }
  return { classes: '', consumed: new Set<string>() }
}

/** Maps legacy table/metadata class strings to Tailwind for gradual migration. */
export function legacyTableClass(className: string): string {
  const parts = className.split(/\s+/).filter(Boolean)
  const set = new Set(parts)
  const shell = resolveLegacyTableShell(set)
  const mapped = parts
    .filter((part) => !shell.consumed.has(part) && !LEGACY_TABLE_CLASS_SKIP.has(part))
    .map((part) => LEGACY_TABLE_CLASS_MAP[part] ?? part)

  return cn(shell.classes, ...mapped)
}
