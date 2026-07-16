import { Fragment } from 'react'

import type { Locale } from '../../i18n'
import { LOCALE_OPTIONS, SUPPORTED_LOCALES, type useT } from '../../i18n'
import { buttonClass, iconBtnClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import {
  metadataEmptyHintClass,
  metadataFilterEmptyRowClass,
  metadataLangTabClass,
  metadataLangTabsClass,
  metadataRowActionClass,
  metadataRowActionLabelClass,
  metadataTableColActionsClass,
  metadataTableColDescClass,
  metadataTableColNameClass,
  metadataTableColTypeClass,
  metadataTableRowClass,
  metadataToolbarActionsClass,
  metadataToolbarClass,
  metadataToolbarLangGroupClass,
  metadataToolbarTitleClass,
  metadataToolbarTopRowClass,
  metadataTypeBadgeClass,
  resultsTableMetadataListClass,
  resultsTableScrollClass,
} from '../../lib/tableClasses'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { LoadingScreen } from '../ui/LoadingScreen'
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
  openTableId,
  columns,
  editing,
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
  onSaveDisplayExpression,
}: {
  t: ReturnType<typeof useT>
  locale: Locale
  editLocale: Locale
  onEditLocaleChange: (loc: Locale) => void
  tables: TableRow[]
  filteredTables: TableRow[]
  tablesLoading: boolean
  loading: boolean
  openTableId: string | null
  columns: ColumnRow[]
  editing: MetadataEditingState | null
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
  onSaveDisplayExpression: (tab: TableRow, expr: string) => Promise<boolean>
}) {
  return (
    <div className={cardClass()}>
      <div className={metadataToolbarClass}>
        <div className={metadataToolbarTopRowClass}>
          <h2 className={metadataToolbarTitleClass}>
            {t('metadata.tables')} ({filteredTables.length}
            {filteredTables.length !== tables.length ? ` / ${tables.length}` : ''})
          </h2>
          <div className={metadataToolbarActionsClass}>
            <div className={metadataToolbarLangGroupClass}>
              <div
                className={metadataLangTabsClass}
                role="tablist"
                aria-label={t('metadata.lang_tabs_aria')}
              >
                {SUPPORTED_LOCALES.map((loc) => (
                  <button
                    key={loc}
                    type="button"
                    role="tab"
                    aria-selected={editLocale === loc}
                    className={metadataLangTabClass(editLocale === loc)}
                    onClick={() => onEditLocaleChange(loc)}
                  >
                    {LOCALE_OPTIONS[loc].short}
                  </button>
                ))}
              </div>
            </div>
            {tables.length > 0 && (
              <button
                type="button"
                className={cn(
                  buttonClass('secondary', { size: 'sm' }),
                  'metadata-toolbar-action-btn shrink-0 px-2.5 py-1 text-[0.75rem] whitespace-nowrap',
                )}
                onClick={onBulkOpen}
                disabled={bulkRunning || !!activeDescribeBatchJob}
              >
                {t('metadata.bulk_ai_btn')}
              </button>
            )}
          </div>
        </div>
      </div>
      {tablesLoading && tables.length === 0 ? (
        <LoadingScreen minHeight="150px" />
      ) : tables.length === 0 && !loading ? (
        <p className={metadataEmptyHintClass}>
          {t('metadata.no_tables_before')}
          <strong>{t('datasources.sync')}</strong>
          {t('metadata.no_tables_after')}
        </p>
      ) : null}

      {(!tablesLoading || tables.length > 0) && tables.length > 0 && (
        <div className={cn(resultsTableScrollClass, 'mt-2')}>
          <table className={resultsTableMetadataListClass()} lang={locale}>
            <colgroup>
              <col className={metadataTableColNameClass} />
              <col className={metadataTableColTypeClass} />
              <col className={metadataTableColDescClass} />
              <col className={metadataTableColActionsClass} />
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
                  <td colSpan={4} className={metadataFilterEmptyRowClass}>
                    {t('metadata.filter_no_match')}
                  </td>
                </tr>
              )}
              {filteredTables.map((tab) => (
                <Fragment key={tab.id}>
                  <tr
                    className={cn(
                      metadataTableRowClass(openTableId === tab.id),
                      'border-border border-b last:border-b-0',
                      'odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even) hover:bg-(--table-stripe-hover)',
                    )}
                  >
                    <td>
                      <button
                        type="button"
                        className={cn(iconBtnClass, 'text-caption gap-[0.35rem]')}
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
                        <span className="text-foreground-muted inline-block w-[0.7rem] text-[0.7rem]">
                          {openTableId === tab.id ? '▼' : '▶'}
                        </span>
                        {tab.schema_name}.{tab.table_name}
                      </button>
                    </td>
                    <td className="metadata-col-type">
                      <span
                        className={metadataTypeBadgeClass(
                          tab.table_type.toUpperCase().includes('VIEW'),
                        )}
                      >
                        {tab.table_type}
                      </span>
                    </td>
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
                        className={metadataRowActionClass}
                        onClick={() => onDescribeOpen(tab)}
                        aria-label={t('metadata.btn_ai_describe_aria', {
                          name: `${tab.schema_name}.${tab.table_name}`,
                        })}
                        title={t('metadata.btn_ai_describe')}
                      >
                        <span aria-hidden="true">✨</span>
                        <span className={metadataRowActionLabelClass}>
                          {t('metadata.btn_ai_describe')}
                        </span>
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
                      onSaveDisplayExpression={onSaveDisplayExpression}
                      t={t}
                    />
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
