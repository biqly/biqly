import { localeLanguageTag, useT, type Locale } from '../../i18n'
import type { PasskeyInfo } from '../../types/auth'
import { EmptyState } from '../ui/EmptyState'

interface PasskeyTableProps {
  passkeys: PasskeyInfo[]
  loading: boolean
  locale: Locale
  onRename: (passkey: PasskeyInfo) => void
  onDelete: (passkey: PasskeyInfo) => void
}

export function PasskeyTable({
  passkeys,
  loading,
  locale,
  onRename,
  onDelete,
}: PasskeyTableProps) {
  const t = useT()
  const languageTag = localeLanguageTag(locale)

  if (loading) {
    return (
      <div className="admin-center-container">
        <div className="spinner" style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}></div>
      </div>
    )
  }

  if (passkeys.length === 0) {
    return <EmptyState title={t('passkeys.empty_title')} description={t('passkeys.empty_desc')} />
  }

  return (
    <div className="results-table-scroll border-border overflow-hidden">
      <table className="results-table" style={{ margin: 0 }}>
        <thead>
          <tr>
            <th>{t('passkeys.col_name')}</th>
            <th>{t('passkeys.col_created')}</th>
            <th>{t('passkeys.col_last_used')}</th>
            <th className="actions" style={{ width: '80px', textAlign: 'right' }}></th>
          </tr>
        </thead>
        <tbody>
          {passkeys.map((passkey) => (
            <tr key={passkey.id}>
              <td style={{ fontWeight: 600 }}>
                <span style={{ marginRight: '6px' }}>🔑</span> {passkey.name}
              </td>
              <td>{new Date(passkey.created_at).toLocaleString(languageTag)}</td>
              <td>
                {passkey.last_used_at
                  ? new Date(passkey.last_used_at).toLocaleString(languageTag)
                  : t('passkeys.never_used')}
              </td>
              <td className="actions">
                <div className="flex-gap-center-end">
                  <button
                    type="button"
                    className="btn btn-sm btn-secondary btn-icon-only"
                    onClick={() => onRename(passkey)}
                  >
                    ✏️
                  </button>
                  <button
                    type="button"
                    className="btn btn-sm btn-danger-outline btn-icon-only"
                    onClick={() => onDelete(passkey)}
                  >
                    🗑️
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
