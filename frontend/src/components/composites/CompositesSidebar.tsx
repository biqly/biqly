import type { useT } from '../../i18n'
import type { CompositeModelSummary } from '../../types/composite'
import { EmptyState } from '../ui/EmptyState'
import {
  compositeNameClass,
  compositesDeleteBtnClass,
  compositesListBtnClass,
  compositesSidebarClass,
  compositesSidebarEmptyClass,
  compositesSidebarHeaderClass,
  compositesSidebarHeaderTitleClass,
  compositesSidebarItemClass,
  compositesSidebarListClass,
  compositeStatusClass,
} from './compositesClasses'

export function CompositesSidebar({
  t,
  composites,
  selectedId,
  onSelect,
  onDelete,
}: {
  t: ReturnType<typeof useT>
  composites: CompositeModelSummary[]
  selectedId: string | null
  onSelect: (id: string) => void
  onDelete: (id: string) => void
}) {
  return (
    <aside className={compositesSidebarClass}>
      <div className={compositesSidebarHeaderClass}>
        <h2 className={compositesSidebarHeaderTitleClass}>{t('composites.sidebar_title')}</h2>
      </div>
      {composites.length === 0 ? (
        <div className={compositesSidebarEmptyClass}>
          <EmptyState description={t('composites.empty_list')} />
        </div>
      ) : (
        <ul className={compositesSidebarListClass}>
          {composites.map((c) => {
            const active = c.id === selectedId
            return (
              <li key={c.id} className={compositesSidebarItemClass}>
                <button
                  type="button"
                  className={compositesListBtnClass(active)}
                  onClick={() => onSelect(c.id)}
                >
                  <span className={compositeNameClass(active)}>{c.label ?? c.name}</span>
                  <span className={compositeStatusClass(c.status)}>{c.status}</span>
                </button>
                <button
                  type="button"
                  className={compositesDeleteBtnClass}
                  aria-label={t('composites.aria_delete')}
                  onClick={() => onDelete(c.id)}
                >
                  ×
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </aside>
  )
}
