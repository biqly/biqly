import type { useT } from '../../i18n'
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
      <p className="bulk-lede">{t('metadata.bulk_lede')}</p>
      <div className="bulk-panel-grid">
        <fieldset className="bulk-fieldset">
          <legend className="bulk-legend">{t('metadata.bulk_legend_types')}</legend>
          <div className="bulk-pill-row" role="group" aria-label={t('metadata.bulk_aria_types')}>
            {typeOptions.map((ty) => (
              <button
                key={ty}
                type="button"
                className={`bulk-pill${bulkTypeEnabled[ty] === true ? ' bulk-pill--on' : ''}`}
                aria-pressed={bulkTypeEnabled[ty] === true}
                onClick={() => onToggleType(ty)}
              >
                <span className="bulk-pill-label">{objectTypeLabel(ty, t)}</span>
                <span className="bulk-pill-code">{ty}</span>
              </button>
            ))}
          </div>
          {!bulkHasObjectType && (
            <p className="bulk-modal-warn">{t('metadata.bulk_warn_pick_type')}</p>
          )}
        </fieldset>
        <fieldset className="bulk-fieldset">
          <legend className="bulk-legend">{t('metadata.bulk_legend_schemas')}</legend>
          <div
            className="bulk-segmented"
            role="group"
            aria-label={t('metadata.bulk_aria_schema_scope')}
          >
            <button
              type="button"
              className={
                !bulkSchemaRestrict
                  ? 'bulk-segmented__btn bulk-segmented__btn--active'
                  : 'bulk-segmented__btn'
              }
              onClick={onSchemaRestrictAll}
            >
              {t('metadata.bulk_all_schemas')}
            </button>
            <button
              type="button"
              className={
                bulkSchemaRestrict
                  ? 'bulk-segmented__btn bulk-segmented__btn--active'
                  : 'bulk-segmented__btn'
              }
              onClick={onSchemaRestrictPick}
            >
              {t('metadata.bulk_pick_schemas')}
            </button>
          </div>
          <div className={`bulk-schema-box${bulkSchemaRestrict ? ' bulk-schema-box--active' : ''}`}>
            {!bulkSchemaRestrict ? (
              <p className="bulk-schema-placeholder">{t('metadata.bulk_schema_all_hint')}</p>
            ) : (
              <>
                <MultiSelect
                  id="bulk-schema-multiselect"
                  display="inline"
                  className="bulk-schema-multiselect"
                  ariaLabel={t('metadata.bulk_aria_schemas_pick')}
                  value={bulkSchemasSelected}
                  onChange={onSchemasSelectedChange}
                  maxHeight={Math.min(288, Math.max(144, schemaOptions.length * 36))}
                  options={schemaOptions.map((s) => ({ value: s, label: s }))}
                />
                <div className="bulk-schema-multiselect-tools">
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onSchemasSelectedChange([...schemaOptions])}
                  >
                    {t('metadata.bulk_select_all')}
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onSchemasSelectedChange([])}
                  >
                    {t('metadata.bulk_select_none')}
                  </button>
                  <span className="bulk-schema-hint">{t('metadata.bulk_multiselect_hint')}</span>
                </div>
              </>
            )}
          </div>
        </fieldset>
      </div>
      <div className="bulk-options-row">
        <div className="form-group bulk-opt-field">
          <label className="bulk-opt-label" htmlFor="bulk-sample-size">
            {t('metadata.bulk_sample_rows')}
          </label>
          <input
            id="bulk-sample-size"
            type="number"
            min={1}
            max={100}
            className="bulk-opt-input"
            value={bulkConfig.sample_size}
            onChange={(e) => onConfigChange({ sample_size: Number(e.target.value) })}
          />
        </div>
        <label className="bulk-skip-label" htmlFor="bulk-skip-existing">
          <input
            id="bulk-skip-existing"
            type="checkbox"
            checked={bulkConfig.skip_existing}
            onChange={(e) => onConfigChange({ skip_existing: e.target.checked })}
          />
          <span>{t('metadata.bulk_skip_existing')}</span>
        </label>
      </div>
      <div className="bulk-scope-footer">
        <span className="bulk-scope-stat">
          {t('metadata.bulk_scope_objects')} <strong>{bulkTargetTables.length}</strong>{' '}
          {t('metadata.bulk_scope_suffix')}
          {bulkTargetTables.length !== tablesCount && (
            <span className="bulk-scope-of">
              {t('metadata.bulk_scope_total', { total: tablesCount })}
            </span>
          )}
        </span>
      </div>
      {bulkScopeConflict && (
        <p className="bulk-modal-warn" role="status">
          {bulkScopeConflict.message}{' '}
          {bulkScopeConflict.schemas
            ? t('metadata.already_running_schemas', { schemas: bulkScopeConflict.schemas })
            : null}
        </p>
      )}
      <div className="modal-actions">
        <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
          {t('metadata.bulk_cancel')}
        </button>
        <button type="button" className="btn btn-sm" onClick={onStart} disabled={!bulkCanStart}>
          {t('metadata.bulk_start', { count: bulkTargetTables.length })}
        </button>
      </div>
    </>
  )
}
