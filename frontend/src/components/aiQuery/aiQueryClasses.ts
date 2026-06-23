export const aiQueryLayoutClass =
  'flex gap-6 items-stretch h-[calc(100vh-13.5rem)] min-h-[500px] max-[900px]:flex-col max-[900px]:h-auto max-[900px]:min-h-0'

export const conversationSidebarClass =
  'w-[300px] max-[900px]:w-full max-[900px]:max-h-[30vh] shrink-0 bg-card border border-border rounded-2xl p-5 flex flex-col h-full max-[900px]:h-auto shadow-card backdrop-blur-md'

export const sidebarHeaderClass =
  'flex justify-between items-center mb-4 pb-3 border-b border-border gap-2'

export const sidebarHeaderTitleClass =
  'text-[0.85rem] font-bold text-foreground-muted uppercase tracking-[0.05em] m-0 min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap'

export const conversationsListClass = 'flex-1 overflow-y-auto flex flex-col gap-2 pr-1'

export const conversationItemBaseClass =
  'group relative flex items-center justify-between w-full shrink-0 text-left bg-card-raised border border-border rounded-[0.6rem] text-foreground transition-all duration-200 overflow-hidden hover:bg-[var(--control-hover-bg)] hover:border-[var(--control-hover-border)]'

export const conversationItemActiveClass =
  'border-accent! bg-[#5b8eff]/8! shadow-[inset_3px_0_0_var(--accent)]'

export function conversationItemClass(active: boolean): string {
  return active
    ? `${conversationItemBaseClass} ${conversationItemActiveClass}`
    : conversationItemBaseClass
}

export const convItemContentClass = 'flex-1 min-w-0 flex flex-col gap-[0.15rem] pr-6'

export const convTitleClass =
  'block text-[0.85rem] font-medium text-foreground overflow-hidden text-ellipsis whitespace-nowrap'

export const convTimeClass = 'block text-[0.72rem] text-foreground-faint'

export const convActionsClass =
  'absolute right-[0.4rem] top-1/2 -translate-y-1/2 flex gap-[0.15rem] items-center opacity-0 pointer-events-none group-hover:opacity-100 group-hover:pointer-events-auto group-focus-within:opacity-100 group-focus-within:pointer-events-auto transition-opacity duration-150 bg-card border border-border rounded-[0.45rem] p-[0.1rem_0.2rem] shadow-card-sm'

export const btnConvActionClass =
  'bg-transparent border-0 cursor-pointer p-1 text-[0.85rem] rounded-[0.3rem] flex items-center justify-center opacity-60 transition-[opacity,background-color] duration-150 hover:opacity-100 hover:bg-[var(--border-strong)]'

export const convEditInputClass =
  'w-full bg-canvas border border-accent rounded-[0.35rem] text-foreground text-[0.8rem] p-[0.3rem_0.5rem] outline-none shadow-[0_0_0_2px_rgba(91,142,255,0.15)]'

export const chatFeedClass =
  'flex-1 overflow-y-auto flex flex-col gap-5 p-5 bg-canvas-subtle border border-border rounded-2xl shadow-[inset_0_2px_6px_rgba(0,0,0,0.08)] [data-theme=dark]:shadow-[inset_0_2px_8px_rgba(0,0,0,0.35)] min-h-0 max-h-none max-[900px]:max-h-[45vh] overscroll-contain'

export const chatMsgClass = 'flex w-full animate-bubble-appear'

export const chatMsgUserClass = 'justify-start'

export const chatMsgAssistantClass = 'items-start flex-row-reverse gap-[0.65rem]'

export const chatMsgAvatarClass =
  'shrink-0 inline-flex items-center justify-center w-8 h-8 rounded-full bg-gradient-to-br from-accent to-accent-strong text-white text-[0.85rem] shadow-[0_2px_8px_var(--accent-glow)] mt-[0.1rem]'

export const chatMsgMainClass = 'flex-1 min-w-0 flex flex-col gap-[0.35rem]'

export const chatMsgMetaClass = 'flex items-baseline gap-2 text-[0.72rem] text-foreground-muted'

export const chatMsgMetaAssistantClass = 'justify-end'

export const chatMsgAuthorClass = 'font-semibold text-foreground'

export const chatTypingClass =
  'inline-flex items-center gap-[0.6rem] self-end bg-card border border-border rounded-[1rem_0.35rem_1rem_1rem] p-[0.65rem_1rem] shadow-card'

export const chatTypingHintClass = 'text-right'

export const chatTypingDotsClass = 'inline-flex gap-[0.25rem]'

export const chatTypingDot1Class =
  'w-[0.4rem] h-[0.4rem] rounded-full bg-accent opacity-40 animate-chat-typing-dot'

export const chatTypingDot2Class =
  'w-[0.4rem] h-[0.4rem] rounded-full bg-accent opacity-40 animate-chat-typing-dot [animation-delay:0.18s]'

export const chatTypingDot3Class =
  'w-[0.4rem] h-[0.4rem] rounded-full bg-accent opacity-40 animate-chat-typing-dot [animation-delay:0.36s]'

export const chatTypingLabelClass = 'text-[0.85rem] text-foreground'

export const chatTypingElapsedClass =
  'text-[0.78rem] text-foreground-muted [font-variant-numeric:tabular-nums]'

export const chatTypingHintLabelClass = 'm-0 text-[0.72rem] text-foreground-faint'

export const chatBubbleClass = 'flex flex-col max-w-[85%]'

export const userBubbleClass =
  'self-start bg-gradient-to-br from-accent to-accent-strong text-white rounded-[1rem_1rem_1rem_0.15rem] p-[0.85rem_1.25rem] shadow-[0_4px_14px_var(--accent-glow)]'

export const userBubbleContentClass =
  'text-[0.92rem] leading-normal font-medium whitespace-pre-wrap'

export const userBubbleTimeClass = 'text-[0.68rem] opacity-85 self-start mt-[0.35rem]'

export const assistantCardClass =
  'bg-card border border-border rounded-[1rem_0.35rem_1rem_1rem] p-5 shadow-card flex flex-col gap-4 w-full transition-all duration-200 backdrop-blur-md hover:border-border-strong'

export const assistantCardTopClass = 'flex items-center justify-between gap-3 flex-wrap'

export const assistantSummaryClass = 'flex items-center gap-[0.6rem] flex-wrap min-w-0'

const confidenceBaseClass =
  'inline-flex items-center h-7 px-[0.6rem] rounded-full text-[0.76rem] font-semibold whitespace-nowrap leading-none box-border'

export function assistantConfidenceClass(level: string): string {
  if (level === 'high') {
    return `${confidenceBaseClass} text-success bg-success/10`
  }
  if (level === 'mid') {
    return `${confidenceBaseClass} text-warning bg-warning/10`
  }
  return `${confidenceBaseClass} text-error bg-error/10`
}

export const assistantCardDetailsToggleClass =
  'inline-flex items-center gap-[0.4rem] shrink-0 ml-auto h-7 px-[0.7rem] border border-border rounded-[0.45rem] bg-transparent text-foreground-muted text-[0.78rem] cursor-pointer leading-none box-border transition-colors duration-100 hover:border-accent hover:text-foreground aria-expanded:border-accent aria-expanded:text-accent aria-expanded:bg-accent/6'

export const chatEmptyStateClass =
  'flex flex-col items-center justify-center flex-1 text-center text-foreground-muted p-12 gap-3'

export const chatEmptyStateTitleClass = 'text-[1.35rem] text-foreground font-semibold m-0'

export const chatEmptyStateDescClass =
  'text-[0.9rem] text-foreground-faint max-w-[24rem] leading-normal m-0'

export const chatEmptyStateSuggestionsClass =
  'flex flex-wrap justify-center gap-2 mt-2 max-w-[30rem]'

export const chatEmptyStateChipClass =
  'bg-card-raised text-foreground-muted border border-border rounded-full p-[0.4rem_0.85rem] text-[0.82rem] cursor-pointer transition-all duration-100 hover:border-accent hover:text-foreground hover:bg-accent/15'

export const chatInputAreaClass =
  'flex flex-col gap-2 shrink-0 max-[900px]:sticky max-[900px]:bottom-0 max-[900px]:bg-card max-[900px]:z-10 max-[900px]:pb-3'

export const chatComposerClass =
  'flex flex-col gap-[0.6rem] bg-card border border-border rounded-2xl p-[0.85rem_1rem] shadow-card transition-all duration-150 focus-within:border-accent focus-within:ring-2 focus-within:ring-[#5b8eff]/15'

export const chatComposerInputClass =
  'w-full min-h-[3.2rem] max-h-48 resize-none bg-transparent border-0 p-[0.15rem_0.1rem] text-foreground text-[0.92rem] leading-normal outline-none'

export const chatComposerBarClass = 'flex items-center justify-between gap-3 flex-wrap'

export const chatComposerOptionsClass = 'flex items-center gap-[0.85rem] flex-wrap min-w-0'

export const chatComposerHintClass = 'text-[0.72rem] text-foreground-faint whitespace-nowrap'

export const chatComposerActionsClass = 'flex items-center gap-2 ml-auto'

export const chatComposerActionBtnClass =
  '!w-auto !mt-0 h-[2.25rem] min-h-[2.25rem] shrink-0 whitespace-nowrap px-3.5'

export const chatComposerSendClass = 'inline-flex items-center gap-[0.4rem]'

export const chatComposerSendIconClass = 'text-[0.75rem] leading-none'

// ─── @-mention autosuggest (schema-aware prompt) ────────────────────
export const mentionWrapClass = 'relative'

export const mentionListClass =
  'absolute bottom-full left-0 right-0 mb-2 z-30 max-h-64 overflow-y-auto rounded-xl border border-border bg-card-raised shadow-card animate-[popoverFadeIn_0.14s_cubic-bezier(0.16,1,0.3,1)]'

export const mentionGroupClass = 'flex flex-col'

export const mentionGroupLabelClass =
  'px-3 pt-2 pb-1 text-[0.66rem] font-semibold uppercase tracking-wide text-foreground-faint'

export const mentionItemBaseClass =
  'w-full text-left flex items-center gap-2 px-3 py-[0.4rem] cursor-pointer transition-colors duration-100 border-0 bg-transparent'

export const mentionItemActiveClass = 'bg-accent text-white'

export const mentionItemTypeClass =
  'shrink-0 rounded-full border border-border bg-card px-[0.4rem] py-[0.05rem] text-[0.62rem] font-semibold text-foreground-muted'

export const mentionItemLabelClass = 'block truncate text-[0.84rem] font-medium'

export const mentionItemHintClass = 'block truncate text-[0.7rem] opacity-80'

export const pastQueriesToggleClass =
  'inline-flex items-center gap-[0.4rem] text-[0.78rem] text-foreground-muted cursor-pointer select-none'

export const feedbackRowClass = 'flex items-center gap-2 mt-2 border-t border-border pt-3'

export const feedbackLearnedBadgeClass = 'ml-2 text-[0.78rem] text-success'

const feedbackBtnBaseClass =
  'bg-transparent border border-transparent cursor-pointer text-[1.05rem] p-[0.3rem_0.6rem] rounded-[0.35rem] transition-all duration-150 hover:bg-card-raised hover:border-border-strong hover:scale-105 active:scale-95'

export function feedbackBtnClass(active: boolean, negative = false): string {
  if (!active) {
    return feedbackBtnBaseClass
  }
  return negative
    ? `${feedbackBtnBaseClass} bg-rose-500/12! border-rose-500/35!`
    : `${feedbackBtnBaseClass} bg-emerald-500/12! border-emerald-500/35!`
}

export const feedbackFormClass =
  'mt-2 p-4 bg-card-raised border border-border rounded-[0.6rem] flex flex-col gap-[0.65rem] animate-[popoverFadeIn_0.2s_cubic-bezier(0.16,1,0.3,1)]'

export const feedbackCategoriesClass = 'flex flex-wrap gap-[0.4rem]'

const feedbackCatBtnBaseClass =
  'bg-card border border-border rounded-[0.3rem] p-[0.3rem_0.6rem] text-[0.76rem] text-foreground-muted cursor-pointer transition-all duration-150 hover:border-foreground-faint hover:text-foreground'

export function feedbackCatBtnClass(active: boolean): string {
  return active
    ? `${feedbackCatBtnBaseClass} bg-accent! border-accent! text-white! hover:border-accent hover:text-white`
    : feedbackCatBtnBaseClass
}

export const btnRunQueryContainerClass = 'flex justify-start mt-2'

export const formRowClass = 'flex gap-4 items-end mb-4'

export const formFieldClass = 'flex flex-col gap-[0.35rem]'

export const formLabelClass = 'text-[0.8rem] font-semibold text-foreground-muted'

export const aiQueryMainClass =
  'flex-1 min-w-0 flex flex-col gap-4 h-full max-[900px]:h-auto max-[900px]:min-h-[55vh]'

export const cardHeaderRowClass = 'mb-[0.25rem]'

export const cardHeaderRowSpacedClass = 'mb-4'

export const contextBadgeClass =
  'text-[0.72rem] bg-card-raised border border-border rounded-full p-[0.2rem_0.6rem] text-foreground-muted'

export const queryConfigHeaderClass = 'flex flex-col gap-2 min-w-0'

export const queryConfigTopClass =
  'flex items-center gap-3 flex-nowrap min-w-0 max-[720px]:flex-wrap max-[720px]:items-start'

export const queryConfigEmbedBtnClass =
  'flex-0 shrink-0 w-auto max-w-[min(100%,14rem)] mt-0 whitespace-nowrap max-[720px]:max-w-full'

export const queryConfigEmbedStatusClass = 'flex flex-col gap-[0.25rem] text-[0.74rem] min-w-0'

export const queryControlsClass =
  'grid grid-cols-[minmax(11rem,1fr)_minmax(11rem,1fr)_max-content] gap-4 items-end max-[1340px]:grid-cols-2 max-[720px]:grid-cols-1'

export const aiRuntimeSettingsClass =
  'border border-border rounded-[0.6rem] p-[0.85rem] bg-card-raised'

export const aiSettingsGroupClass = 'align-self-stretch'

export const aiSettingsGridClass = 'flex flex-nowrap gap-2.5 max-[900px]:flex-wrap'

export const aiSettingsSectionClass =
  'min-w-0 border border-border rounded-lg p-[0.6rem_0.7rem] bg-card'

export const aiSettingsSectionTitleClass =
  'm-0 mb-2 text-foreground-muted text-[0.72rem] font-bold tracking-wider leading-normal uppercase'

export const aiSettingsDlClass =
  'grid grid-cols-[minmax(4.8rem,max-content)_minmax(0,1fr)] gap-x-[0.55rem] gap-y-1 m-0 text-[0.78rem]'

export const aiRuntimeSettingsTitleClass = 'text-[0.82rem] font-bold mb-2'

export const aiSettingsDtClass = 'text-foreground-muted font-semibold'

export const aiSettingsMetaClass =
  'block text-foreground-faint text-[0.68rem] leading-tight [overflow-wrap:anywhere]'

export const aiSettingsDdClass = 'min-w-0 m-0 [overflow-wrap:anywhere]'

export const aiSettingsCodeClass =
  'text-foreground text-[0.74rem] leading-tight [overflow-wrap:anywhere]'

export const aiSettingsHintClass = 'mt-2 text-[0.72rem] text-foreground-faint leading-normal'

export const aiEmbeddingActionsClass =
  'grid grid-cols-[max-content_minmax(12rem,1fr)] gap-x-3 gap-y-2.5 items-center mt-[0.7rem] pt-[0.7rem] border-t border-border max-[720px]:grid-cols-1'

export const aiEmbeddingStatusClass = 'text-success text-[0.74rem] leading-normal m-0'

export const aiEmbeddingErrorClass = 'text-error text-[0.74rem] leading-normal m-0'

export const routingToggleRowClass = 'inline-flex gap-2.5 items-center flex-nowrap max-w-full'

export const routingToggleRowGroupClass =
  'shrink-0 min-w-0 h-[2.1rem] box-border inline-flex items-center p-[0.15rem]'

export const routingToggleRowBtnClass =
  'h-full px-[0.85rem] inline-flex items-center justify-center box-border'

export const confidenceSectionClass = 'mb-5'

export const confidenceHeaderClass =
  "flex justify-between font-['Plus_Jakarta_Sans',sans-serif] text-[0.84rem] font-semibold mb-[0.4rem]"

export const confidenceBarBgClass =
  'h-2 rounded-full bg-canvas overflow-hidden shadow-[inset_0_1px_2px_rgba(0,0,0,0.2)]'

export const confidenceBarFillBaseClass =
  'h-full rounded-full transition-[width] duration-400 ease-[cubic-bezier(0.4,0,0.2,1)]'

export function confidenceBarFillClass(status: string): string {
  if (status === 'success') {
    return `${confidenceBarFillBaseClass} bg-gradient-to-r from-emerald-500 to-emerald-600 shadow-[0_0_8px_rgba(16,185,129,0.35)]`
  }
  if (status === 'warning') {
    return `${confidenceBarFillBaseClass} bg-gradient-to-r from-amber-500 to-amber-600 shadow-[0_0_8px_rgba(245,158,11,0.35)]`
  }
  return `${confidenceBarFillBaseClass} bg-gradient-to-r from-red-500 to-red-600 shadow-[0_0_8px_rgba(239,68,68,0.35)]`
}

export const confidenceBreakdownClass = 'mt-2.5 flex flex-col gap-2'

export const breakdownRowClass = 'flex items-center gap-2.5 text-[0.75rem] text-foreground-muted'

export const breakdownBarBgClass =
  'flex-1 h-1 rounded bg-canvas overflow-hidden shadow-[inset_0_0.5px_1px_rgba(0,0,0,0.1)]'

export const breakdownBarFillBaseClass =
  'h-full rounded-full transition-[width] duration-300 ease-[cubic-bezier(0.4,0,0.2,1)]'

export function breakdownBarFillClass(status: string): string {
  if (status === 'success') {
    return `${breakdownBarFillBaseClass} bg-gradient-to-r from-emerald-500 to-emerald-600`
  }
  if (status === 'warning') {
    return `${breakdownBarFillBaseClass} bg-gradient-to-r from-amber-500 to-amber-600`
  }
  return `${breakdownBarFillBaseClass} bg-gradient-to-r from-red-500 to-red-600`
}

export const confidenceHintClass = 'text-[0.75rem] text-warning mt-2 font-medium'

export const costBadgeClass =
  'text-[0.74rem] text-foreground-muted bg-card-raised border border-border rounded-[0.375rem] h-7 px-[0.55rem] mb-3 inline-flex items-center gap-[0.35rem] font-medium shadow-card-sm leading-none box-border'

export const retryBadgeClass =
  'text-[0.76rem] text-warning font-semibold bg-warning/6 border border-warning/25 rounded-[0.375rem] h-7 px-[0.55rem] mb-3 inline-flex items-center gap-1 leading-none box-border'

export const clarificationCardClass =
  'bg-[color-mix(in_srgb,var(--warning)_5%,var(--bg-card))] border border-[color-mix(in_srgb,var(--warning)_20%,transparent)] rounded-[0.6rem] p-[1.1rem_1.15rem] mb-4 shadow-card-sm'

export const clarificationCardAmbiguityClass =
  'bg-[color-mix(in_srgb,var(--accent)_5%,var(--bg-card))] border border-[color-mix(in_srgb,var(--accent)_22%,transparent)] rounded-[0.6rem] p-[1.1rem_1.15rem] mb-4 shadow-card-sm'

export const clarificationTitleClass =
  'flex items-center gap-2 font-semibold text-foreground mb-[0.35rem]'

export const clarificationRoundIndicatorClass =
  'ml-auto px-2 py-[0.1rem] rounded-full border border-[color-mix(in_srgb,var(--accent)_30%,transparent)] bg-[color-mix(in_srgb,var(--accent)_10%,var(--bg-card))] text-foreground-muted text-[0.72rem] font-semibold whitespace-nowrap'

export const clarificationCapNoticeClass =
  'm-0 mb-2 rounded border border-warning bg-warning/12 p-[0.4rem_0.6rem] text-[0.82rem] text-foreground'

export const clarificationReasonClass = 'm-0 mb-[0.35rem] text-[0.82rem] text-foreground-muted'

export const clarificationOptionHintClass =
  'mt-[0.1rem] block text-[0.74rem] font-normal leading-[1.35] text-foreground-faint'

export const clarificationAmbiguityTermsClass = 'list-none m-0 mt-2 mb-1 flex flex-col gap-1'

export const clarificationAmbiguityTermsLiClass = 'flex items-center gap-[0.45rem]'

export const clarificationAmbiguityTermsStrongClass = 'font-bold text-foreground text-[0.92rem]'

export const clarificationAmbiguityTypeClass =
  'text-[0.66rem] font-semibold tracking-wider uppercase text-accent bg-accent/12 border border-accent/25 px-2 py-[0.08rem] rounded-full'

export const generationTraceClass = 'mt-3 text-[0.88rem] text-foreground-muted'

export const generationTraceRowClass = 'flex flex-wrap items-baseline gap-x-2 gap-y-1 mb-2'

export const generationTraceLabelClass = 'font-semibold text-foreground'

export const generationTraceMetaClass = 'text-[0.78rem] text-foreground-faint'

export const generationTraceDetailClass =
  'm-0 mt-1 mb-2 text-[0.82rem] leading-normal whitespace-pre-wrap'

export const generationTraceColumnsClass = 'list-none m-0 mt-2 flex flex-col gap-1.5'

export const generationTraceColumnsLiClass = 'flex flex-wrap items-center gap-[0.35rem]'

export const generationTraceArrowClass = 'text-foreground-faint text-[0.85rem]'

export const clarificationQuestionClass =
  'text-foreground-muted m-0 mt-[0.15rem] mb-[0.85rem] text-[0.86rem] leading-normal'

export const clarificationOptionsClass = 'flex flex-col gap-2 mb-[0.85rem]'

export const btnClarificationClass =
  'flex flex-col items-start gap-[0.15rem] w-full text-left bg-card-raised border border-border-strong text-foreground p-[0.6rem_0.85rem] rounded-lg cursor-pointer font-semibold text-[0.85rem] transition-all duration-150 hover:bg-card hover:border-accent hover:shadow-[0_2px_8px_var(--accent-glow)] hover:-translate-y-[1px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-1 active:scale-[0.995]'

export const btnSkipClass =
  'block mx-auto bg-transparent border-0 text-foreground-faint cursor-pointer text-[0.8rem] underline transition-colors duration-150 hover:text-foreground-muted'

export const candidatePanelClass = 'mb-5'

export const candidateHeaderClass = 'text-[0.88rem] font-bold mb-3 text-foreground tracking-tight'

export const candidateCardsClass =
  'flex gap-3 overflow-x-auto pb-[0.65rem] scroll-smooth max-[900px]:flex-col'

const candidateCardBaseClass =
  'min-w-[270px] max-w-[320px] max-[900px]:max-w-none max-[900px]:min-w-0 bg-card border border-border rounded-[0.6rem] p-[0.85rem] shrink-0 transition-all duration-150 shadow-card-sm hover:-translate-y-[2px] hover:border-border-strong hover:shadow-card'

export function candidateCardClass(best: boolean): string {
  if (!best) {
    return candidateCardBaseClass
  }
  return `${candidateCardBaseClass} border-success bg-[color-mix(in_srgb,var(--success)_2.5%,var(--bg-card))] hover:border-success shadow-[0_0_10px_rgba(16,185,129,0.12)] hover:shadow-[0_0_16px_rgba(16,185,129,0.22)]`
}

export const candidateCardHeaderClass = 'flex justify-between text-[0.78rem] font-bold mb-2'

export function candidateScoreClass(best: boolean): string {
  return best ? 'text-success font-bold' : 'text-foreground-faint'
}

export const candidateReasoningClass =
  'text-[0.75rem] text-foreground-muted mb-[0.65rem] leading-normal'

export const candidateJsonClass =
  'font-mono text-[0.7rem] max-h-[150px] overflow-auto bg-black/25 rounded-[0.4rem] p-[0.45rem] border border-border text-zinc-300 [data-theme=light]:bg-black/3 [data-theme=light]:text-slate-900 [data-theme=light]:border-border-strong'

export const btnCandidateUseClass =
  'mt-2.5 w-full p-[0.45rem] bg-card-raised border border-border-strong text-foreground rounded-[0.4rem] cursor-pointer text-[0.78rem] font-semibold transition-all duration-150 hover:bg-[var(--control-hover-bg)] hover:border-accent hover:text-foreground hover:shadow-[0_2px_6px_var(--accent-glow)] active:scale-[0.98]'

export const tableRoutingVizClass = 'py-[0.25rem]'

export const routingHeaderClass = 'flex justify-between text-[0.82rem] font-semibold mb-2'

export const routingConfidenceClass = 'text-foreground-muted'

export const routingContextGridClass =
  'grid grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-2 mt-[0.65rem] mb-[0.85rem]'

export const routingContextGridItemClass =
  'border border-border bg-card rounded-lg p-[0.55rem_0.7rem] min-w-0 shadow-card-sm transition-all duration-150 hover:border-border-strong hover:-translate-y-[0.5px]'

export const routingContextLabelClass =
  'block text-foreground-faint text-[0.65rem] font-bold mb-[0.2rem] uppercase tracking-wider'

export const routingContextValueClass =
  'block text-foreground text-[0.78rem] font-semibold leading-normal [overflow-wrap:anywhere]'

export const routingTableListClass = 'grid gap-1'

export const routingCandidateClass =
  'grid grid-cols-[minmax(150px,220px)_minmax(80px,1fr)_42px_18px_96px] max-[640px]:grid-cols-[minmax(96px,36vw)_minmax(64px,1fr)_38px_14px] items-center gap-2.5 text-[0.78rem] mb-[0.35rem]'

export const routingTableNameClass =
  'min-w-0 font-mono text-[0.74rem] text-foreground overflow-hidden text-ellipsis whitespace-nowrap'

export const routingBarBgClass = 'flex-1 h-1.5 rounded-full bg-canvas overflow-hidden'

export const routingBarFillClass =
  'h-full rounded-full bg-gradient-to-r from-accent to-accent-strong transition-[width] duration-300 shadow-[0_0_4px_var(--accent-glow)]'

export const routingScoreClass =
  'min-w-[32px] text-right text-[0.72rem] text-foreground-faint [font-variant-numeric:tabular-nums]'

export const routingScoreDetailClass =
  'min-w-[92px] max-[640px]:col-span-full max-[640px]:justify-self-end text-foreground-faint text-[0.68rem]'

export const routingSelectedClass = 'text-success font-bold shadow-[0_0_6px_rgba(5,150,105,0.15)]'

export const routingSelectedEmptyClass = 'invisible'

export const routingReasoningClass =
  'text-[0.75rem] text-foreground-muted italic mt-2 leading-normal'

export const routingDebugClass =
  'grid gap-1.5 mt-[0.75rem] pt-[0.7rem] border-t border-border-strong'

export const routingDebugCodeClass =
  'block [white-space:normal] [overflow-wrap:anywhere] text-foreground-muted text-[0.72rem] bg-black/20 p-2.5 rounded-[0.35rem] [data-theme=light]:bg-black/3'

export const btnSampleClass = 'mt-2.5 text-[0.75rem] p-[0.3rem_0.75rem]'

export const collapsibleSectionClass =
  'mb-3 bg-card border border-border rounded-lg overflow-hidden shadow-card-sm transition-all duration-150'

export const collapsibleSectionSummaryClass =
  'cursor-pointer font-semibold text-[0.84rem] text-foreground p-[0.65rem_0.95rem] bg-black/2 border-b border-transparent list-none flex justify-between items-center transition-all duration-180 select-none hover:bg-canvas-subtle hover:text-foreground'

export const collapsibleContentClass = 'p-[0.9rem]'

export const explainOutputClass = 'max-h-[220px] overflow-auto font-mono'

export function planStatusClass(ok: boolean): string {
  const base = 'text-[0.8rem] mt-2.5 font-bold'
  return ok ? `${base} text-success` : `${base} text-warning`
}

export const tokenInfoClass = 'text-[0.75rem] text-foreground-faint mt-[0.3rem]'

export const errorRecoveryClass = 'bg-rose-500/6 border border-rose-500/20 rounded-lg p-3 mb-4'

export const errorRecoveryPClass = 'mb-2 text-error text-[0.85rem]'

export const recoveryOptionsClass = 'flex gap-2'

export const recoveryOptionsButtonClass =
  'p-[0.35rem_0.7rem] bg-card-raised border border-border-strong text-foreground rounded-[0.35rem] cursor-pointer text-[0.8rem] hover:border-accent'

export const resultsSectionClass = 'mt-4'

export const resultsHeaderClass = 'flex items-center gap-3 flex-wrap mb-3'

export const vizHintClass =
  'text-[0.75rem] text-foreground-muted bg-card-raised border border-border rounded-[0.3rem] p-[0.15rem_0.5rem] cursor-help'

export const chartToggleClass = 'flex gap-1.5 ml-auto'

const chartToggleBtnBase =
  'p-[0.25rem_0.6rem] bg-transparent border border-border text-foreground-muted rounded-[0.3rem] cursor-pointer text-[0.75rem] hover:text-foreground hover:border-border-strong'

export function chartToggleBtnClass(active: boolean): string {
  return active
    ? `${chartToggleBtnBase} bg-card-raised border-border-strong text-foreground`
    : chartToggleBtnBase
}

export const resultFooterClass =
  'flex justify-between text-[0.75rem] text-foreground-faint pt-[0.4rem]'

export const resultsTableSortButtonClass =
  'inline-flex items-center gap-[0.35rem] w-full border-0 bg-transparent text-inherit cursor-pointer text-left font-inherit font-weight-inherit focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2'

export const contextMenuClass =
  'bg-card border border-border-strong rounded-[0.4rem] shadow-card overflow-hidden'

export const contextMenuItemClass =
  'block w-full text-left p-[0.4rem_0.75rem] bg-transparent border-0 text-foreground cursor-pointer text-[0.8rem] hover:bg-card-raised'

export const promptPreviewClass = 'max-h-[300px] overflow-auto text-[0.72rem]'

export const followUpBarClass = 'mt-4 p-3 bg-card border border-border rounded-[0.6rem]'

export const followUpBarInputClass =
  'w-full p-[0.5rem_0.75rem] bg-canvas border border-border rounded-[0.4rem] text-foreground text-[0.85rem] focus:outline-none focus:border-accent'
