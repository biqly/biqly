import { useT } from '../../i18n'
import type { AIRuntimeSettings } from '../../types/ai'
import type { TableRow } from '../../types/semantic'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { type BulkEntry } from './bulkProgress'
import { MetadataBulkDescribeProgress } from './MetadataBulkDescribeProgress'
import { MetadataBulkDescribeSetup } from './MetadataBulkDescribeSetup'
import { useMetadataBulkDescribeModalState } from './useMetadataBulkDescribeModalState'

export interface MetadataBulkDescribeModalProps {
  open: boolean
  onClose: () => void
  datasourceId: string
  tables: TableRow[]
  schemaOptions: string[]
  typeOptions: string[]
  aiRuntime: AIRuntimeSettings | null
  describeModel?: string
  bulkRunning: boolean
  bulkEntries: BulkEntry[]
  bulkSummary: { ok: number; error: number; skipped: number } | null
  activeDescribeBatchJob: { progress_json?: unknown } | null | undefined
  onStartBulk: (params: {
    targets: TableRow[]
    sampleSize: number
    skipExisting: boolean
    onConflict: (message: string) => void
    onFinished: () => void
  }) => void
  onCancelBulk: () => void
  onRefreshTables: () => void
}

export function MetadataBulkDescribeModal({
  open,
  onClose,
  datasourceId,
  tables,
  schemaOptions,
  typeOptions,
  aiRuntime,
  describeModel,
  bulkRunning,
  bulkEntries,
  bulkSummary,
  activeDescribeBatchJob,
  onStartBulk,
  onCancelBulk,
  onRefreshTables,
}: MetadataBulkDescribeModalProps) {
  const t = useT()
  const dbManaged = aiRuntime?.db_managed === true
  const managedRuntime = dbManaged ? aiRuntime : null
  const activeDescribe = managedRuntime?.active_models?.find((m) => m.purpose === 'describe')
  const activeTranslation = managedRuntime?.active_models?.find((m) => m.purpose === 'translation')

  const {
    bulkConfig,
    setBulkConfig,
    bulkTypeEnabled,
    setBulkTypeEnabled,
    bulkSchemaRestrict,
    setBulkSchemaRestrict,
    bulkSchemasSelected,
    setBulkSchemasSelected,
    bulkScopeConflict,
    bulkTargetTables,
    bulkHasObjectType,
    bulkCanStart,
    bulkEntriesDisplay,
    runBulkDescribe,
  } = useMetadataBulkDescribeModalState({
    open,
    datasourceId,
    tables,
    typeOptions,
    bulkRunning,
    bulkEntries,
    t,
    onStartBulk,
    onRefreshTables,
  })

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose()
        }
      }}
    >
      <section
        className="modal-card w-[min(100%,52rem)] max-h-[min(calc(100vh-1.5rem),90vh)] flex flex-col min-h-0"
        role="dialog"
        aria-modal="true"
        aria-labelledby="bulk-metadata-title"
      >
        <header className="modal-header items-start p-[0.65rem_1rem] gap-3">
          <div>
            <h2
              id="bulk-metadata-title"
              className="m-0 text-[0.95rem] font-[650] tracking-[-0.02em] leading-tight"
            >
              {t('metadata.bulk_modal_title')}
            </h2>
            <p className="mt-[0.2rem] mx-0 mb-0 text-[0.72rem] text-foreground-faint leading-[1.35] max-w-lg">
              {t('metadata.bulk_modal_subtitle')}
            </p>
            <ModelBadgeRow
              primaryLabel={t('metadata.describe_badge_label')}
              primaryModel={describeModel ?? aiRuntime?.llm_model}
              primaryNote={dbManaged ? activeDescribe?.provider_name : undefined}
              translationModel={
                aiRuntime?.translation_enabled ? aiRuntime.translation_model : undefined
              }
              translationNote={dbManaged ? activeTranslation?.provider_name : undefined}
            />
          </div>
          <button
            type="button"
            className="modal-close shrink-0 mt-[0.05rem]"
            aria-label={t('metadata.bulk_close_aria')}
            onClick={onClose}
          >
            ×
          </button>
        </header>
        <div
          className={`modal-body ${
            bulkEntries.length > 0
              ? 'modal-body--scroll flex-1 min-h-0 flex flex-col overflow-hidden gap-[0.65rem] pt-[0.85rem] pb-4'
              : 'gap-[0.65rem] p-[0.85rem_1rem_1rem]'
          }`}
        >
          {bulkEntries.length === 0 && !bulkRunning && (
            <MetadataBulkDescribeSetup
              t={t}
              typeOptions={typeOptions}
              bulkTypeEnabled={bulkTypeEnabled}
              onToggleType={(ty) => setBulkTypeEnabled((prev) => ({ ...prev, [ty]: !prev[ty] }))}
              bulkHasObjectType={bulkHasObjectType}
              bulkSchemaRestrict={bulkSchemaRestrict}
              onSchemaRestrictAll={() => {
                setBulkSchemaRestrict(false)
                setBulkSchemasSelected([])
              }}
              onSchemaRestrictPick={() => {
                setBulkSchemaRestrict(true)
                setBulkSchemasSelected((prev) => (prev.length > 0 ? prev : [...schemaOptions]))
              }}
              schemaOptions={schemaOptions}
              bulkSchemasSelected={bulkSchemasSelected}
              onSchemasSelectedChange={setBulkSchemasSelected}
              bulkConfig={bulkConfig}
              onConfigChange={(patch) => setBulkConfig({ ...bulkConfig, ...patch })}
              bulkTargetTables={bulkTargetTables}
              tablesCount={tables.length}
              bulkScopeConflict={bulkScopeConflict}
              bulkCanStart={bulkCanStart}
              onClose={onClose}
              onStart={runBulkDescribe}
            />
          )}

          {bulkEntries.length > 0 && (
            <MetadataBulkDescribeProgress
              t={t}
              bulkEntries={bulkEntries}
              bulkEntriesDisplay={bulkEntriesDisplay}
              bulkRunning={bulkRunning}
              bulkSummary={bulkSummary}
              activeDescribeBatchJob={activeDescribeBatchJob}
              onClose={onClose}
              onCancelBulk={onCancelBulk}
            />
          )}
        </div>
      </section>
    </div>
  )
}
