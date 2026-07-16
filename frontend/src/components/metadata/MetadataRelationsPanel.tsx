import type { Locale } from '../../i18n'
import { LOCALE_OPTIONS, SUPPORTED_LOCALES, type useT } from '../../i18n'
import { cardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import {
  metadataEmptyHintClass,
  metadataLangTabClass,
  metadataLangTabsClass,
  metadataRowActionClass,
  metadataRowActionLabelClass,
  metadataToolbarActionsClass,
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
  editLocale,
  onEditLocaleChange,
  describingRelationId,
  onDescribeOne,
}: {
  t: ReturnType<typeof useT>
  relations: RelationDetail[]
  loading: boolean
  editLocale: Locale
  onEditLocaleChange: (loc: Locale) => void
  describingRelationId: string | null
  onDescribeOne: (rel: RelationDetail) => void
}) {
  return (
    <div className={cardClass()}>
      <div className={metadataToolbarClass}>
        <div className={metadataToolbarTopRowClass}>
          <h2 className={metadataToolbarTitleClass}>
            {t('metadata.relations_title')} ({relations.length})
          </h2>
          <div className={metadataToolbarActionsClass}>
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
                <th scope="col" className="actions">
                  {t('metadata.col_actions')}
                </th>
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
                  <td className="actions">
                    <button
                      type="button"
                      className={metadataRowActionClass}
                      onClick={() => onDescribeOne(rel)}
                      disabled={describingRelationId !== null}
                      aria-label={t('metadata.relations_describe_one_aria', {
                        from: `${rel.from_table}.${rel.from_column}`,
                        to: `${rel.to_table}.${rel.to_column}`,
                      })}
                      title={t('metadata.relations_describe_one')}
                    >
                      <span aria-hidden="true">✨</span>
                      <span className={metadataRowActionLabelClass}>
                        {describingRelationId === rel.id
                          ? t('metadata.relations_describing')
                          : t('metadata.relations_describe_one')}
                      </span>
                    </button>
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
