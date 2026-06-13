import type { TranslationKey } from '../../i18n'
import type { SemanticJoin } from '../../types/semantic'
import { joinTypeHintKey } from '../ui/joinType'
import { JoinTypeIcon } from '../ui/JoinTypeIcon'
import { joinRelationshipCardinality } from './joinCardinality'
import { NotebookStep } from './NotebookStep'
import {
  qbJoinCardinalityClass,
  qbJoinConnectorClass,
  qbJoinExpressionClass,
  qbJoinFlowClass,
  qbJoinLineClass,
  qbJoinOnClauseClass,
  qbJoinOnLabelClass,
  qbJoinTablePrefixClass,
  qbJoinTypeClass,
  qbTagBase,
  qbTagTableClass,
} from './queryBuilderClasses'

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
      {joins.map((j, index) => {
        const hintKey = joinTypeHintKey(j.join_type)
        return (
          <div key={j.id || index} className={qbJoinFlowClass}>
            <span className={qbJoinTypeClass} title={hintKey ? t(hintKey) : undefined}>
              <JoinTypeIcon type={j.join_type} />
              {j.join_type}
            </span>
            <span className={`${qbTagBase} ${qbTagTableClass}`}>{j.from_table}</span>
            <span className={qbJoinConnectorClass}>
              <span className={qbJoinLineClass}></span>
              <span className={qbJoinCardinalityClass}>
                {joinRelationshipCardinality(j.relationship)}
              </span>
              <span className={qbJoinLineClass}></span>
            </span>
            <span className={`${qbTagBase} ${qbTagTableClass}`}>{j.to_table}</span>
            <span className={qbJoinOnClauseClass}>
              <span className={qbJoinOnLabelClass}>ON</span>
              <code className={qbJoinExpressionClass}>
                <span className={qbJoinTablePrefixClass}>{j.from_table}</span>.{j.from_column} ={' '}
                <span className={qbJoinTablePrefixClass}>{j.to_table}</span>.{j.to_column}
              </code>
            </span>
          </div>
        )
      })}
    </NotebookStep>
  )
}
