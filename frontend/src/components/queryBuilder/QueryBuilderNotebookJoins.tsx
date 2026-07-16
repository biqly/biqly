import type { TranslationKey } from '../../i18n'
import type { SemanticJoin } from '../../types/semantic'
import { joinDataTypesCompatible } from '../../utils/joinCompatibility'
import { joinTypeHintKey } from '../ui/joinType'
import { JoinTypeIcon } from '../ui/JoinTypeIcon'
import { Select } from '../ui/Select'
import { joinRelationshipCardinality } from './joinCardinality'
import type { TableOption } from './metadataModel'
import { metadataTableKey, splitMetadataTableKey } from './metadataModel'
import { NotebookStep } from './NotebookStep'
import {
  qbAddBtnClass,
  qbJoinCardinalityClass,
  qbJoinTypeClass,
  qbTagCloseClass,
} from './queryBuilderClasses'

interface ColumnOption {
  value: string
  label: string
  hint?: string
}
type ColumnOptionsByTable = Record<string, ColumnOption[]>
type ColumnTypesByTable = Record<string, Record<string, string>>

// Restricts ON-column options to types SQL-joinable with the column selected
// on the other side. The current selection always stays listed so a saved
// (incompatible) pair remains visible instead of silently blanking out.
function compatibleColumnOptions(
  options: ColumnOption[],
  columnTypes: Record<string, string> | undefined,
  otherSideType: string,
  selected: string,
): ColumnOption[] {
  if (!otherSideType || !columnTypes) {
    return options
  }
  return options.filter(
    (option) =>
      option.value === selected ||
      joinDataTypesCompatible(columnTypes[option.value] ?? '', otherSideType),
  )
}

export function QueryBuilderNotebookJoins({
  joins,
  editable = false,
  tableOptions = [],
  includedTableOptions = tableOptions,
  columnOptionsByTable = {},
  columnTypesByTable = {},
  onAddJoin,
  onUpdateJoin,
  onRemoveJoin,
  t,
}: {
  joins: SemanticJoin[]
  editable?: boolean
  tableOptions?: TableOption[]
  includedTableOptions?: TableOption[]
  columnOptionsByTable?: ColumnOptionsByTable
  columnTypesByTable?: ColumnTypesByTable
  onAddJoin?: () => void
  onUpdateJoin?: (index: number, join: SemanticJoin) => void
  onRemoveJoin?: (index: number) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  if (!joins.length && !editable) {
    return null
  }
  return (
    <NotebookStep
      label="Join data"
      themeClass="join"
      collapsible={!editable}
      defaultCollapsed={!editable && joins.length > 4}
      summary={t('query_builder.join_definitions', { count: joins.length })}
    >
      {editable && (
        <div className="flex w-full items-center justify-between gap-3">
          <span className="text-caption text-foreground-muted">
            {t('query_builder.metadata_joins_hint')}
          </span>
          <button
            type="button"
            className={qbAddBtnClass}
            onClick={onAddJoin}
            aria-label={t('query_builder.add_join')}
          >
            +
          </button>
        </div>
      )}
      <div className="border-border max-h-72 w-full min-w-0 overflow-auto rounded-md border">
        <table className="text-caption w-full border-collapse">
          <thead>
            <tr>
              <th className="border-border-strong text-foreground-muted sticky top-0 z-3 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-left text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.join_type')}
              </th>
              <th className="border-border-strong text-foreground-muted sticky top-0 z-3 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-left text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.qb_from')}
              </th>
              <th className="border-border-strong text-foreground-muted sticky top-0 z-3 w-16 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-center text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.qb_rel')}
              </th>
              <th className="border-border-strong text-foreground-muted sticky top-0 z-3 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-left text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.qb_to')}
              </th>
              <th className="border-border-strong text-foreground-muted sticky top-0 z-3 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-left text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.qb_on')}
              </th>
              {editable && (
                <th className="border-border-strong text-foreground-muted sticky top-0 z-3 w-10 border-b bg-(--table-header-bg) px-3 py-[0.45rem] text-right text-[0.68rem] font-bold tracking-[0.06em] uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                  {t('common.actions')}
                </th>
              )}
            </tr>
          </thead>
          <tbody>
            {editable && joins.length === 0 && (
              <tr>
                <td className="text-foreground-muted px-3 py-4 text-center" colSpan={6}>
                  {t('query_builder.metadata_joins_empty')}
                </td>
              </tr>
            )}
            {joins.map((j, index) => {
              const hintKey = joinTypeHintKey(j.join_type)
              const fromKey = metadataTableKey(j.from_schema ?? '', j.from_table)
              const toKey = metadataTableKey(j.to_schema ?? '', j.to_table)
              const fromType = columnTypesByTable[fromKey]?.[j.from_column] ?? ''
              const toType = columnTypesByTable[toKey]?.[j.to_column] ?? ''
              const incompatible = Boolean(
                fromType && toType && !joinDataTypesCompatible(fromType, toType),
              )
              return (
                <tr
                  key={j.id || index}
                  className="border-border border-b last:border-b-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even)"
                >
                  <td className="px-3 py-[0.4rem] align-middle">
                    {editable ? (
                      <Select
                        value={j.join_type}
                        onChange={(value) =>
                          onUpdateJoin?.(index, {
                            ...j,
                            join_type: value,
                          })
                        }
                        options={[
                          { value: 'LEFT', label: 'LEFT' },
                          { value: 'INNER', label: 'INNER' },
                          { value: 'RIGHT', label: 'RIGHT' },
                        ]}
                        size="sm"
                      />
                    ) : (
                      <span className={qbJoinTypeClass} title={hintKey ? t(hintKey) : undefined}>
                        <JoinTypeIcon type={j.join_type} />
                        {j.join_type}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    {editable ? (
                      <Select
                        value={fromKey}
                        onChange={(value) => {
                          const { schema, table } = splitMetadataTableKey(value)
                          onUpdateJoin?.(index, {
                            ...j,
                            from_schema: schema,
                            from_table: table,
                            from_column: '',
                          })
                        }}
                        options={includedTableOptions}
                        size="sm"
                        searchable
                      />
                    ) : (
                      <code className="text-foreground text-[0.76rem]">{j.from_table}</code>
                    )}
                  </td>
                  <td className="px-3 py-[0.4rem] text-center align-middle">
                    {editable ? (
                      <Select
                        value={j.relationship}
                        onChange={(value) =>
                          onUpdateJoin?.(index, {
                            ...j,
                            relationship: value,
                          })
                        }
                        options={[
                          { value: 'many_to_one', label: 'N:1' },
                          { value: 'one_to_many', label: '1:N' },
                          { value: 'one_to_one', label: '1:1' },
                          { value: 'many_to_many', label: 'N:N' },
                        ]}
                        size="sm"
                      />
                    ) : (
                      <span className={qbJoinCardinalityClass}>
                        {joinRelationshipCardinality(j.relationship)}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    {editable ? (
                      <Select
                        value={toKey}
                        onChange={(value) => {
                          const { schema, table } = splitMetadataTableKey(value)
                          onUpdateJoin?.(index, {
                            ...j,
                            to_schema: schema,
                            to_table: table,
                            to_column: '',
                          })
                        }}
                        options={tableOptions}
                        size="sm"
                        searchable
                      />
                    ) : (
                      <code className="text-foreground text-[0.76rem]">{j.to_table}</code>
                    )}
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    {editable ? (
                      <div className="grid min-w-80 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2">
                        <Select
                          value={j.from_column}
                          onChange={(value) => onUpdateJoin?.(index, { ...j, from_column: value })}
                          placeholder={t('query_builder.pick_column')}
                          options={compatibleColumnOptions(
                            columnOptionsByTable[fromKey] ?? [],
                            columnTypesByTable[fromKey],
                            toType,
                            j.from_column,
                          )}
                          size="sm"
                          searchable
                        />
                        <span className="text-foreground-faint">=</span>
                        <Select
                          value={j.to_column}
                          onChange={(value) => onUpdateJoin?.(index, { ...j, to_column: value })}
                          placeholder={t('query_builder.pick_column')}
                          options={compatibleColumnOptions(
                            columnOptionsByTable[toKey] ?? [],
                            columnTypesByTable[toKey],
                            fromType,
                            j.to_column,
                          )}
                          size="sm"
                          searchable
                        />
                        {incompatible && (
                          <p
                            className="text-caption col-span-3 m-0 text-amber-600 dark:text-amber-400"
                            role="alert"
                          >
                            {t('query_builder.join_incompatible_types', {
                              from: fromType,
                              to: toType,
                            })}
                          </p>
                        )}
                      </div>
                    ) : (
                      <code className="text-foreground-muted text-[0.72rem]">
                        <span className="text-indigo-600 dark:text-indigo-300">{j.from_table}</span>
                        .{j.from_column} ={' '}
                        <span className="text-indigo-600 dark:text-indigo-300">{j.to_table}</span>.
                        {j.to_column}
                      </code>
                    )}
                  </td>
                  {editable && (
                    <td className="px-3 py-[0.4rem] text-right align-middle">
                      <button
                        type="button"
                        className={qbTagCloseClass}
                        onClick={() => onRemoveJoin?.(index)}
                        aria-label={t('query_builder.remove_join_aria', { n: index + 1 })}
                      >
                        ×
                      </button>
                    </td>
                  )}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </NotebookStep>
  )
}
