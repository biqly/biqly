import type { useT } from '../../i18n'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import {
  metadataEmptyHintClass,
  metadataToolbarClass,
  metadataToolbarTitleClass,
  metadataToolbarTopRowClass,
  metadataTypeBadgeClass,
  resultsTableMetadataListClass,
  resultsTableScrollClass,
} from '../../lib/tableClasses'
import type { RelationDetail } from '../../types/metadata'
import { LoadingScreen } from '../ui/LoadingScreen'

const RELATIONSHIP_SHORT: Record<string, string> = {
  many_to_one: 'N:1',
  one_to_many: '1:N',
  one_to_one: '1:1',
  many_to_many: 'N:N',
}

export function MetadataRelationsPanel({
  t,
  relations,
  loading,
}: {
  t: ReturnType<typeof useT>
  relations: RelationDetail[]
  loading: boolean
}) {
  return (
    <div className={cardClass()}>
      <div className={metadataToolbarClass}>
        <div className={metadataToolbarTopRowClass}>
          <h2 className={metadataToolbarTitleClass}>
            {t('metadata.relations_title')} ({relations.length})
          </h2>
        </div>
      </div>
      {loading && relations.length === 0 ? (
        <LoadingScreen minHeight="100px" />
      ) : relations.length === 0 ? (
        <p className={metadataEmptyHintClass}>{t('metadata.relations_empty')}</p>
      ) : (
        <div className={cn(resultsTableScrollClass, 'mt-2')}>
          <table className={resultsTableMetadataListClass()}>
            <thead>
              <tr>
                <th scope="col">{t('metadata.col_relation_from')}</th>
                <th scope="col">{t('metadata.col_relation_type')}</th>
                <th scope="col">{t('metadata.col_relation_to')}</th>
                <th scope="col">{t('metadata.col_table_desc')}</th>
              </tr>
            </thead>
            <tbody>
              {relations.map((rel) => (
                <tr
                  key={rel.id}
                  className="border-border border-b last:border-b-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even) hover:bg-(--table-stripe-hover)"
                >
                  <td className="font-mono text-[0.8rem]">
                    {rel.from_schema}.{rel.from_table}.{rel.from_column}
                  </td>
                  <td>
                    <span className={metadataTypeBadgeClass(false)}>
                      {RELATIONSHIP_SHORT[rel.relationship_type] ?? rel.relationship_type}
                    </span>
                  </td>
                  <td className="font-mono text-[0.8rem]">
                    {rel.to_schema}.{rel.to_table}.{rel.to_column}
                  </td>
                  <td>
                    {rel.description ? (
                      rel.description
                    ) : (
                      <span className="text-foreground-muted italic">
                        {t('metadata.relations_no_desc')}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
