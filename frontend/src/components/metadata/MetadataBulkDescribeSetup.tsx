import type { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { legacyFormClass } from '../../lib/formClasses'
import { modalActionsClass } from '../../lib/modalClasses'
import type { TableRow } from '../../types/semantic'
import { MultiSelect } from '../ui/MultiSelect'
import { objectTypeLabel } from './bulkProgress'
export function MetadataBulkDescribeSetup({
  t,
  typeOptions,
  bulkTypeEnabled,
  onToggleType,
  bulkHasObjectType,
  bulkSchemaRestrict,
  onSchemaRestrictAll,
  onSchemaRestrictPick,
  schemaOptions,
  bulkSchemasSelected,
  onSchemasSelectedChange,
  bulkConfig,
  onConfigChange,
  bulkTargetTables,
  tablesCount,
  bulkScopeConflict,
  bulkCanStart,
  onClose,
  onStart,
}: {
  t: ReturnType<typeof useT>
  typeOptions: string[]
  bulkTypeEnabled: Record<string, boolean>
  onToggleType: (ty: string) => void
  bulkHasObjectType: boolean
  bulkSchemaRestrict: boolean
  onSchemaRestrictAll: () => void
  onSchemaRestrictPick: () => void
  schemaOptions: string[]
  bulkSchemasSelected: string[]
  onSchemasSelectedChange: (schemas: string[]) => void
  bulkConfig: { sample_size: number; skip_existing: boolean }
  onConfigChange: (patch: Partial<{ sample_size: number; skip_existing: boolean }>) => void
  bulkTargetTables: TableRow[]
  tablesCount: number
  bulkScopeConflict: { message: string; schemas?: string } | null
  bulkCanStart: boolean
  onClose: () => void
  onStart: () => void
}) {
  return (
    <>
      <p className="m-0 text-[0.78rem] leading-[1.45] text-foreground-muted">
        {t('metadata.bulk_lede')}
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-[0.65rem] items-stretch">
        <fieldset
          className={legacyCardClass(
            'm-0 min-w-0 p-[0.55rem_0.65rem_0.65rem] border border-border rounded-lg bg-card-raised',
          )}
        >
          <legend className="px-[0.25rem] py-0 text-[0.62rem] font-[800] tracking-[0.07em] uppercase text-foreground-faint">
            {t('metadata.bulk_legend_types')}
          </legend>
          <div
            className="flex flex-wrap gap-[0.35rem] mt-[0.4rem]"
            role="group"
            aria-label={t('metadata.bulk_aria_types')}
          >
            {typeOptions.map((ty) => (
              <button
                key={ty}
                type="button"
                className={cn(
                  legacyCardClass(
                    'inline-flex items-baseline gap-[0.35rem] border bg-card text-foreground-muted p-[0.28rem_0.55rem] rounded-full text-[0.75rem] leading-[1.2] cursor-pointer transition-[background,border-color,color] duration-120 hover:border-border-strong hover:text-foreground',
                  ),
                  bulkTypeEnabled[ty] === true
                    ? 'bg-card-raised border-border-strong text-foreground shadow-[inset_0_0_0_1px_rgba(91,142,255,0.35)]'
                    : 'border-border',
                )}
                aria-pressed={bulkTypeEnabled[ty] === true}
                onClick={() => onToggleType(ty)}
              >
                <span className="font-semibold whitespace-nowrap">{objectTypeLabel(ty, t)}</span>
                <span
                  className={`text-[0.65rem] font-medium uppercase tracking-[0.04em] ${
                    bulkTypeEnabled[ty] === true ? 'text-blue-300/85' : 'text-foreground-faint'
                  }`}
                >
                  {ty}
                </span>
              </button>
            ))}
          </div>
          {!bulkHasObjectType && (
            <p className={legacyFeedbackClass('mt-[0.4rem] mx-0 mb-0 text-[0.74rem] text-error')}>
              {t('metadata.bulk_warn_pick_type')}
            </p>
          )}
        </fieldset>
        <fieldset
          className={legacyCardClass(
            'm-0 min-w-0 p-[0.55rem_0.65rem_0.65rem] border border-border rounded-lg bg-card-raised',
          )}
        >
          <legend className="px-[0.25rem] py-0 text-[0.62rem] font-[800] tracking-[0.07em] uppercase text-foreground-faint">
            {t('metadata.bulk_legend_schemas')}
          </legend>
          <div
            className={`flex mt-[0.4rem] rounded-[7px] border border-border overflow-hidden w-fit max-w-full divide-x divide-border`}
            role="group"
            aria-label={t('metadata.bulk_aria_schema_scope')}
          >
            <button
              type="button"
              className={
                !bulkSchemaRestrict
                  ? 'm-0 border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] text-foreground cursor-pointer transition-[background,color] duration-120 bg-card font-semibold'
                  : 'm-0 border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] text-foreground-muted cursor-pointer transition-[background,color] duration-120 hover:text-foreground hover:bg-card'
              }
              onClick={onSchemaRestrictAll}
            >
              {t('metadata.bulk_all_schemas')}
            </button>
            <button
              type="button"
              className={
                bulkSchemaRestrict
                  ? 'm-0 border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] text-foreground cursor-pointer transition-[background,color] duration-120 bg-card font-semibold'
                  : 'm-0 border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] text-foreground-muted cursor-pointer transition-[background,color] duration-120 hover:text-foreground hover:bg-card'
              }
              onClick={onSchemaRestrictPick}
            >
              {t('metadata.bulk_pick_schemas')}
            </button>
          </div>
          <div
            className={`mt-[0.45rem] min-h-[4.5rem] rounded-md p-[0.45rem_0.5rem] bg-slate-900/25 border ${
              bulkSchemaRestrict
                ? 'border-solid border-slate-400/28'
                : 'border-dashed border-slate-400/25'
            }`}
          >
            {!bulkSchemaRestrict ? (
              <p className="m-0 text-[0.72rem] leading-[1.4] text-foreground-faint p-[0.35rem_0.15rem]">
                {t('metadata.bulk_schema_all_hint')}
              </p>
            ) : (
              <>
                <MultiSelect
                  id="bulk-schema-multiselect"
                  display="inline"
                  className="block w-full text-[0.74rem] [&_.ui-select-option]:text-[0.74rem]"
                  ariaLabel={t('metadata.bulk_aria_schemas_pick')}
                  value={bulkSchemasSelected}
                  onChange={onSchemasSelectedChange}
                  maxHeight={Math.min(288, Math.max(144, schemaOptions.length * 36))}
                  options={schemaOptions.map((s) => ({ value: s, label: s }))}
                />
                <div className="flex flex-wrap items-center gap-[0.3rem_0.5rem] mt-[0.35rem]">
                  <button
                    type="button"
                    className={legacyButtonClass('btn btn-ghost btn-sm')}
                    onClick={() => onSchemasSelectedChange([...schemaOptions])}
                  >
                    {t('metadata.bulk_select_all')}
                  </button>
                  <button
                    type="button"
                    className={legacyButtonClass('btn btn-ghost btn-sm')}
                    onClick={() => onSchemasSelectedChange([])}
                  >
                    {t('metadata.bulk_select_none')}
                  </button>
                  <span className="text-[0.68rem] text-foreground-faint">
                    {t('metadata.bulk_multiselect_hint')}
                  </span>
                </div>
              </>
            )}
          </div>
        </fieldset>
      </div>
      <div className="flex flex-wrap items-end gap-[0.75rem_1.25rem]">
        <div className={legacyFormClass('form-group mb-0 flex flex-col gap-[0.2rem]')}>
          <label
            className="text-[0.68rem] font-bold tracking-wider uppercase text-foreground-faint"
            htmlFor="bulk-sample-size"
          >
            {t('metadata.bulk_sample_rows')}
          </label>
          <input
            id="bulk-sample-size"
            type="number"
            min={1}
            max={100}
            className="w-[4.25rem] text-[0.8rem] p-[0.3rem_0.45rem]"
            value={bulkConfig.sample_size}
            onChange={(e) => onConfigChange({ sample_size: Number(e.target.value) })}
          />
        </div>
        <label
          className="inline-flex items-center gap-[0.45rem] m-0 text-[0.78rem] text-foreground-muted cursor-pointer pb-[0.15rem]"
          htmlFor="bulk-skip-existing"
        >
          <input
            id="bulk-skip-existing"
            type="checkbox"
            className="shrink-0"
            checked={bulkConfig.skip_existing}
            onChange={(e) => onConfigChange({ skip_existing: e.target.checked })}
          />
          <span>{t('metadata.bulk_skip_existing')}</span>
        </label>
      </div>
      <div className="pt-[0.35rem] px-0 pb-0 border-t border-slate-400/12">
        <span className="text-[0.76rem] text-foreground-faint">
          {t('metadata.bulk_scope_objects')}{' '}
          <strong className="text-foreground font-[650]">{bulkTargetTables.length}</strong>{' '}
          {t('metadata.bulk_scope_suffix')}
          {bulkTargetTables.length !== tablesCount && (
            <span className="opacity-90">
              {t('metadata.bulk_scope_total', { total: tablesCount })}
            </span>
          )}
        </span>
      </div>
      {bulkScopeConflict && (
        <p
          className={legacyFeedbackClass('mt-[0.4rem] mx-0 mb-0 text-[0.74rem] text-error')}
          role="status"
        >
          {bulkScopeConflict.message}{' '}
          {bulkScopeConflict.schemas
            ? t('metadata.already_running_schemas', { schemas: bulkScopeConflict.schemas })
            : null}
        </p>
      )}
      <div className={modalActionsClass()}>
        <button
          type="button"
          className={legacyButtonClass('btn btn-ghost btn-sm')}
          onClick={onClose}
        >
          {t('metadata.bulk_cancel')}
        </button>
        <button
          type="button"
          className={legacyButtonClass('btn btn-sm')}
          onClick={onStart}
          disabled={!bulkCanStart}
        >
          {t('metadata.bulk_start', { count: bulkTargetTables.length })}
        </button>
      </div>
    </>
  )
}
