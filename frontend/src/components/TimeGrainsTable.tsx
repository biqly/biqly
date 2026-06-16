import { useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyTableClass } from '../lib/tableClasses'
import { LoadingOverlay } from './ui/LoadingOverlay'
interface TimeGrain {
  grain: string
  suffix: string
  requires_time: boolean
  synonyms: string[]
}

export function TimeGrainsTable({
  grains,
  loading,
  onEdit,
}: {
  grains: TimeGrain[]
  loading: boolean
  onEdit: (grain: TimeGrain) => void
}) {
  const t = useT()
  return (
    <LoadingOverlay loading={loading}>
      <div style={{ minHeight: grains.length === 0 && loading ? 120 : 'auto' }}>
        {grains.length > 0 ? (
          <table className={legacyTableClass('results-table')}>
            <thead>
              <tr>
                <th>{t('time_grains.col_grain')}</th>
                <th>{t('time_grains.col_suffix')}</th>
                <th>{t('time_grains.col_requires_time')}</th>
                <th>{t('time_grains.col_synonyms')}</th>
                <th className="actions">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {grains.map((tg) => (
                <tr key={tg.grain}>
                  <td style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{tg.grain}</td>
                  <td>
                    <code style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>{tg.suffix}</code>
                  </td>
                  <td>
                    {tg.requires_time ? (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          padding: '0.15rem 0.5rem',
                          background: 'rgba(16,185,129,0.1)',
                          color: 'var(--success)',
                          borderRadius: '999px',
                          fontSize: '0.72rem',
                          fontWeight: 500,
                        }}
                      >
                        {t('common.yes')}
                      </span>
                    ) : (
                      <span
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          padding: '0.15rem 0.5rem',
                          background: 'rgba(255,255,255,0.04)',
                          color: 'var(--text-muted)',
                          borderRadius: '999px',
                          fontSize: '0.72rem',
                        }}
                      >
                        {t('common.no')}
                      </span>
                    )}
                  </td>
                  <td>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.3rem' }}>
                      {tg.synonyms.map((syn) => (
                        <span
                          key={syn}
                          className="text-accent inline-block rounded-[0.3rem] border border-(--accent-tint-border) bg-(--accent-tint-bg) px-2 py-[0.15rem] text-[0.72rem]"
                        >
                          {syn}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="actions">
                    <button
                      type="button"
                      className={legacyButtonClass('btn btn-sm btn-ghost')}
                      onClick={() => onEdit(tg)}
                    >
                      {t('common.edit')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>
            {loading ? '' : t('common.no_data')}
          </div>
        )}
      </div>
    </LoadingOverlay>
  )
}
