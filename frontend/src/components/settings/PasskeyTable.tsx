import { type Locale, localeLanguageTag, useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import type { PasskeyInfo } from '../../types/auth'
import { formatDateTime } from '../../utils/formatters'
import { adminBtnIconOnlyClass, adminCenterContainerClass } from '../admin/adminClasses'
import { EmptyState } from '../ui/EmptyState'

interface PasskeyTableProps {
  passkeys: PasskeyInfo[]
  loading: boolean
  locale: Locale
  onRename: (passkey: PasskeyInfo) => void
  onDelete: (passkey: PasskeyInfo) => void
}

export function PasskeyTable({ passkeys, loading, locale, onRename, onDelete }: PasskeyTableProps) {
  const t = useT()
  const languageTag = localeLanguageTag(locale)

  if (loading) {
    return (
      <div className={adminCenterContainerClass}>
        <div
          className="spinner"
          style={{ width: '24px', height: '24px', borderTopColor: 'var(--accent)' }}
        ></div>
      </div>
    )
  }

  if (passkeys.length === 0) {
    return <EmptyState title={t('passkeys.empty_title')} description={t('passkeys.empty_desc')} />
  }

  return (
    <ul
      className={`border-border bg-bg-secondary m-0 flex list-none flex-col gap-0 overflow-hidden rounded-lg border p-0`}
      role="list"
    >
      {passkeys.map((passkey) => {
        const created = formatDateTime(passkey.created_at, languageTag)
        const lastUsed = passkey.last_used_at
          ? formatDateTime(passkey.last_used_at, languageTag)
          : t('passkeys.never_used')

        return (
          <li
            key={passkey.id}
            className={`border-border border-b px-4 py-[0.9rem] transition-colors duration-150 last:border-b-0 hover:bg-white/1.5`}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex min-w-0 flex-wrap items-center gap-x-[0.55rem] gap-y-[0.4rem]">
                <span
                  className="shrink-0 text-[1rem] leading-[1.2]"
                  aria-hidden
                  style={{ display: 'inline-flex', alignItems: 'center' }}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-key-round"
                    style={{ color: 'var(--warning, #f59e0b)' }}
                  >
                    <path d="M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.828-.828A5 5 0 1 0 3.414 11l-.828.828z" />
                    <circle cx="17" cy="7" r="1" />
                  </svg>
                </span>
                <span className="text-[0.9rem] leading-[1.35] font-semibold wrap-break-word">
                  {passkey.name}
                </span>
              </div>
              <div className="flex shrink-0 items-center gap-[0.35rem]">
                <button
                  type="button"
                  className={cn(
                    legacyButtonClass('btn btn-sm btn-secondary'),
                    adminBtnIconOnlyClass,
                  )}
                  title={t('passkeys.rename_title')}
                  onClick={() => onRename(passkey)}
                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-pencil"
                  >
                    <path d="M12 20h9" />
                    <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
                  </svg>
                </button>
                <button
                  type="button"
                  className={cn(
                    legacyButtonClass('btn btn-sm btn-danger-outline'),
                    adminBtnIconOnlyClass,
                  )}
                  title={t('passkeys.delete_title')}
                  onClick={() => onDelete(passkey)}
                  style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-trash-2"
                  >
                    <path d="M3 6h18" />
                    <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                    <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                    <line x1="10" x2="10" y1="11" y2="17" />
                    <line x1="14" x2="14" y1="11" y2="17" />
                  </svg>
                </button>
              </div>
            </div>
            <p className="text-foreground-muted mt-[0.45rem] mr-0 mb-0 ml-0 text-[0.78rem] leading-[1.45]">
              <span>
                {t('passkeys.col_created')}: {created}
              </span>
              <span className="mx-[0.35rem] opacity-55" aria-hidden>
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
