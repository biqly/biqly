import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
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
  relationCount,
  bulkIncludeRelations,
  onToggleRelations,
  bulkSchemaRestrict,
  onSchemaRestrictAll,
  onSchemaRestrictPick,
  schemaOptions,
  bulkSchemasSelected,
  onSchemasSelectedChange,
  bulkConfig,
  onConfigChange,
  bulkTargetTables,
  bulkScopeCount,
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
  relationCount: number
  bulkIncludeRelations: boolean
  onToggleRelations: () => void
  bulkSchemaRestrict: boolean
  onSchemaRestrictAll: () => void
  onSchemaRestrictPick: () => void
  schemaOptions: string[]
  bulkSchemasSelected: string[]
  onSchemasSelectedChange: (schemas: string[]) => void
  bulkConfig: { sample_size: number; skip_existing: boolean }
  onConfigChange: (patch: Partial<{ sample_size: number; skip_existing: boolean }>) => void
  bulkTargetTables: TableRow[]
  bulkScopeCount: number
  tablesCount: number
  bulkScopeConflict: { message: string; schemas?: string } | null
  bulkCanStart: boolean
  onClose: () => void
  onStart: () => void
}) {
  return (
    <>
      <p className="text-foreground-muted m-0 text-[0.78rem] leading-[1.45]">
        {t('metadata.bulk_lede')}
      </p>
      <div className="grid grid-cols-1 items-stretch gap-[0.65rem] sm:grid-cols-2">
        <fieldset
          className={
            'border-border bg-card-raised m-0 min-w-0 rounded-lg border p-[0.55rem_0.65rem_0.65rem]'
          }
        >
          <legend className="text-foreground-faint px-1 py-0 text-[0.62rem] font-extrabold tracking-[0.07em] uppercase">
            {t('metadata.bulk_legend_types')}
          </legend>
          <div
            className="mt-[0.4rem] flex flex-wrap gap-[0.35rem]"
            role="group"
            aria-label={t('metadata.bulk_aria_types')}
          >
            {typeOptions.map((ty) => (
              <button
                key={ty}
                type="button"
                className={cn(
                  'bg-card text-foreground-muted hover:border-border-strong hover:text-foreground inline-flex cursor-pointer items-baseline gap-[0.35rem] rounded-full border p-[0.28rem_0.55rem] text-[0.75rem] leading-[1.2] transition-[background,border-color,color] duration-120',
                  bulkTypeEnabled[ty] === true
                    ? 'border-border-strong bg-card-raised text-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_35%,transparent)]'
                    : 'border-border',
                )}
                aria-pressed={bulkTypeEnabled[ty] === true}
                onClick={() => onToggleType(ty)}
              >
                <span className="font-semibold whitespace-nowrap">{objectTypeLabel(ty, t)}</span>
                <span
                  className={cn(
                    'text-[0.65rem] font-medium tracking-[0.04em] uppercase',
                    bulkTypeEnabled[ty] === true ? 'text-accent' : 'text-foreground-faint',
                  )}
                >
                  {ty}
                </span>
              </button>
            ))}
            {relationCount > 0 && (
              <button
                type="button"
                className={cn(
                  'bg-card text-foreground-muted hover:border-border-strong hover:text-foreground inline-flex cursor-pointer items-baseline gap-[0.35rem] rounded-full border p-[0.28rem_0.55rem] text-[0.75rem] leading-[1.2] transition-[background,border-color,color] duration-120',
                  bulkIncludeRelations
                    ? 'border-border-strong bg-card-raised text-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_35%,transparent)]'
                    : 'border-border',
                )}
                aria-pressed={bulkIncludeRelations}
                onClick={onToggleRelations}
              >
                <span className="font-semibold whitespace-nowrap">
                  {t('metadata.bulk_type_relations')}
                </span>
                <span
                  className={cn(
                    'text-[0.65rem] font-medium tracking-[0.04em] uppercase',
                    bulkIncludeRelations ? 'text-accent' : 'text-foreground-faint',
                  )}
                >
                  JOIN
                </span>
              </button>
            )}
          </div>
          {!bulkHasObjectType && !bulkIncludeRelations && (
            <p className={legacyFeedbackClass('text-error mx-0 mt-[0.4rem] mb-0 text-[0.74rem]')}>
              {t('metadata.bulk_warn_pick_type')}
            </p>
          )}
        </fieldset>
        <fieldset
          className={
            'border-border bg-card-raised m-0 min-w-0 rounded-lg border p-[0.55rem_0.65rem_0.65rem]'
          }
        >
          <legend className="text-foreground-faint px-1 py-0 text-[0.62rem] font-extrabold tracking-[0.07em] uppercase">
            {t('metadata.bulk_legend_schemas')}
          </legend>
          <div
            className={`border-border divide-border mt-[0.4rem] flex w-fit max-w-full divide-x overflow-hidden rounded-[7px] border`}
            role="group"
            aria-label={t('metadata.bulk_aria_schema_scope')}
          >
            <button
              type="button"
              className={
                !bulkSchemaRestrict
                  ? 'text-foreground m-0 cursor-pointer border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] font-semibold transition-[background,color] duration-120'
                  : 'text-foreground-muted hover:text-foreground hover:bg-card m-0 cursor-pointer border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] transition-[background,color] duration-120'
              }
              onClick={onSchemaRestrictAll}
            >
              {t('metadata.bulk_all_schemas')}
            </button>
            <button
              type="button"
              className={
                bulkSchemaRestrict
                  ? 'text-foreground m-0 cursor-pointer border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] font-semibold transition-[background,color] duration-120'
                  : 'text-foreground-muted hover:text-foreground hover:bg-card m-0 cursor-pointer border-0 bg-transparent p-[0.32rem_0.75rem] text-[0.74rem] transition-[background,color] duration-120'
              }
              onClick={onSchemaRestrictPick}
            >
              {t('metadata.bulk_pick_schemas')}
            </button>
          </div>
          <div
            className={cn(
              'bg-card-raised mt-[0.45rem] min-h-18 rounded-md border p-[0.45rem_0.5rem]',
              bulkSchemaRestrict ? 'border-border border-solid' : 'border-border border-dashed',
            )}
          >
            {!bulkSchemaRestrict ? (
              <p className="text-foreground-faint m-0 p-[0.35rem_0.15rem] text-[0.72rem] leading-[1.4]">
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
                <div className="mt-[0.35rem] flex flex-wrap items-center gap-[0.3rem_0.5rem]">
                  <button
                    type="button"
                    className={buttonClass('ghost', { size: 'sm' })}
                    onClick={() => onSchemasSelectedChange([...schemaOptions])}
                  >
                    {t('metadata.bulk_select_all')}
                  </button>
                  <button
                    type="button"
                    className={buttonClass('ghost', { size: 'sm' })}
                    onClick={() => onSchemasSelectedChange([])}
                  >
                    {t('metadata.bulk_select_none')}
                  </button>
                  <span className="text-foreground-faint text-[0.68rem]">
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
            className="text-foreground-faint text-[0.68rem] font-bold tracking-wider uppercase"
            htmlFor="bulk-sample-size"
          >
            {t('metadata.bulk_sample_rows')}
          </label>
          <input
            id="bulk-sample-size"
            type="number"
            min={1}
            max={100}
            className="w-17 p-[0.3rem_0.45rem] text-[0.8rem]"
            value={bulkConfig.sample_size}
            onChange={(e) => onConfigChange({ sample_size: Number(e.target.value) })}
          />
        </div>
        <label
          className="text-foreground-muted m-0 inline-flex cursor-pointer items-center gap-[0.45rem] pb-[0.15rem] text-[0.78rem]"
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
      <div className="border-border border-t px-0 pt-[0.35rem] pb-0">
        <span className="text-foreground-faint text-[0.76rem]">
          {t('metadata.bulk_scope_objects')}{' '}
          <strong className="text-foreground font-[650]">{bulkScopeCount}</strong>{' '}
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
          className={legacyFeedbackClass('text-error mx-0 mt-[0.4rem] mb-0 text-[0.74rem]')}
          role="status"
        >
          {bulkScopeConflict.message}{' '}
          {bulkScopeConflict.schemas
            ? t('metadata.already_running_schemas', { schemas: bulkScopeConflict.schemas })
            : null}
        </p>
      )}
      <div className={modalActionsClass()}>
        <button type="button" className={buttonClass('ghost', { size: 'sm' })} onClick={onClose}>
          {t('metadata.bulk_cancel')}
        </button>
        <button
          type="button"
          className={buttonClass('secondary', { size: 'sm' })}
          onClick={onStart}
          disabled={!bulkCanStart}
        >
          {t('metadata.bulk_start', { count: bulkScopeCount })}
        </button>
      </div>
    </>
  )
}
