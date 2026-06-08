import type { useT } from '../../i18n'
import type { CompositeModelSummary } from '../../types/composite'
import { EmptyState } from '../ui/EmptyState'

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
    <aside className="composites-sidebar">
      <div className="composites-sidebar-header">
        <h2>{t('composites.sidebar_title')}</h2>
      </div>
      {composites.length === 0 ? (
        <div style={{ padding: '1rem' }}>
          <EmptyState description={t('composites.empty_list')} />
        </div>
      ) : (
        <ul>
          {composites.map((c) => (
            <li key={c.id} className={c.id === selectedId ? 'active' : ''}>
              <button type="button" className="composites-list-btn" onClick={() => onSelect(c.id)}>
                <span className="composite-name">{c.label ?? c.name}</span>
                <span className={`composite-status status-${c.status}`}>{c.status}</span>
              </button>
              <button
                type="button"
                className="composites-delete-btn"
                aria-label={t('composites.aria_delete')}
                onClick={() => onDelete(c.id)}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
    </aside>
  )
}
