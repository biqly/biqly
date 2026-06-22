import type { TranslationKey } from '../../i18n'
import type { SemanticJoin } from '../../types/semantic'
import { joinTypeHintKey } from '../ui/joinType'
import { JoinTypeIcon } from '../ui/JoinTypeIcon'
import { joinRelationshipCardinality } from './joinCardinality'
import { NotebookStep } from './NotebookStep'
import { qbJoinCardinalityClass, qbJoinTypeClass } from './queryBuilderClasses'

export function QueryBuilderNotebookJoins({
  joins,
  t,
}: {
  joins: SemanticJoin[]
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  if (!joins.length) {
    return null
  }
  return (
    <NotebookStep label="Join data" themeClass="join">
      <div className="border-border max-h-72 overflow-y-auto rounded-md border">
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
            </tr>
          </thead>
          <tbody>
            {joins.map((j, index) => {
              const hintKey = joinTypeHintKey(j.join_type)
              return (
                <tr
                  key={j.id || index}
                  className="border-border border-b last:border-b-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even)"
                >
                  <td className="px-3 py-[0.4rem] align-middle">
                    <span className={qbJoinTypeClass} title={hintKey ? t(hintKey) : undefined}>
                      <JoinTypeIcon type={j.join_type} />
                      {j.join_type}
                    </span>
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    <code className="text-foreground text-[0.76rem]">{j.from_table}</code>
                  </td>
                  <td className="px-3 py-[0.4rem] text-center align-middle">
                    <span className={qbJoinCardinalityClass}>
                      {joinRelationshipCardinality(j.relationship)}
                    </span>
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    <code className="text-foreground text-[0.76rem]">{j.to_table}</code>
                  </td>
                  <td className="px-3 py-[0.4rem] align-middle">
                    <code className="text-foreground-muted text-[0.72rem]">
                      <span className="text-indigo-600 dark:text-indigo-300">{j.from_table}</span>.
                      {j.from_column} ={' '}
                      <span className="text-indigo-600 dark:text-indigo-300">{j.to_table}</span>.
                      {j.to_column}
                    </code>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </NotebookStep>
  )
}
