import { Fragment } from 'react'

import type { Locale } from '../../i18n'
import { FALLBACK_LOCALE, LOCALE_OPTIONS, SUPPORTED_LOCALES, type useT } from '../../i18n'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Select } from '../ui/Select'
import { MetadataColumnPanel } from './MetadataColumnPanel'
import { MetadataDescriptionCell } from './MetadataDescriptionCell'
import type { MetadataEditingState } from './utils'

export function MetadataTablesPanel({
  t,
  locale,
  editLocale,
  onEditLocaleChange,
  tables,
  filteredTables,
  tablesLoading,
  loading,
  tableFilterSchema,
  tableFilterType,
  schemaOptions,
  typeOptions,
  openTableId,
  columns,
  editing,
  onSchemaFilterChange,
  onTypeFilterChange,
  onBulkOpen,
  bulkRunning,
  activeDescribeBatchJob,
  onToggleTable,
  onStartEditTable,
  onEditTableChange,
  onSaveDescription,
  onCancelEdit,
  onDescribeOpen,
  onStartEditColumn,
  onEditColumnChange,
}: {
  t: ReturnType<typeof useT>
  locale: Locale
  editLocale: Locale
  onEditLocaleChange: (loc: Locale) => void
  tables: TableRow[]
  filteredTables: TableRow[]
  tablesLoading: boolean
  loading: boolean
  tableFilterSchema: string
  tableFilterType: string
  schemaOptions: string[]
  typeOptions: string[]
  openTableId: string | null
  columns: ColumnRow[]
  editing: MetadataEditingState | null
  onSchemaFilterChange: (v: string) => void
  onTypeFilterChange: (v: string) => void
  onBulkOpen: () => void
  bulkRunning: boolean
  activeDescribeBatchJob: unknown
  onToggleTable: (tab: TableRow) => void
  onStartEditTable: (tab: TableRow) => void
  onEditTableChange: (id: string, value: string) => void
  onSaveDescription: () => void
  onCancelEdit: () => void
  onDescribeOpen: (tab: TableRow) => void
  onStartEditColumn: (c: ColumnRow) => void
  onEditColumnChange: (columnId: string, value: string) => void
}) {
  return (
    <div className="card">
      <div className="metadata-toolbar">
        <h2 className="metadata-toolbar__title">
          {t('metadata.tables')} ({filteredTables.length}
          {filteredTables.length !== tables.length ? ` / ${tables.length}` : ''})
        </h2>
        {tables.length > 0 && (
          <div className="metadata-table-filters metadata-table-filters--toolbar">
            <div className="metadata-filter-field">
              <Select
                id="metadata-filter-schema"
                size="sm"
                ariaLabel={t('metadata.filter_schema_aria')}
                value={tableFilterSchema}
                onChange={onSchemaFilterChange}
                options={[
                  { value: '', label: t('metadata.filter_all_schemas') },
                  ...schemaOptions.map((s) => ({ value: s, label: s })),
                ]}
              />
            </div>
            <div className="metadata-filter-field">
              <Select
                id="metadata-filter-type"
                size="sm"
                ariaLabel={t('metadata.filter_type_aria')}
                value={tableFilterType}
                onChange={onTypeFilterChange}
                options={[
                  { value: '', label: t('metadata.filter_all_types') },
                  ...typeOptions.map((ty) => ({ value: ty, label: ty })),
                ]}
              />
            </div>
          </div>
        )}
        <div className="metadata-toolbar__actions">
          <div
            className="metadata-lang-tabs"
            role="tablist"
            aria-label={t('metadata.lang_tabs_aria')}
          >
            {SUPPORTED_LOCALES.map((loc) => (
              <button
                key={loc}
                type="button"
                role="tab"
                aria-selected={editLocale === loc}
                className={`metadata-lang-tab${editLocale === loc ? ' metadata-lang-tab--active' : ''}`}
                onClick={() => onEditLocaleChange(loc)}
              >
                {LOCALE_OPTIONS[loc].short}
              </button>
            ))}
          </div>
          {editLocale !== FALLBACK_LOCALE && (
            <button
              type="button"
              className="metadata-hint-btn"
              aria-label={t('metadata.desc_lang_hint_aria')}
              title={t('metadata.desc_lang_tr_hint')}
            >
              i
            </button>
          )}
          {tables.length > 0 && (
            <button
              type="button"
              className="btn btn-sm"
              onClick={onBulkOpen}
              disabled={bulkRunning || !!activeDescribeBatchJob}
            >
              {t('metadata.bulk_ai_btn')}
            </button>
          )}
        </div>
      </div>
      {tablesLoading && tables.length === 0 ? (
        <LoadingScreen minHeight="150px" />
      ) : tables.length === 0 && !loading ? (
        <p className="metadata-empty-hint">
          {t('metadata.no_tables_before')}
          <strong>{t('datasources.sync')}</strong>
          {t('metadata.no_tables_after')}
        </p>
      ) : null}

      {(!tablesLoading || tables.length > 0) && tables.length > 0 && (
        <table className="results-table results-table--metadata-list" lang={locale}>
          <colgroup>
            <col className="metadata-cw-name" />
            <col className="metadata-cw-type" />
            <col className="metadata-cw-desc" />
            <col className="metadata-cw-actions" />
          </colgroup>
          <thead>
            <tr>
              <th scope="col" className="metadata-col-name">
                {t('metadata.col_table_name')}
              </th>
              <th scope="col" className="metadata-col-type">
                {t('metadata.col_object_type')}
              </th>
              <th scope="col" className="metadata-col-desc">
                {t('metadata.col_table_desc')}
              </th>
              <th scope="col" className="actions metadata-col-actions">
                {t('metadata.col_actions')}
              </th>
            </tr>
          </thead>
          <tbody>
            {filteredTables.length === 0 && tables.length > 0 && (
              <tr>
                <td
                  colSpan={4}
                  style={{
                    color: 'var(--text-secondary)',
                    fontSize: '0.85rem',
                    padding: '0.75rem',
                  }}
                >
                  {t('metadata.filter_no_match')}
                </td>
              </tr>
            )}
            {filteredTables.map((tab) => (
              <Fragment key={tab.id}>
                <tr
                  className={
                    openTableId === tab.id
                      ? 'metadata-table-row metadata-table-row--expanded'
                      : 'metadata-table-row'
                  }
                >
                  <td>
                    <button
                      type="button"
                      className="icon-btn"
                      aria-expanded={openTableId === tab.id}
                      aria-label={
                        openTableId === tab.id
                          ? t('metadata.aria_table_collapse', {
                              name: `${tab.schema_name}.${tab.table_name}`,
                            })
                          : t('metadata.aria_table_expand', {
                              name: `${tab.schema_name}.${tab.table_name}`,
                            })
                      }
                      onClick={() => onToggleTable(tab)}
                    >
                      <span className="chevron">{openTableId === tab.id ? '▼' : '▶'}</span>
                      {tab.schema_name}.{tab.table_name}
                    </button>
                  </td>
                  <td className="metadata-col-type">{tab.table_type}</td>
                  <MetadataDescriptionCell
                    kind="table"
                    entityId={tab.id}
                    description={tab.description}
                    editing={editing}
                    placeholder={t('metadata.placeholder_double_click')}
                    onStartEdit={() => onStartEditTable(tab)}
                    onChange={(value) => onEditTableChange(tab.id, value)}
                    onSave={onSaveDescription}
                    onCancel={onCancelEdit}
                  />
                  <td className="actions">
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => onDescribeOpen(tab)}
                    >
                      {t('metadata.btn_ai_describe')}
                    </button>
                  </td>
                </tr>
                {openTableId === tab.id && columns.length > 0 && (
                  <MetadataColumnPanel
                    table={tab}
                    columns={columns}
                    locale={locale}
                    editing={editing}
                    onStartEdit={onStartEditColumn}
                    onEditChange={onEditColumnChange}
                    onSave={onSaveDescription}
                    onCancelEdit={onCancelEdit}
                  />
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
