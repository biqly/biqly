import type { TranslationKey } from '../../i18n'
import type { SemanticJoin } from '../../types/semantic'
import { joinTypeHintKey } from '../ui/joinType'
import { JoinTypeIcon } from '../ui/JoinTypeIcon'
import { joinRelationshipCardinality } from './joinCardinality'
import { NotebookStep } from './NotebookStep'

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
          <div key={j.id || index} className="notebook-join-flow">
            <span className="notebook-join-type" title={hintKey ? t(hintKey) : undefined}>
              <JoinTypeIcon type={j.join_type} />
              {j.join_type}
            </span>
            <span className="notebook-tag notebook-tag--table">{j.from_table}</span>
            <span className="notebook-join-connector">
              <span className="notebook-join-line"></span>
              <span className="notebook-join-cardinality">
                {joinRelationshipCardinality(j.relationship)}
              </span>
              <span className="notebook-join-line"></span>
            </span>
            <span className="notebook-tag notebook-tag--table">{j.to_table}</span>
            <span className="notebook-join-on-clause">
              <span className="notebook-join-on-label">ON</span>
              <code className="notebook-join-expression">
                <span className="notebook-join-table-prefix">{j.from_table}</span>.{j.from_column} ={' '}
                <span className="notebook-join-table-prefix">{j.to_table}</span>.{j.to_column}
              </code>
            </span>
          </div>
        )
      })}
    </NotebookStep>
  )
}
