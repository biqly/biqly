import { buttonClass } from '../../lib/buttonClasses'

export const adminLayoutClass =
  'grid min-w-0 gap-5 min-[900px]:grid-cols-[minmax(12rem,14.5rem)_minmax(0,1fr)] min-[900px]:items-start min-[900px]:gap-x-8 min-[900px]:gap-y-6'

export const adminContentClass = 'min-w-0'

export const adminNavClass = 'min-w-0'

export const adminNavMobileClass = 'flex flex-col gap-[0.4rem] min-[900px]:hidden'

export const adminNavMobileLabelClass =
  'm-0 text-[0.72rem] font-semibold tracking-[0.06em] text-foreground-muted uppercase'

export const adminNavSelectClass =
  'w-full cursor-pointer rounded-[0.55rem] border border-border bg-card px-[0.85rem] py-[0.65rem] text-[0.9rem] font-medium text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent'

export const adminNavDesktopClass =
  'hidden min-[900px]:sticky min-[900px]:top-4 min-[900px]:flex min-[900px]:flex-col min-[900px]:gap-[1.15rem] min-[900px]:rounded-xl min-[900px]:border min-[900px]:border-border min-[900px]:bg-card min-[900px]:p-[0.85rem] min-[900px]:shadow-sm'

export const adminNavGroupClass =
  'flex flex-col gap-1 border-t border-border pt-3 mt-3 first:border-t-0 first:pt-0 first:mt-0'

export const adminNavGroupTitleClass =
  'm-0 mb-[0.2rem] px-[0.65rem] font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.62rem] font-bold tracking-[0.11em] text-foreground-muted uppercase select-none'

export const adminNavListClass = 'm-0 flex list-none flex-col gap-[0.1rem] py-0 pl-0'

export const adminNavLinkBaseClass =
  'block w-full cursor-pointer rounded-lg border-l-2 border-transparent bg-transparent px-[0.7rem] py-[0.52rem] text-left text-[0.875rem] font-medium leading-[1.35] text-foreground-muted transition-[background,color,border-color] duration-150 hover:bg-card-raised hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent'

export const adminNavLinkActiveClass =
  'border-l-accent bg-(--nav-link-active-bg) font-semibold text-foreground shadow-none'

export function adminNavLinkClass(active: boolean): string {
  return active ? `${adminNavLinkBaseClass} ${adminNavLinkActiveClass}` : adminNavLinkBaseClass
}

export const adminPanelClass = 'flex flex-col gap-4'

export const adminPanelHeaderClass = 'flex flex-wrap items-center justify-between gap-3'

// Delegate to the shared ui/Button styling (buttonClass) so admin buttons render
// the same design-system look as <Button>. autoWidth keeps them inline (the base
// buttonClass is full-width), matching the original admin toolbar layout.
export const adminBtnPrimaryClass = buttonClass('primary', { autoWidth: true })

export const adminBtnSecondaryClass = buttonClass('secondary', { autoWidth: true })

export const adminBtnGhostClass = buttonClass('ghost', { autoWidth: true })

export const adminBtnDangerClass =
  'cursor-pointer rounded border border-red-500/20 bg-red-500/10 px-2.5 py-1 text-xs font-medium text-error transition-all duration-200 hover:bg-red-500/20'

export const adminBtnRevokeClass =
  'cursor-pointer rounded border border-error bg-transparent px-2 py-1 text-xs font-medium text-error transition-all duration-200 hover:bg-red-500/10'

export const adminBtnSubmitClass =
  'mt-2 cursor-pointer rounded-md border-0 bg-accent px-2.5 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-[var(--accent-hover,#4338ca)]'

export const adminBtnActivateClass =
  'cursor-pointer rounded-md border-0 bg-success px-4 py-2 text-caption font-semibold text-white transition-all duration-200 hover:opacity-90'

export const adminBtnDeactivateClass =
  'cursor-pointer rounded-md border-0 bg-error px-4 py-2 text-caption font-semibold text-white transition-all duration-200 hover:opacity-90'

export const adminBtnResendClass =
  'cursor-pointer rounded-md border border-border bg-card-raised px-3 py-1.5 text-caption font-medium text-foreground transition-all duration-200 hover:bg-[var(--control-hover-bg,rgba(255,255,255,0.08))]'

export const adminRoleListItemBaseClass =
  'block w-full cursor-pointer border-0 border-b border-border bg-transparent px-4 py-3 text-left text-foreground transition-[background] duration-150 last:border-b-0 hover:bg-card-raised'

export const adminRoleListItemActiveClass =
  'bg-[var(--accent-glow)] shadow-[inset_3px_0_0_var(--accent)]'

export function adminRoleListItemClass(active: boolean): string {
  return active
    ? `${adminRoleListItemBaseClass} ${adminRoleListItemActiveClass}`
    : adminRoleListItemBaseClass
}

const adminBadgeBaseClass = 'inline-block rounded-full px-2 py-0.5 text-xs font-medium'

export const adminBadgeActiveClass = `${adminBadgeBaseClass} bg-emerald-500/12 text-success`

export const adminBadgeInactiveClass = `${adminBadgeBaseClass} bg-red-500/12 text-error`

export const adminBadgeVerifiedClass = `${adminBadgeBaseClass} bg-[var(--accent-glow)] text-accent`

export const adminBadgeUnverifiedClass = `${adminBadgeBaseClass} bg-amber-500/14 text-warning`

export const adminBadgeClaimedClass = `${adminBadgeBaseClass} bg-emerald-500/12 text-success`

export const adminBadgePendingClass = `${adminBadgeBaseClass} bg-amber-500/14 text-warning`

export const adminBadgeExpiredClass = `${adminBadgeBaseClass} bg-red-500/12 text-error`

export const adminBadgeGlobalClass =
  'inline-block rounded px-1.5 py-px text-2xs font-semibold uppercase bg-card-raised text-foreground'

export const adminBadgeWorkspaceClass =
  'inline-block rounded px-1.5 py-px text-2xs font-semibold uppercase bg-emerald-500/12 text-success'

export function adminActiveBadgeClass(active: boolean): string {
  return active ? adminBadgeActiveClass : adminBadgeInactiveClass
}

export function adminVerifiedBadgeClass(verified: boolean): string {
  return verified ? adminBadgeVerifiedClass : adminBadgeUnverifiedClass
}

export function adminMfaStatusBadgeClass(mfaEnabled?: boolean, mfaPending?: boolean): string {
  if (mfaEnabled) {
    return adminBadgeVerifiedClass
  }
  if (mfaPending) {
    return adminBadgeUnverifiedClass
  }
  return adminBadgeInactiveClass
}

export const adminBadgeNeutralClass = `${adminBadgeBaseClass} bg-card-raised text-foreground-muted`

export function adminPasskeyBadgeClass(count: number): string {
  return count > 0 ? adminBadgeVerifiedClass : adminBadgeNeutralClass
}

export const adminRowRevealActionClass =
  'opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:opacity-100'

export function adminInvitationBadgeClass(status: 'claimed' | 'expired' | 'pending'): string {
  if (status === 'claimed') {
    return adminBadgeClaimedClass
  }
  if (status === 'expired') {
    return adminBadgeExpiredClass
  }
  return adminBadgePendingClass
}

export function adminScopeBadgeClass(scopeType: string): string {
  return scopeType === 'global' ? adminBadgeGlobalClass : adminBadgeWorkspaceClass
}

export function adminActivateBtnClass(isActive: boolean): string {
  return isActive ? adminBtnDeactivateClass : adminBtnActivateClass
}

export const cardLeadMarginClass = 'mb-5!'

export const adminBadgeActionClass =
  'inline-flex items-center rounded border border-border bg-[var(--accent-glow)] px-1.5 py-0.5 font-mono text-xs text-accent'

export const adminCountBadgeClass =
  'rounded-xl border border-border bg-card-raised px-2 py-1 text-xs font-semibold text-foreground-muted'

export const adminCardClass =
  'rounded-lg border border-border bg-card p-6 shadow-sm max-[680px]:p-4'

export const adminAvatarCircleClass =
  'flex size-14 items-center justify-center rounded-full bg-[var(--bg-avatar,#e0e7ff)] text-lg font-bold text-[var(--text-avatar,#4f46e5)]'

export const adminGridClass =
  'mt-4 grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4 border-t border-border pt-4 max-[680px]:mt-3 max-[680px]:grid-cols-1 max-[680px]:gap-3 max-[680px]:pt-3'

export const adminGridItemClass = 'flex flex-col gap-1'

export const adminFormLabelClass =
  'flex flex-col gap-1.5 text-caption font-semibold text-foreground-muted [&_.ui-select]:w-full [&_.ui-select]:min-w-0'

export const adminFilterRowClass = '[&_.ui-select]:min-w-0 [&_.ui-select]:flex-1'

export const adminLabelClass =
  'text-xs font-semibold tracking-[0.05em] text-foreground-muted uppercase'

export const adminLabelTextClass = 'text-xs text-foreground-muted'

export const adminValClass = 'text-sm font-medium text-foreground'

export const adminInputClass =
  'w-full rounded-md border border-border bg-[var(--input-bg,#fff)] px-3 py-2 text-sm text-foreground focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none'

export const adminInputWideClass = `${adminInputClass} box-border`

export const adminSelectClass =
  'rounded-md border border-border bg-[var(--input-bg,#fff)] px-3 py-2 text-sm text-foreground focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none'

export const adminSelectWideClass = `${adminSelectClass} w-full`

export const adminSelectSmallClass =
  'rounded border border-border bg-[var(--input-bg,#fff)] px-2 py-1 text-xs text-foreground focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none'

export const adminTableContainerClass =
  'overflow-x-auto overflow-hidden rounded-lg border border-border bg-card shadow-sm [-webkit-overflow-scrolling:touch]'

export const adminTableClass =
  'w-full border-collapse text-left text-sm max-[899px]:min-w-[36rem] max-[680px]:min-w-[32rem]'

export const adminTheadRowClass =
  'border-b border-border bg-[var(--table-header-bg,#f9fafb)] text-left'

export const adminThClass = 'px-4 py-3 font-semibold text-[var(--table-header-fg,#4b5563)]'

export const adminTrClass = 'border-b border-border'

export const adminTableRowHoverClass = `${adminTrClass} group transition-colors duration-150 hover:bg-[var(--table-stripe-hover)]`

export const adminTdClass = 'px-4 py-3 text-foreground'

export const adminTdMonoClass = 'px-4 py-3 font-mono text-xs text-foreground'

export const adminTdMetadataClass =
  'max-w-[360px] overflow-hidden px-4 py-3 font-mono text-xs text-ellipsis whitespace-nowrap text-foreground'

export const adminSubtextClass = 'font-mono text-2xs text-[#9ca3af]'

export const adminTextMutedClass = 'p-4 text-sm text-foreground-muted'

export const adminErrTextClass = 'p-4 font-semibold text-error'

export const adminSuccessBoxClass = 'rounded-md bg-emerald-500/12 p-3 text-sm text-success'

export const adminErrBoxClass = 'rounded-md bg-red-500/12 p-3 text-sm text-error'

export function adminMessageBoxClass(type: 'success' | 'error'): string {
  return type === 'success' ? adminSuccessBoxClass : adminErrBoxClass
}

export const adminTabContainerClass = 'mb-2 flex gap-4 border-b border-border'

export const adminTabButtonBaseClass =
  'cursor-pointer border-0 border-b-2 border-transparent bg-transparent px-4 pt-2 pb-3 text-sm font-medium text-foreground-muted outline-none transition-all duration-200'

export const adminTabButtonActiveClass = 'border-b-2 border-accent font-semibold text-foreground'

export function adminTabButtonClass(active: boolean): string {
  return active
    ? `${adminTabButtonBaseClass} ${adminTabButtonActiveClass}`
    : adminTabButtonBaseClass
}

export const adminLevelReadClass = 'bg-emerald-500/12! font-semibold text-success!'

export const adminLevelWriteClass = 'bg-amber-500/14! font-semibold text-warning!'

export const adminLevelAdminClass = 'bg-red-500/12! font-semibold text-error!'

export function adminLevelClass(level: string): string {
  if (level === 'read') {
    return adminLevelReadClass
  }
  if (level === 'write') {
    return adminLevelWriteClass
  }
  if (level === 'admin') {
    return adminLevelAdminClass
  }
  return ''
}

export const adminAlertSuccessClass =
  'mb-5 flex items-center justify-between rounded-lg border border-success bg-[color-mix(in_srgb,var(--success)_12%,transparent)] px-4 py-3 text-[0.82rem] font-medium text-success'

export const adminCenterContainerClass = 'flex justify-center p-10'

export const adminAlertCloseBtnClass =
  'cursor-pointer border-0 bg-transparent px-1 text-[1.2rem] leading-none text-inherit'

export const adminUserSecurityClass = 'flex flex-col items-start gap-1'

export const adminListAvatarClass = 'size-8 shrink-0 rounded-full object-cover bg-card-raised'

export const adminRolesGridClass =
  'grid min-w-0 grid-cols-[3fr_2fr] items-start gap-6 max-[800px]:grid-cols-1 [&>*]:min-w-0'

export const mfaQrContainerClass =
  'mx-auto my-2 flex w-fit justify-center self-center rounded-lg bg-white p-2'

export const mfaOtpInputClass = 'text-center text-xl tracking-[4px]'

export const recoveryCodesGridClass =
  'grid grid-cols-2 gap-2 rounded bg-black/20 p-3 text-center font-mono text-[0.9rem] text-accent'

export const adminBtnAutoWidthClass = 'm-0! w-auto!'

export const adminBtnIconOnlyClass =
  'm-0! inline-flex! aspect-square! size-9! min-h-auto! items-center! justify-center! p-0!'

export const adminBtnSmIconOnlyClass = 'size-[1.85rem]!'

export const adminFlexGapCenterEndClass = 'flex items-center justify-end gap-1'

export const adminRecoveryCodesBorderClass = 'border border-border'

export const ldapFieldsetClass =
  'm-0 flex flex-col gap-3.5 rounded-[10px] border border-border bg-[var(--surface-elevated,rgba(255,255,255,0.02))] p-[16px_18px_18px] disabled:opacity-60 [&>legend]:px-1.5 [&>legend]:text-xs [&>legend]:font-semibold [&>legend]:tracking-[0.4px] [&>legend]:text-foreground-muted [&>legend]:uppercase'

export const ldapGridClass = 'grid grid-cols-4 gap-3 max-[720px]:grid-cols-1 [&>*]:min-w-0'

export const ldapToggleClass =
  'flex cursor-pointer items-start gap-3 rounded-[10px] border border-border bg-card-raised p-[14px_16px] [&_input]:mt-[3px]'

export const adminRangeSliderClass = 'admin-range-slider'

export const jobDetailModalClass = 'w-[min(100%,56rem)]'

export const jobDetailModalBodyClass = 'max-h-[min(80vh,40rem)] overflow-y-auto'
