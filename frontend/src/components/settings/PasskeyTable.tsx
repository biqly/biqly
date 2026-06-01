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
    <ul className="settings-security-list" role="list">
      {passkeys.map((passkey) => {
        const created = new Date(passkey.created_at).toLocaleString(languageTag)
        const lastUsed = passkey.last_used_at
          ? new Date(passkey.last_used_at).toLocaleString(languageTag)
          : t('passkeys.never_used')

        return (
          <li key={passkey.id} className="settings-security-item">
            <div className="settings-security-item__row">
              <div className="settings-security-item__identity">
                <span className="settings-security-item__icon" aria-hidden>
                  🔑
                </span>
                <span className="settings-security-item__name">{passkey.name}</span>
              </div>
              <div className="settings-security-item__actions">
                <button
                  type="button"
                  className="btn btn-sm btn-secondary btn-icon-only"
                  title={t('passkeys.rename_title')}
                  onClick={() => onRename(passkey)}
                >
                  ✏️
                </button>
                <button
                  type="button"
                  className="btn btn-sm btn-danger-outline btn-icon-only"
                  title={t('passkeys.delete_title')}
                  onClick={() => onDelete(passkey)}
                >
                  🗑️
                </button>
              </div>
            </div>
            <p className="settings-security-item__detail">
              <span>
                {t('passkeys.col_created')}: {created}
              </span>
              <span className="settings-security-item__sep" aria-hidden>
                ·
              </span>
              <span>
                {t('passkeys.col_last_used')}: {lastUsed}
              </span>
            </p>
          </li>
        )
      })}
    </ul>
  )
}
