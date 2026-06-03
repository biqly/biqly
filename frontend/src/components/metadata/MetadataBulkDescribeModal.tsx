import '../../styles/bulk-describe.css'

import { useEffect, useMemo, useState } from 'react'

import { fetchDescribeBatchConflict } from '../../api/describeBatchConflict'
import { useT } from '../../i18n'
import type { AIRuntimeSettings } from '../../types/ai'
import type { TableRow } from '../../types/semantic'
import { ModelBadgeRow } from '../ui/ModelBadgeRow'
import { MultiSelect } from '../ui/MultiSelect'
import {
  type BulkEntry,
  BulkProgressHeader,
  BulkQueuePreview,
  BulkStatusBadge,
  objectTypeLabel,
  sortBulkEntriesForDisplay,
} from './bulkProgress'

export interface MetadataBulkDescribeModalProps {
  open: boolean
  onClose: () => void
  datasourceId: string
  tables: TableRow[]
  schemaOptions: string[]
  typeOptions: string[]
  aiRuntime: AIRuntimeSettings | null
  // Effective per-user describe model label (preference resolved); falls back to
  // the global default from aiRuntime when not set.
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
  const [bulkConfig, setBulkConfig] = useState({ sample_size: 10, skip_existing: true })
  const [bulkTypeEnabled, setBulkTypeEnabled] = useState<Record<string, boolean>>({})
  const [bulkSchemaRestrict, setBulkSchemaRestrict] = useState(false)
  const [bulkSchemasSelected, setBulkSchemasSelected] = useState<string[]>([])
  const [bulkScopeConflict, setBulkScopeConflict] = useState<{
    message: string
    schemas?: string
  } | null>(null)
  const dbManaged = aiRuntime?.db_managed === true
  const activeDescribe = dbManaged
    ? aiRuntime?.active_models?.find((m) => m.purpose === 'describe')
    : undefined
  const activeTranslation = dbManaged
    ? aiRuntime?.active_models?.find((m) => m.purpose === 'translation')
    : undefined

  useEffect(() => {
    if (!open) {
      return
    }
    setBulkTypeEnabled(Object.fromEntries(typeOptions.map((ty) => [ty, true])))
    setBulkSchemaRestrict(false)
    setBulkSchemasSelected([])
  }, [open, typeOptions])

  const bulkTargetTables = useMemo(() => {
    const restrictTypes = Object.keys(bulkTypeEnabled).length > 0
    return tables.filter((tab) => {
      if (restrictTypes && !bulkTypeEnabled[tab.table_type]) {
        return false
      }
      if (bulkSchemaRestrict) {
        if (bulkSchemasSelected.length === 0) {
          return false
        }
        if (!bulkSchemasSelected.includes(tab.schema_name)) {
          return false
        }
      }
      return true
    })
  }, [tables, bulkTypeEnabled, bulkSchemaRestrict, bulkSchemasSelected])

  const bulkHasObjectType =
    typeOptions.length === 0 || typeOptions.some((ty) => bulkTypeEnabled[ty])

  const bulkScopeSchemas = useMemo(() => {
    if (bulkSchemaRestrict) {
      return [...bulkSchemasSelected].sort((a, b) => a.localeCompare(b))
    }
    return [...new Set(bulkTargetTables.map((tab) => tab.schema_name))].sort((a, b) =>
      a.localeCompare(b),
    )
  }, [bulkSchemaRestrict, bulkSchemasSelected, bulkTargetTables])

  useEffect(() => {
    if (!datasourceId || bulkScopeSchemas.length === 0) {
      setBulkScopeConflict(null)
      return
    }
    let cancelled = false
    void fetchDescribeBatchConflict(datasourceId, bulkScopeSchemas).then((res) => {
      if (cancelled) {
        return
      }
      if (res?.conflict) {
        setBulkScopeConflict({
          message: t('metadata.already_running'),
          schemas: res.scope_schemas?.join(', ') ?? bulkScopeSchemas.join(', '),
        })
      } else {
        setBulkScopeConflict(null)
      }
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId, bulkScopeSchemas, t])

  const bulkCanStart =
    bulkTargetTables.length > 0 &&
    bulkHasObjectType &&
    (!bulkSchemaRestrict || bulkSchemasSelected.length > 0) &&
    !bulkScopeConflict &&
    !bulkRunning

  const bulkEntriesDisplay = useMemo(
    () => (bulkEntries.length > 0 ? sortBulkEntriesForDisplay(bulkEntries) : []),
    [bulkEntries],
  )

  const runBulkDescribe = () => {
    const targets = bulkTargetTables
    if (!datasourceId || targets.length === 0 || bulkScopeConflict) {
      return
    }
    setBulkScopeConflict(null)
    onStartBulk({
      targets,
      sampleSize: bulkConfig.sample_size,
      skipExisting: bulkConfig.skip_existing,
      onConflict: (message) => {
        setBulkScopeConflict({ message })
      },
      onFinished: () => {
        onRefreshTables()
      },
    })
  }

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
        className="modal-card modal-card--bulk-describe"
        role="dialog"
        aria-modal="true"
        aria-labelledby="bulk-metadata-title"
      >
        <header className="modal-header modal-header--compact">
          <div>
            <h2 id="bulk-metadata-title" className="bulk-modal-title">
              {t('metadata.bulk_modal_title')}
            </h2>
            <p className="bulk-modal-subtitle">{t('metadata.bulk_modal_subtitle')}</p>
            <ModelBadgeRow
              primaryLabel={t('metadata.describe_badge_label')}
              primaryModel={describeModel || aiRuntime?.llm_model}
              primaryNote={dbManaged ? activeDescribe?.provider_name : undefined}
              translationModel={
                aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined
              }
              translationNote={dbManaged ? activeTranslation?.provider_name : undefined}
            />
          </div>
          <button
            type="button"
            className="modal-close"
            aria-label={t('metadata.bulk_close_aria')}
            onClick={onClose}
          >
            ×
          </button>
        </header>
        <div className={`modal-body${bulkEntries.length > 0 ? ' modal-body--scroll' : ''}`}>
          {bulkEntries.length === 0 && !bulkRunning && (
            <>
              <p className="bulk-lede">{t('metadata.bulk_lede')}</p>
              <div className="bulk-panel-grid">
                <fieldset className="bulk-fieldset">
                  <legend className="bulk-legend">{t('metadata.bulk_legend_types')}</legend>
                  <div
                    className="bulk-pill-row"
                    role="group"
                    aria-label={t('metadata.bulk_aria_types')}
                  >
                    {typeOptions.map((ty) => (
                      <button
                        key={ty}
                        type="button"
                        className={`bulk-pill${bulkTypeEnabled[ty] === true ? ' bulk-pill--on' : ''}`}
                        aria-pressed={bulkTypeEnabled[ty] === true}
                        onClick={() => setBulkTypeEnabled((prev) => ({ ...prev, [ty]: !prev[ty] }))}
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
                      onClick={() => {
                        setBulkSchemaRestrict(false)
                        setBulkSchemasSelected([])
                      }}
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
                      onClick={() => {
                        setBulkSchemaRestrict(true)
                        setBulkSchemasSelected((prev) =>
                          prev.length > 0 ? prev : [...schemaOptions],
                        )
                      }}
                    >
                      {t('metadata.bulk_pick_schemas')}
                    </button>
                  </div>
                  <div
                    className={`bulk-schema-box${bulkSchemaRestrict ? ' bulk-schema-box--active' : ''}`}
                  >
                    {!bulkSchemaRestrict ? (
                      <p className="bulk-schema-placeholder">
                        {t('metadata.bulk_schema_all_hint')}
                      </p>
                    ) : (
                      <>
                        <MultiSelect
                          id="bulk-schema-multiselect"
                          display="inline"
                          className="bulk-schema-multiselect"
                          ariaLabel={t('metadata.bulk_aria_schemas_pick')}
                          value={bulkSchemasSelected}
                          onChange={setBulkSchemasSelected}
                          maxHeight={Math.min(288, Math.max(144, schemaOptions.length * 36))}
                          options={schemaOptions.map((s) => ({ value: s, label: s }))}
                        />
                        <div className="bulk-schema-multiselect-tools">
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => setBulkSchemasSelected([...schemaOptions])}
                          >
                            {t('metadata.bulk_select_all')}
                          </button>
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => setBulkSchemasSelected([])}
                          >
                            {t('metadata.bulk_select_none')}
                          </button>
                          <span className="bulk-schema-hint">
                            {t('metadata.bulk_multiselect_hint')}
                          </span>
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
                    onChange={(e) =>
                      setBulkConfig({ ...bulkConfig, sample_size: Number(e.target.value) })
                    }
                  />
                </div>
                <label className="bulk-skip-label" htmlFor="bulk-skip-existing">
                  <input
                    id="bulk-skip-existing"
                    type="checkbox"
                    checked={bulkConfig.skip_existing}
                    onChange={(e) =>
                      setBulkConfig({ ...bulkConfig, skip_existing: e.target.checked })
                    }
                  />
                  <span>{t('metadata.bulk_skip_existing')}</span>
                </label>
              </div>
              <div className="bulk-scope-footer">
                <span className="bulk-scope-stat">
                  {t('metadata.bulk_scope_objects')} <strong>{bulkTargetTables.length}</strong>{' '}
                  {t('metadata.bulk_scope_suffix')}
                  {bulkTargetTables.length !== tables.length && (
                    <span className="bulk-scope-of">
                      {t('metadata.bulk_scope_total', { total: tables.length })}
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
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={runBulkDescribe}
                  disabled={!bulkCanStart}
                >
                  {t('metadata.bulk_start', { count: bulkTargetTables.length })}
                </button>
              </div>
            </>
          )}

          {bulkEntries.length > 0 && (
            <>
              <BulkProgressHeader
                entries={bulkEntries}
                running={bulkRunning}
                summary={bulkSummary}
              />
              {bulkRunning && (
                <BulkQueuePreview
                  entries={bulkEntries}
                  progress={activeDescribeBatchJob?.progress_json ?? null}
                />
              )}
              <div className="bulk-describe-scroll">
                <table className="results-table results-table--dense" style={{ margin: 0 }}>
                  <thead>
                    <tr>
                      <th className="bulk-col-idx">{t('metadata.bulk_table_idx')}</th>
                      <th>{t('metadata.bulk_table_name')}</th>
                      <th className="bulk-col-status">{t('metadata.bulk_table_status')}</th>
                      <th>{t('metadata.bulk_table_detail')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {bulkEntriesDisplay.map((e, idx) => (
                      <tr key={`${e.schema}.${e.table}`}>
                        <td className="bulk-col-idx">{idx + 1}</td>
                        <td className="bulk-col-name">
                          <code>
                            {e.schema}.{e.table}
                          </code>
                        </td>
                        <td className="bulk-col-status">
                          <BulkStatusBadge status={e.status} />
                        </td>
                        <td className="bulk-col-detail" style={{ color: 'var(--text-secondary)' }}>
                          <span className="bulk-col-detail-inner" title={e.message}>
                            {e.message || (e.status === 'pending' ? t('common.em_dash') : '')}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="modal-actions">
                {bulkRunning ? (
                  <>
                    <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
                      {t('metadata.bulk_run_background')}
                    </button>
                    <button type="button" className="btn btn-ghost btn-sm" onClick={onCancelBulk}>
                      {t('metadata.bulk_stop_after')}
                    </button>
                  </>
                ) : (
                  <button type="button" className="btn btn-sm" onClick={onClose}>
                    {t('metadata.bulk_close_btn')}
                  </button>
                )}
              </div>
            </>
          )}
        </div>
      </section>
    </div>
  )
}
