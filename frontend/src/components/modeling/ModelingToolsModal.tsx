import type { ReactNode } from 'react'

import type { TranslationKey } from '../../i18n'
import { cn } from '../../lib/cn'
import { Modal } from '../ui/Modal'

export type ModelingToolsTab = 'semantic' | 'relationship'

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string

interface ModelingToolLaunchersProps {
  tableCount: number
  relationshipCount: number
  onOpen: (tab: ModelingToolsTab) => void
  t: Translate
}

interface ModelingToolsModalProps {
  activeTab: ModelingToolsTab | null
  onTabChange: (tab: ModelingToolsTab) => void
  onClose: () => void
  semanticContent: ReactNode
  relationshipContent: ReactNode
  t: Translate
}

const launcherClass =
  'group flex min-w-0 touch-manipulation items-center gap-2.5 border-0 bg-transparent px-3.5 py-2.5 text-left text-foreground transition-colors duration-150 hover:bg-accent/8 focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent motion-reduce:transition-none'

export function ModelingToolLaunchers({
  tableCount,
  relationshipCount,
  onOpen,
  t,
}: ModelingToolLaunchersProps) {
  return (
    <div className="border-border bg-card/94 absolute bottom-4 left-4 z-20 flex overflow-hidden rounded-xl border shadow-[0_8px_24px_rgba(15,23,42,0.14)] backdrop-blur-md max-[560px]:right-3 max-[560px]:bottom-3 max-[560px]:left-3">
      <button
        type="button"
        className={launcherClass}
        aria-label={t('modeling.open_semantic_panel')}
        onClick={() => onOpen('semantic')}
      >
        <span
          className="bg-accent/12 text-accent grid size-8 shrink-0 place-items-center rounded-lg text-base"
          aria-hidden="true"
        >
          ◫
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="text-[0.78rem] leading-tight font-bold">
            {t('modeling.semantic_layer')}
          </span>
          <span className="text-foreground-muted text-[0.68rem] leading-tight tabular-nums">
            {t('modeling.tools_tables_count', { count: tableCount })}
          </span>
        </span>
      </button>
      <span className="bg-border my-2 w-px" aria-hidden="true" />
      <button
        type="button"
        className={launcherClass}
        aria-label={t('modeling.open_join_panel')}
        onClick={() => onOpen('relationship')}
      >
        <span
          className="bg-accent/12 text-accent grid size-8 shrink-0 place-items-center rounded-lg text-base"
          aria-hidden="true"
        >
          ⇄
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="text-[0.78rem] leading-tight font-bold">
            {t('modeling.add_relationship')}
          </span>
          <span className="text-foreground-muted text-[0.68rem] leading-tight tabular-nums">
            {t('modeling.tools_relationships_count', { count: relationshipCount })}
          </span>
        </span>
      </button>
    </div>
  )
}

const tabClass =
  'flex w-full touch-manipulation items-start gap-3 rounded-xl border border-transparent p-3 text-left transition-[border-color,background-color,color,box-shadow] duration-150 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent motion-reduce:transition-none'

export function ModelingToolsModal({
  activeTab,
  onTabChange,
  onClose,
  semanticContent,
  relationshipContent,
  t,
}: ModelingToolsModalProps) {
  const selectedTab = activeTab ?? 'semantic'
  const isSemantic = selectedTab === 'semantic'

  return (
    <Modal
      open={activeTab !== null}
      title={t('modeling.tools_modal_title')}
      subtitle={t('modeling.tools_modal_subtitle')}
      onClose={onClose}
      labelledBy="modeling-tools-title"
      className="w-[min(100%,76rem)] overflow-hidden"
      bodyClassName="block p-0 max-[680px]:p-0"
    >
      <div className="grid min-h-0 grid-cols-[15rem_minmax(0,1fr)] max-[760px]:grid-cols-1">
        <div
          className="border-border bg-card-raised/75 flex flex-col gap-2 border-r p-3 max-[760px]:grid max-[760px]:grid-cols-2 max-[760px]:border-r-0 max-[760px]:border-b"
          role="tablist"
          aria-label={t('modeling.tools_tabs_aria')}
        >
          <button
            id="modeling-tools-tab-semantic"
            type="button"
            role="tab"
            className={cn(
              tabClass,
              isSemantic
                ? 'border-accent/25 bg-card text-accent shadow-sm'
                : 'text-foreground-muted hover:bg-card/70 hover:text-foreground',
            )}
            aria-selected={isSemantic}
            aria-controls="modeling-tools-panel-semantic"
            aria-label={t('modeling.semantic_layer')}
            tabIndex={isSemantic ? 0 : -1}
            data-modal-autofocus={isSemantic || undefined}
            onClick={() => onTabChange('semantic')}
            onKeyDown={(event) => {
              if (event.key === 'ArrowRight' || event.key === 'ArrowDown' || event.key === 'End') {
                event.preventDefault()
                onTabChange('relationship')
              }
            }}
          >
            <span
              className="bg-accent/12 text-accent grid size-9 shrink-0 place-items-center rounded-lg"
              aria-hidden="true"
            >
              ◫
            </span>
            <span className="min-w-0">
              <strong className="block text-[0.82rem] leading-tight">
                {t('modeling.semantic_layer')}
              </strong>
              <span className="mt-1 block text-[0.68rem] leading-snug">
                {t('modeling.tab_short_tables')} · {t('modeling.joins_tab')}
              </span>
            </span>
          </button>
          <button
            id="modeling-tools-tab-relationship"
            type="button"
            role="tab"
            className={cn(
              tabClass,
              !isSemantic
                ? 'border-accent/25 bg-card text-accent shadow-sm'
                : 'text-foreground-muted hover:bg-card/70 hover:text-foreground',
            )}
            aria-selected={!isSemantic}
            aria-controls="modeling-tools-panel-relationship"
            aria-label={t('modeling.manual_relationship')}
            tabIndex={isSemantic ? -1 : 0}
            data-modal-autofocus={!isSemantic || undefined}
            onClick={() => onTabChange('relationship')}
            onKeyDown={(event) => {
              if (event.key === 'ArrowLeft' || event.key === 'ArrowUp' || event.key === 'Home') {
                event.preventDefault()
                onTabChange('semantic')
              }
            }}
          >
            <span
              className="bg-accent/12 text-accent grid size-9 shrink-0 place-items-center rounded-lg"
              aria-hidden="true"
            >
              ⇄
            </span>
            <span className="min-w-0">
              <strong className="block text-[0.82rem] leading-tight">
                {t('modeling.manual_relationship')}
              </strong>
              <span className="mt-1 block text-[0.68rem] leading-snug">
                {t('modeling.manual_title')}
              </span>
            </span>
          </button>
        </div>

        <section
          id={isSemantic ? 'modeling-tools-panel-semantic' : 'modeling-tools-panel-relationship'}
          className="bg-card max-h-[min(72vh,48rem)] min-h-[32rem] overflow-y-auto overscroll-contain max-[760px]:min-h-0"
          role="tabpanel"
          aria-labelledby={
            isSemantic ? 'modeling-tools-tab-semantic' : 'modeling-tools-tab-relationship'
          }
          tabIndex={0}
        >
          {isSemantic ? semanticContent : relationshipContent}
        </section>
      </div>
    </Modal>
  )
}
